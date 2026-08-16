package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	agentlifecyclev1 "agent-platform/backend/api/agentlifecycle/v1"
	typesv1 "agent-platform/backend/api/types/v1"
	agentapplication "agent-platform/backend/internal/biz/agentlifecycle/application"
	agentdomain "agent-platform/backend/internal/biz/agentlifecycle/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func agentQueryError(err error) error {
	if errors.Is(err, agentdomain.ErrAgentNotFound) || errors.Is(err, agentdomain.ErrDraftNotFound) || errors.Is(err, agentdomain.ErrReleaseNotFound) {
		return publicError(404, "agent_lifecycle_resource_not_found")
	}
	return publicError(500, "agent_lifecycle_query_failed")
}
func validAgentIDs(values ...string) error {
	for _, value := range values {
		if _, err := uuid.Parse(value); err != nil {
			return publicError(400, "invalid_agent_lifecycle_id")
		}
	}
	return nil
}

func (service *GeneratedServices) ListAgents(ctx context.Context, request *agentlifecyclev1.ListAgentsRequest) (*agentlifecyclev1.ListAgentsResponse, error) {
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.AgentLifecycle.ListAgents(ctx, actor.OrganizationID, actor.TeamID)
	if err != nil {
		return nil, agentQueryError(err)
	}
	items := make([]agentResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newAgentResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &agentlifecyclev1.ListAgentsResponse{})
}

func (service *GeneratedServices) CreateAgent(ctx context.Context, request *agentlifecyclev1.CreateAgentRequest) (*agentlifecyclev1.Agent, error) {
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent.create", "", request, http.StatusCreated, &agentlifecyclev1.Agent{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.CreateAgent(ctx, agentapplication.CreateAgentCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, Name: request.Name, Description: request.Description, CreatedBy: actor.UserID})
	}, func(value any) any { return newAgentResponse(value.(agentdomain.Agent)) })
}

func (service *GeneratedServices) GetAgent(ctx context.Context, request *agentlifecyclev1.GetAgentRequest) (*agentlifecyclev1.Agent, error) {
	if err := validAgentIDs(request.AgentId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.AgentLifecycle.GetAgent(ctx, actor.OrganizationID, actor.TeamID, request.AgentId)
	if err != nil {
		return nil, agentQueryError(err)
	}
	return mappedResponse(ctx, http.StatusOK, newAgentResponse(value), &agentlifecyclev1.Agent{})
}

func (service *GeneratedServices) UpdateAgent(ctx context.Context, request *agentlifecyclev1.UpdateAgentRequest) (*agentlifecyclev1.Agent, error) {
	if err := validAgentIDs(request.AgentId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent.update:"+request.AgentId, strconv.FormatInt(version, 10), request, http.StatusOK, &agentlifecyclev1.Agent{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.UpdateAgent(ctx, agentapplication.UpdateAgentCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, ID: request.AgentId, Name: request.Name, Description: request.Description, ExpectedVersion: version})
	}, func(value any) any { return newAgentResponse(value.(agentdomain.Agent)) })
}

func (service *GeneratedServices) ListAgentDrafts(ctx context.Context, request *agentlifecyclev1.ListAgentDraftsRequest) (*agentlifecyclev1.ListAgentDraftsResponse, error) {
	if err := validAgentIDs(request.AgentId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.AgentLifecycle.ListDrafts(ctx, actor.OrganizationID, actor.TeamID, request.AgentId)
	if err != nil {
		return nil, agentQueryError(err)
	}
	items := make([]draftResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newDraftResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &agentlifecyclev1.ListAgentDraftsResponse{})
}

func (service *GeneratedServices) CreateAgentDraft(ctx context.Context, request *agentlifecyclev1.CreateAgentDraftRequest) (*agentlifecyclev1.AgentDraft, error) {
	if err := validAgentIDs(request.AgentId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	configuration, err := agentConfiguration(request.Configuration)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent-draft.create:"+request.AgentId, "", request, http.StatusCreated, &agentlifecyclev1.AgentDraft{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.CreateDraft(ctx, agentapplication.CreateDraftCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, AgentID: request.AgentId, CreatedBy: actor.UserID, Configuration: configuration, ReleaseRisk: agentdomain.ReleaseRisk(request.ReleaseRisk)})
	}, func(value any) any { return newDraftResponse(value.(agentdomain.Draft)) })
}

func (service *GeneratedServices) GetAgentDraft(ctx context.Context, request *agentlifecyclev1.GetAgentDraftRequest) (*agentlifecyclev1.AgentDraft, error) {
	if err := validAgentIDs(request.AgentId, request.DraftId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.AgentLifecycle.GetDraft(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.DraftId)
	if err != nil {
		return nil, agentQueryError(err)
	}
	return mappedResponse(ctx, http.StatusOK, newDraftResponse(value), &agentlifecyclev1.AgentDraft{})
}

func (service *GeneratedServices) UpdateAgentDraft(ctx context.Context, request *agentlifecyclev1.UpdateAgentDraftRequest) (*agentlifecyclev1.AgentDraft, error) {
	if err := validAgentIDs(request.AgentId, request.DraftId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	configuration, err := agentConfiguration(request.Configuration)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent-draft.update:"+request.DraftId, strconv.FormatInt(version, 10), request, http.StatusOK, &agentlifecyclev1.AgentDraft{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.EditDraft(ctx, agentapplication.EditDraftCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, AgentID: request.AgentId, DraftID: request.DraftId, Configuration: configuration, ReleaseRisk: agentdomain.ReleaseRisk(request.ReleaseRisk), ExpectedVersion: version})
	}, func(value any) any { return newDraftResponse(value.(agentdomain.Draft)) })
}

func (service *GeneratedServices) ValidateAgentDraft(ctx context.Context, request *agentlifecyclev1.ValidateAgentDraftRequest) (*agentlifecyclev1.AgentDraft, error) {
	return draftAction(service, ctx, request, request.AgentId, request.DraftId, request.TeamId, "agent-draft.validate:", func(services transaction.TransactionServices, actor identitydomain.Actor, version int64) (any, error) {
		return services.Agents.ValidateDraft(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.DraftId, version)
	}, &agentlifecyclev1.AgentDraft{}, func(value any) any { return newDraftResponse(value.(agentdomain.Draft)) })
}

func (service *GeneratedServices) RequestAgentDraftApproval(ctx context.Context, request *agentlifecyclev1.RequestAgentDraftApprovalRequest) (*agentlifecyclev1.ReleaseApproval, error) {
	if err := validAgentIDs(request.AgentId, request.DraftId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent-draft.approval.request:"+request.DraftId, "", request, http.StatusCreated, &agentlifecyclev1.ReleaseApproval{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.RequestApproval(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.DraftId, actor.UserID)
	}, func(value any) any { return newApprovalResponse(value.(agentdomain.ReleaseApproval)) })
}

func (service *GeneratedServices) DecideAgentDraftApproval(ctx context.Context, request *agentlifecyclev1.DecideAgentDraftApprovalRequest) (*agentlifecyclev1.ReleaseApproval, error) {
	return draftAction(service, ctx, request, request.AgentId, request.DraftId, request.TeamId, "agent-draft.approval.decide:", func(services transaction.TransactionServices, actor identitydomain.Actor, version int64) (any, error) {
		return services.Agents.DecideApproval(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.DraftId, version, request.Approved, actor.UserID, request.Reason)
	}, &agentlifecyclev1.ReleaseApproval{}, func(value any) any { return newApprovalResponse(value.(agentdomain.ReleaseApproval)) })
}

func (service *GeneratedServices) PublishAgentDraft(ctx context.Context, request *agentlifecyclev1.PublishAgentDraftRequest) (*agentlifecyclev1.AgentRelease, error) {
	if err := validAgentIDs(request.AgentId, request.DraftId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return agentWrite(service, ctx, actor, "agent-release.publish:"+request.DraftId, "", request, http.StatusCreated, &agentlifecyclev1.AgentRelease{}, func(services transaction.TransactionServices) (any, error) {
		return services.Agents.Publish(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.DraftId, actor.UserID)
	}, func(value any) any { return newReleaseResponse(value.(agentdomain.Release)) })
}

func (service *GeneratedServices) ListAgentReleases(ctx context.Context, request *agentlifecyclev1.ListAgentReleasesRequest) (*agentlifecyclev1.ListAgentReleasesResponse, error) {
	if err := validAgentIDs(request.AgentId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.AgentLifecycle.ListReleases(ctx, actor.OrganizationID, actor.TeamID, request.AgentId)
	if err != nil {
		return nil, agentQueryError(err)
	}
	items := make([]releaseResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newReleaseResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &agentlifecyclev1.ListAgentReleasesResponse{})
}

func (service *GeneratedServices) GetAgentRelease(ctx context.Context, request *agentlifecyclev1.GetAgentReleaseRequest) (*agentlifecyclev1.AgentRelease, error) {
	if err := validAgentIDs(request.AgentId, request.ReleaseId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.AgentLifecycle.GetRelease(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, request.ReleaseId)
	if err != nil {
		return nil, agentQueryError(err)
	}
	return mappedResponse(ctx, http.StatusOK, newReleaseResponse(value), &agentlifecyclev1.AgentRelease{})
}

func (service *GeneratedServices) DeprecateAgentRelease(ctx context.Context, request *agentlifecyclev1.DeprecateAgentReleaseRequest) (*agentlifecyclev1.AgentRelease, error) {
	return releaseStatus(service, ctx, request, request.AgentId, request.ReleaseId, request.TeamId, false)
}
func (service *GeneratedServices) BlockAgentRelease(ctx context.Context, request *agentlifecyclev1.BlockAgentReleaseRequest) (*agentlifecyclev1.AgentRelease, error) {
	return releaseStatus(service, ctx, request, request.AgentId, request.ReleaseId, request.TeamId, true)
}

func agentConfiguration(input *typesv1.AgentConfiguration) (agentdomain.Configuration, error) {
	if input == nil {
		return agentdomain.Configuration{}, publicError(400, "invalid_request_body")
	}
	result := agentdomain.Configuration{Instructions: input.Instructions, RepositoryBindingID: input.RepositoryBindingId, RuntimeImageID: input.RuntimeImageId, ConfiguredModelID: input.ConfiguredModelId, NativeSubagents: input.NativeSubagents}
	if input.ModelBudget != nil {
		result.ModelBudget = agentdomain.ModelBudget{MaxInputTokens: input.ModelBudget.MaxInputTokens, MaxOutputTokens: input.ModelBudget.MaxOutputTokens, MaxCostAmount: input.ModelBudget.MaxCostAmount}
	}
	if input.ExecutionLimits != nil {
		result.ExecutionLimits = agentdomain.ExecutionLimits{TimeoutSeconds: input.ExecutionLimits.TimeoutSeconds, CPUs: input.ExecutionLimits.Cpus, MemoryBytes: input.ExecutionLimits.MemoryBytes, PIDs: input.ExecutionLimits.Pids, TempBytes: input.ExecutionLimits.TempBytes, Egress: input.ExecutionLimits.Egress}
	}
	return result, nil
}

func agentWrite[T proto.Message](service *GeneratedServices, ctx context.Context, actor identitydomain.Actor, operation, version string, request proto.Message, status int, output T, call func(transaction.TransactionServices) (any, error), convert func(any) any) (T, error) {
	result, err := service.executeWrite(ctx, actor, operation, version, request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := call(services)
		return encodeWriteResult(status, convert(value), err)
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return mappedWriteResponse(ctx, result, output)
}

func draftAction[T proto.Message](service *GeneratedServices, ctx context.Context, request proto.Message, agentID, draftID, teamID, operation string, call func(transaction.TransactionServices, identitydomain.Actor, int64) (any, error), output T, convert func(any) any) (T, error) {
	if err := validAgentIDs(agentID, draftID); err != nil {
		var zero T
		return zero, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	actor, err := service.authorizeAgentBuild(ctx, teamID)
	if err != nil {
		var zero T
		return zero, err
	}
	return agentWrite(service, ctx, actor, operation+draftID, strconv.FormatInt(version, 10), request, http.StatusOK, output, func(services transaction.TransactionServices) (any, error) { return call(services, actor, version) }, convert)
}

func releaseStatus(service *GeneratedServices, ctx context.Context, request proto.Message, agentID, releaseID, teamID string, block bool) (*agentlifecyclev1.AgentRelease, error) {
	if err := validAgentIDs(agentID, releaseID); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeAgentBuild(ctx, teamID)
	if block {
		actor, err = service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
		actor.TeamID = teamID
	}
	if err != nil {
		return nil, mapAuthorizationError(err, "agent_build_access_denied")
	}
	operation := "agent-release.deprecate:"
	if block {
		operation = "agent-release.block:"
	}
	return agentWrite(service, ctx, actor, operation+releaseID, strconv.FormatInt(version, 10), request, http.StatusOK, &agentlifecyclev1.AgentRelease{}, func(services transaction.TransactionServices) (any, error) {
		if block {
			return services.Agents.BlockRelease(ctx, actor.OrganizationID, actor.TeamID, agentID, releaseID, version)
		}
		return services.Agents.DeprecateRelease(ctx, actor.OrganizationID, actor.TeamID, agentID, releaseID, version)
	}, func(value any) any { return newReleaseResponse(value.(agentdomain.Release)) })
}
