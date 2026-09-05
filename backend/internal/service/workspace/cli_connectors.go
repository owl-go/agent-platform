package workspace

import (
	"context"
	"fmt"
	"net/url"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cliConnectorRepository interface {
	ListCLIConnectorDefinitions(context.Context, bool) ([]cliconnector.Definition, error)
	CreateCLIConnectorDefinition(context.Context, string, cliconnector.Definition) (cliconnector.Definition, error)
	UpdateCLIConnectorDefinition(context.Context, string, cliconnector.Definition, int64) (cliconnector.Definition, error)
	DeleteCLIConnectorDefinition(context.Context, string) error
	EnableCLIConnector(context.Context, string, string, string, time.Time) (cliconnector.Enablement, error)
	ListCLIConnectorEnablements(context.Context, string) ([]cliconnector.Enablement, error)
	ListCommandApprovals(context.Context, string, time.Time) ([]workspacedomain.CommandApproval, error)
	DecideCommandApproval(context.Context, string, string, workspacedomain.ApprovalState, workspacedomain.ExecutionIdentity, int64, time.Time) (workspacedomain.CommandApproval, error)
}

func (service *Service) cliConnectors() (cliConnectorRepository, error) {
	repository, ok := service.workspace.Repository().(cliConnectorRepository)
	if !ok {
		return nil, fmt.Errorf("CLI Connector repository is unavailable")
	}
	return repository, nil
}

func (service *Service) ListCLIConnectorDefinitions(ctx context.Context, _ *workspacev1.ListCLIConnectorDefinitionsRequest) (*workspacev1.ListCLIConnectorDefinitionsResponse, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListCLIConnectorDefinitions(ctx, principal.Administrator)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.CLIConnectorDefinition, 0, len(items))
	for _, item := range items {
		response = append(response, cliDefinitionResponse(item, principal.Administrator))
	}
	return &workspacev1.ListCLIConnectorDefinitionsResponse{Items: response}, nil
}

func (service *Service) CreateCLIConnectorDefinition(ctx context.Context, request *workspacev1.CreateCLIConnectorDefinitionRequest) (*workspacev1.CLIConnectorDefinition, error) {
	principal, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	input, err := cliDefinitionInput(request.Definition)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.CreateCLIConnectorDefinition(ctx, principal.UserID, input)
	if err != nil {
		return nil, publicError(err)
	}
	return cliDefinitionResponse(item, true), nil
}

func (service *Service) UpdateCLIConnectorDefinition(ctx context.Context, request *workspacev1.UpdateCLIConnectorDefinitionRequest) (*workspacev1.CLIConnectorDefinition, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	input, err := cliDefinitionInput(request.Definition)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.UpdateCLIConnectorDefinition(ctx, request.DefinitionId, input, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return cliDefinitionResponse(item, true), nil
}

func (service *Service) DeleteCLIConnectorDefinition(ctx context.Context, request *workspacev1.DeleteCLIConnectorDefinitionRequest) (*workspacev1.DeleteResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	if err := repository.DeleteCLIConnectorDefinition(ctx, request.DefinitionId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) EnableCLIConnector(ctx context.Context, request *workspacev1.EnableCLIConnectorRequest) (*workspacev1.CLIConnectorEnablement, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	action := "https://open.feishu.cn/app?" + url.Values{"source": {"agent-workspace"}}.Encode()
	item, err := repository.EnableCLIConnector(ctx, owner, request.DefinitionId, action, expires)
	if err != nil {
		return nil, publicError(err)
	}
	return cliEnablementResponse(item), nil
}

func (service *Service) ListCLIConnectorEnablements(ctx context.Context, _ *workspacev1.ListCLIConnectorEnablementsRequest) (*workspacev1.ListCLIConnectorEnablementsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListCLIConnectorEnablements(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.CLIConnectorEnablement, 0, len(items))
	for _, item := range items {
		response = append(response, cliEnablementResponse(item))
	}
	return &workspacev1.ListCLIConnectorEnablementsResponse{Items: response}, nil
}

func (service *Service) ListCommandApprovals(ctx context.Context, _ *workspacev1.ListCommandApprovalsRequest) (*workspacev1.ListCommandApprovalsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListCommandApprovals(ctx, owner, time.Now().UTC())
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.CommandApproval, 0, len(items))
	for _, item := range items {
		response = append(response, commandApprovalResponse(item))
	}
	return &workspacev1.ListCommandApprovalsResponse{Items: response}, nil
}

func (service *Service) DecideCommandApproval(ctx context.Context, request *workspacev1.DecideCommandApprovalRequest) (*workspacev1.CommandApproval, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	decision := workspacedomain.ApprovalState(request.Decision)
	if decision != workspacedomain.ApprovalApproved && decision != workspacedomain.ApprovalRejected {
		return nil, publicError(fmt.Errorf("%w: decision must be approved or rejected", workspacedomain.ErrInvalid))
	}
	identity := workspacedomain.ExecutionIdentity("")
	if request.Identity != nil {
		identity = workspacedomain.ExecutionIdentity(*request.Identity)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.DecideCommandApproval(ctx, owner, request.ApprovalId, decision, identity, request.ExpectedVersion, time.Now().UTC())
	if err != nil {
		return nil, publicError(err)
	}
	return commandApprovalResponse(item), nil
}

func cliDefinitionInput(input *workspacev1.CLIConnectorDefinitionInput) (cliconnector.Definition, error) {
	if input == nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: CLI Connector Definition is required", workspacedomain.ErrInvalid)
	}
	capabilities := make([]cliconnector.Capability, 0, len(input.Capabilities))
	for _, item := range input.Capabilities {
		identities := make([]cliconnector.Identity, 0, len(item.Identities))
		for _, identity := range item.Identities {
			identities = append(identities, cliconnector.Identity(identity))
		}
		capabilities = append(capabilities, cliconnector.Capability{ID: item.Id, ArgvPrefix: append([]string(nil), item.ArgvPrefix...), Risk: cliconnector.Risk(item.Risk), Identities: identities, Scopes: append([]string(nil), item.Scopes...), EgressHosts: append([]string(nil), item.EgressHosts...), Timeout: time.Duration(item.TimeoutSeconds) * time.Second})
	}
	value := cliconnector.Definition{Name: input.Name, Package: input.NpmPackage, Version: input.NpmVersion, Integrity: input.NpmIntegrity, Executable: input.Executable, AuthenticationDriver: input.AuthenticationDriver, Capabilities: capabilities}
	if err := value.Validate(); err != nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err)
	}
	return value, nil
}
func cliDefinitionResponse(item cliconnector.Definition, mutable bool) *workspacev1.CLIConnectorDefinition {
	capabilities := make([]*workspacev1.CLICapability, 0, len(item.Capabilities))
	for _, value := range item.Capabilities {
		identities := make([]string, 0, len(value.Identities))
		for _, identity := range value.Identities {
			identities = append(identities, string(identity))
		}
		capabilities = append(capabilities, &workspacev1.CLICapability{Id: value.ID, ArgvPrefix: value.ArgvPrefix, Risk: string(value.Risk), Identities: identities, Scopes: value.Scopes, EgressHosts: value.EgressHosts, TimeoutSeconds: int32(value.Timeout / time.Second)})
	}
	response := &workspacev1.CLIConnectorDefinition{Id: item.ID, Name: item.Name, NpmPackage: item.Package, NpmVersion: item.Version, NpmIntegrity: item.Integrity, Executable: item.Executable, AuthenticationDriver: item.AuthenticationDriver, Capabilities: capabilities, State: string(item.State), Mutable: mutable, Version: item.VersionNumber}
	if item.FailureReason != "" {
		response.FailureReason = &item.FailureReason
	}
	if item.BundleSHA256 != "" {
		response.BundleSha256 = &item.BundleSHA256
	}
	return response
}
func cliEnablementResponse(item cliconnector.Enablement) *workspacev1.CLIConnectorEnablement {
	response := &workspacev1.CLIConnectorEnablement{Id: item.ID, DefinitionId: item.DefinitionID, State: item.State, Version: item.Version}
	if item.ActionURL != "" {
		response.ActionUrl = &item.ActionURL
	}
	if item.ActionExpiresAt != nil {
		response.ActionExpiresAt = timestamppb.New(*item.ActionExpiresAt)
	}
	return response
}
func commandApprovalResponse(item workspacedomain.CommandApproval) *workspacev1.CommandApproval {
	response := &workspacev1.CommandApproval{Id: item.ID, ExecutionKind: item.ExecutionKind, ExecutionId: item.ExecutionID, ConnectorName: item.ConnectorName, Operation: item.Operation, Target: item.Target, RedactedArguments: item.RedactedArguments, State: string(item.State), ExpiresAt: timestamppb.New(item.ExpiresAt), Version: item.Version}
	if item.Identity != "" {
		value := string(item.Identity)
		response.Identity = &value
	}
	return response
}
