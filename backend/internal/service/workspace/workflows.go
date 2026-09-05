package workspace

import (
	"context"
	"fmt"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListWorkflows(ctx context.Context, request *workspacev1.ListWorkflowsRequest) (*workspacev1.ListWorkflowsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListWorkflows(ctx, owner, request.Deleted)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Workflow, 0, len(items))
	for _, item := range items {
		response = append(response, workflowResponse(item))
	}
	return &workspacev1.ListWorkflowsResponse{Items: response}, nil
}

func (service *Service) CreateWorkflow(ctx context.Context, request *workspacev1.CreateWorkflowRequest) (*workspacev1.Workflow, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.workflowInput(request.Workflow)
	if err != nil {
		return nil, publicError(err)
	}
	secrets, err := service.encryptEnvironmentSecrets(owner, "workflow-environment:", input.Environment, submittedSecrets, nil)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().CreateWorkflow(ctx, owner, input, secrets)
	if err != nil {
		return nil, publicError(err)
	}
	return workflowResponse(item), nil
}

func (service *Service) GetWorkflow(ctx context.Context, request *workspacev1.GetWorkflowRequest) (*workspacev1.Workflow, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetWorkflow(ctx, owner, request.WorkflowId, true)
	if err != nil {
		return nil, publicError(err)
	}
	return workflowResponse(item), nil
}

func (service *Service) UpdateWorkflow(ctx context.Context, request *workspacev1.UpdateWorkflowRequest) (*workspacev1.Workflow, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.workflowInput(request.Workflow)
	if err != nil {
		return nil, publicError(err)
	}
	existingCiphertext, err := service.workspace.Repository().GetWorkflowEnvironmentSecret(ctx, owner, request.WorkflowId)
	if err != nil {
		return nil, publicError(err)
	}
	secrets, err := service.encryptEnvironmentSecrets(owner, "workflow-environment:", input.Environment, submittedSecrets, existingCiphertext)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().UpdateWorkflow(ctx, owner, request.WorkflowId, input, secrets, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return workflowResponse(item), nil
}

func (service *Service) DeleteWorkflow(ctx context.Context, request *workspacev1.DeleteWorkflowRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetWorkflow(ctx, owner, request.WorkflowId, false)
	if err != nil {
		return nil, publicError(err)
	}
	if err := service.workspace.Repository().DeleteWorkflow(ctx, owner, request.WorkflowId); err != nil {
		return nil, publicError(err)
	}
	if err := service.files.Clear(ctx, item.WorkspacePath); err != nil {
		return nil, publicError(fmt.Errorf("clear deleted Workflow Workspace: %w", err))
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) GenerateWorkflowCredential(ctx context.Context, request *workspacev1.GenerateWorkflowCredentialRequest) (*workspacev1.WorkflowCredential, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	key, err := randomCredential("awk_", 18)
	if err != nil {
		return nil, publicError(err)
	}
	secret, err := randomCredential("aws_", 32)
	if err != nil {
		return nil, publicError(err)
	}
	hash, err := hashSecret(secret)
	if err != nil {
		return nil, publicError(err)
	}
	if _, err := service.workspace.Repository().SetWorkflowCredential(ctx, owner, request.WorkflowId, key, hash); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.WorkflowCredential{ApiKey: key, ApiSecret: secret, CreatedAt: timestamppb.Now()}, nil
}

func (service *Service) ExchangeWorkflowCredential(ctx context.Context, request *workspacev1.ExchangeWorkflowCredentialRequest) (*workspacev1.WorkflowAccessToken, error) {
	access, ok := ctx.Value(workflowCredentialContextKey{}).(workflowCredentialContext)
	if !ok || access.WorkflowID != request.WorkflowId {
		return nil, publicError(accountdomain.ErrUnauthenticated)
	}
	token, expiresAt, err := issueWorkflowToken(access, time.Now())
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.WorkflowAccessToken{JwtToken: token, TokenType: "Bearer", ExpiresAt: timestamppb.New(expiresAt)}, nil
}

func (service *Service) RunWorkflow(ctx context.Context, request *workspacev1.RunWorkflowRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.credits.RequirePositiveBalance(ctx, owner, service.userTimezone(ctx, owner)); err != nil {
		return nil, publicError(err)
	}
	workflow, err := service.workspace.Repository().GetWorkflow(ctx, owner, request.WorkflowId, false)
	if err != nil {
		return nil, publicError(err)
	}
	if err := service.validateExecutionRuntimes(ctx, owner, workflow.ExpertID, workflow.ExpertTeamID); err != nil {
		return nil, publicError(err)
	}
	var jsonInput map[string]any
	if request.JsonInput != nil {
		jsonInput = request.JsonInput.AsMap()
	}
	trigger := "manual"
	if workflowAPIAccess(ctx, request.WorkflowId) {
		trigger = "api"
	}
	var item workspacedomain.Run
	if trigger == "api" {
		key := requestHeader(ctx, "Idempotency-Key")
		var replayed bool
		item, replayed, err = service.workspace.Repository().CreateRunIdempotent(ctx, owner, request.WorkflowId, trigger, key, request.TextInput, jsonInput)
		if err == nil {
			setResponseStatus(ctx, 202)
			if replayed {
				setResponseHeader(ctx, "Idempotency-Replayed", "true")
			}
		}
	} else {
		item, err = service.workspace.Repository().CreateRun(ctx, owner, request.WorkflowId, trigger, request.TextInput, jsonInput)
	}
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(item), nil
}

func requestHeader(ctx context.Context, name string) string {
	transporter, ok := transport.FromServerContext(ctx)
	if !ok || transporter.RequestHeader() == nil {
		return ""
	}
	return transporter.RequestHeader().Get(name)
}

func setResponseStatus(ctx context.Context, status int) {
	setResponseHeader(ctx, "X-Agent-Platform-Internal-Response-Status", fmt.Sprintf("%d", status))
}

func setResponseHeader(ctx context.Context, name, value string) {
	transporter, ok := transport.FromServerContext(ctx)
	if ok && transporter.ReplyHeader() != nil {
		transporter.ReplyHeader().Set(name, value)
	}
}

func (service *Service) ListRuns(ctx context.Context, request *workspacev1.ListRunsRequest) (*workspacev1.ListRunsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListRuns(ctx, owner, request.WorkflowId)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Run, 0, len(items))
	for _, item := range items {
		response = append(response, runResponse(item))
	}
	return &workspacev1.ListRunsResponse{Items: response}, nil
}

func (service *Service) GetRun(ctx context.Context, request *workspacev1.GetRunRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetRun(ctx, owner, request.WorkflowId, request.RunId)
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(item), nil
}

func (service *Service) ListRunTurns(ctx context.Context, request *workspacev1.ListRunTurnsRequest) (*workspacev1.ListRunsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListRunTurns(ctx, owner, request.WorkflowId, request.RunId)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Run, 0, len(items))
	for _, item := range items {
		response = append(response, runResponse(item))
	}
	return &workspacev1.ListRunsResponse{Items: response}, nil
}

func (service *Service) ContinueRunConversation(ctx context.Context, request *workspacev1.ContinueRunConversationRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.credits.RequirePositiveBalance(ctx, owner, service.userTimezone(ctx, owner)); err != nil {
		return nil, publicError(err)
	}
	attachments, err := service.resolveAttachments(ctx, owner, request.AttachmentIds)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().ContinueRunConversation(ctx, owner, request.WorkflowId, request.RunId, request.Content, attachments)
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(item), nil
}

func (service *Service) CancelRun(ctx context.Context, request *workspacev1.CancelRunRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().CancelRun(ctx, owner, request.WorkflowId, request.RunId)
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(item), nil
}

func (service *Service) RerunWorkflow(ctx context.Context, request *workspacev1.RerunWorkflowRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.credits.RequirePositiveBalance(ctx, owner, service.userTimezone(ctx, owner)); err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().Rerun(ctx, owner, request.WorkflowId, request.RunId)
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(item), nil
}

func (service *Service) ListArtifacts(ctx context.Context, request *workspacev1.ListArtifactsRequest) (*workspacev1.ListArtifactsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListArtifacts(ctx, owner, request.WorkflowId)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Artifact, 0, len(items))
	for _, item := range items {
		response = append(response, artifactResponse(item))
	}
	return &workspacev1.ListArtifactsResponse{Items: response}, nil
}

func (service *Service) workflowInput(input *workspacev1.WorkflowInput) (workspacedomain.WorkflowInput, map[string]string, error) {
	if input == nil {
		return workspacedomain.WorkflowInput{}, nil, fmt.Errorf("%w: Workflow input is required", workspacedomain.ErrInvalid)
	}
	domainInput := workspacedomain.WorkflowInput{Name: input.Name, Goal: input.Goal, ExpertID: input.ExpertId, ExpertTeamID: input.ExpertTeamId}
	secrets := make(map[string]string)
	for _, value := range input.Environment {
		if value == nil {
			continue
		}
		domainValue := workspacedomain.EnvironmentVariable{Name: value.Name, Secret: value.Secret, Configured: value.Configured || value.Value != nil}
		if value.Value != nil {
			if value.Secret {
				secrets[value.Name] = *value.Value
			} else {
				domainValue.Value = *value.Value
			}
		}
		domainInput.Environment = append(domainInput.Environment, domainValue)
	}
	if input.Schedule != nil {
		domainInput.Schedule = &workspacedomain.Schedule{Enabled: input.Schedule.Enabled, Frequency: input.Schedule.Frequency, Hour: input.Schedule.Hour, Minute: input.Schedule.Minute, Weekday: input.Schedule.Weekday, Timezone: input.Schedule.Timezone}
	}
	if err := domainInput.Validate(); err != nil {
		return workspacedomain.WorkflowInput{}, nil, err
	}
	return domainInput, secrets, nil
}

func workflowResponse(item workspacedomain.Workflow) *workspacev1.Workflow {
	response := &workspacev1.Workflow{Id: item.ID, Name: item.Name, Goal: item.Goal, ExpertId: item.ExpertID, ExpertTeamId: item.ExpertTeamID, ApiCredentialConfigured: item.APICredentialConfigured, Deleted: item.DeletedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
	for _, value := range item.Environment {
		environment := &workspacev1.EnvironmentVariable{Name: value.Name, Secret: value.Secret, Configured: value.Configured}
		if !value.Secret && value.Value != "" {
			environment.Value = &value.Value
		}
		response.Environment = append(response.Environment, environment)
	}
	if item.Schedule != nil {
		response.Schedule = &workspacev1.Schedule{Enabled: item.Schedule.Enabled, Frequency: item.Schedule.Frequency, Hour: item.Schedule.Hour, Minute: item.Schedule.Minute, Weekday: item.Schedule.Weekday, Timezone: item.Schedule.Timezone}
	}
	if item.GitSource != nil {
		config := make([]*workspacev1.GitConfigEntry, 0, len(item.GitSource.Config))
		for _, entry := range item.GitSource.Config {
			config = append(config, &workspacev1.GitConfigEntry{Key: entry.Key, Value: entry.Value})
		}
		response.GitSource = &workspacev1.GitSource{Url: item.GitSource.URL, Branch: item.GitSource.Branch, Authentication: item.GitSource.Authentication, Username: item.GitSource.Username, Config: config, SshConfig: item.GitSource.SSHConfig, CredentialConfigured: item.GitSource.CredentialConfigured}
	}
	return response
}

func runResponse(item workspacedomain.Run) *workspacev1.Run {
	response := &workspacev1.Run{Id: item.ID, ConversationId: item.ConversationID, TurnNumber: int32(item.TurnNumber), WorkflowId: item.WorkflowID, WorkflowName: item.WorkflowName, Trigger: item.Trigger, State: item.State, TextInput: item.TextInput, FinalText: item.FinalText, QueuedAt: timestamppb.New(item.QueuedAt)}
	if item.JSONInput != nil {
		response.JsonInput, _ = structpb.NewStruct(item.JSONInput)
	}
	if item.WorkflowSnapshot != nil {
		response.WorkflowSnapshot, _ = structpb.NewStruct(item.WorkflowSnapshot)
	}
	if item.FinalJSON != nil {
		response.FinalJson, _ = structpb.NewStruct(item.FinalJSON)
	}
	if item.Error != "" {
		response.Error = &item.Error
	}
	if item.StartedAt != nil {
		response.StartedAt = timestamppb.New(*item.StartedAt)
	}
	if item.EndedAt != nil {
		response.EndedAt = timestamppb.New(*item.EndedAt)
	}
	for _, attachment := range item.Attachments {
		response.Attachments = append(response.Attachments, attachmentResponse(attachment))
	}
	for _, stage := range item.ExpertStages {
		response.ExpertStages = append(response.ExpertStages, expertStageResponse(stage))
	}
	response.CreditConsumption = creditConsumptionResponse(item.CreditConsumption)
	if item.StartedAt != nil {
		end := time.Now()
		if item.EndedAt != nil {
			end = *item.EndedAt
		}
		response.ElapsedMs = end.Sub(*item.StartedAt).Milliseconds()
	}
	return response
}
