package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/feishucli"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cliConnectorRepository interface {
	ListCLIConnectorDefinitions(context.Context, bool) ([]cliconnector.Definition, error)
	CreateCLIConnectorDefinition(context.Context, string, cliconnector.Definition) (cliconnector.Definition, error)
	UpdateCLIConnectorDefinition(context.Context, string, cliconnector.Definition, int64) (cliconnector.Definition, error)
	PublishCLIConnectorDefinition(context.Context, string, int64) (cliconnector.Definition, error)
	DisableCLIConnectorDefinition(context.Context, string, int64) (cliconnector.Definition, error)
	GetAvailableCLIConnectorDefinition(context.Context, string) (cliconnector.Definition, error)
	GetCLIConnectorEnablement(context.Context, string, string) (cliconnector.Enablement, error)
	EnableCLIConnector(context.Context, string, string) (cliconnector.Enablement, error)
	BeginFeishuCLIConnectorEnablement(context.Context, string, string, string, time.Time, []byte) (cliconnector.Enablement, error)
	GetFeishuCLIConnectorRegistration(context.Context, string, string) (cliconnector.EnablementRegistration, error)
	CompleteFeishuCLIConnectorEnablement(context.Context, string, string, []byte, []byte, string, string) (cliconnector.Enablement, error)
	InvalidateCLIConnectorEnablement(context.Context, string, string) (cliconnector.Enablement, error)
	ListCLIConnectorEnablements(context.Context, string) ([]cliconnector.Enablement, error)
	GetFeishuCLIApplicationCredentials(context.Context, string, string) (cliconnector.FeishuApplicationCredentials, error)
	GetCLIConnectorAuthorizationPolicy(context.Context, string, string) (cliconnector.Definition, error)
	BeginCLIConnectorAuthorization(context.Context, string, string, cliconnector.Identity, []string, string, time.Time, []byte) (cliconnector.AuthorizationAttempt, error)
	GetCLIConnectorAuthorizationAttempt(context.Context, string, string) (cliconnector.AuthorizationAttempt, error)
	CompleteCLIConnectorAuthorization(context.Context, cliconnector.AuthorizationAttempt, string, string, []string, []byte, []byte, time.Time) (cliconnector.Authorization, error)
	DeleteCLIConnectorAuthorizationAttempt(context.Context, string, string) error
	ListCLIConnectorAuthorizations(context.Context, string, string) ([]cliconnector.Authorization, error)
	DisconnectCLIConnectorAuthorization(context.Context, string, string, int64) (cliconnector.Authorization, error)
	ListCommandApprovals(context.Context, string, time.Time) ([]workspacedomain.CommandApproval, error)
	DecideCommandApproval(context.Context, string, string, workspacedomain.ApprovalState, workspacedomain.ExecutionIdentity, int64, time.Time) (workspacedomain.CommandApproval, error)
}

type feishuApplicationRegistrar interface {
	Begin(context.Context) (feishucli.Registration, error)
	Poll(context.Context, string) (feishucli.Application, error)
	BeginAuthorization(context.Context, string, string, []string) (feishucli.AuthorizationRequest, error)
	PollAuthorization(context.Context, string, string, string) (feishucli.Authorization, error)
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
		mutable := principal.Administrator && (item.State == cliconnector.StateDraft || item.State == cliconnector.StateFailed)
		response = append(response, cliDefinitionResponse(item, mutable))
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

func (service *Service) PublishCLIConnectorDefinition(ctx context.Context, request *workspacev1.PublishCLIConnectorDefinitionRequest) (*workspacev1.CLIConnectorDefinition, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.PublishCLIConnectorDefinition(ctx, request.DefinitionId, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return cliDefinitionResponse(item, false), nil
}

func (service *Service) DisableCLIConnectorDefinition(ctx context.Context, request *workspacev1.DisableCLIConnectorDefinitionRequest) (*workspacev1.CLIConnectorDefinition, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.DisableCLIConnectorDefinition(ctx, request.DefinitionId, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return cliDefinitionResponse(item, false), nil
}

func (service *Service) EnableCLIConnector(ctx context.Context, request *workspacev1.EnableCLIConnectorRequest) (*workspacev1.CLIConnectorEnablement, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	definition, err := repository.GetAvailableCLIConnectorDefinition(ctx, request.DefinitionId)
	if err != nil {
		return nil, publicError(err)
	}
	if definition.AuthenticationDriver == "none" {
		item, enableErr := repository.EnableCLIConnector(ctx, principal.UserID, definition.ID)
		if enableErr != nil {
			return nil, publicError(enableErr)
		}
		return cliEnablementResponse(item), nil
	}
	if existing, existingErr := repository.GetCLIConnectorEnablement(ctx, principal.UserID, definition.ID); existingErr == nil {
		if existing.State == "enabled" || existing.State == "waiting_for_user" && existing.ActionExpiresAt != nil && time.Now().UTC().Before(*existing.ActionExpiresAt) {
			return cliEnablementResponse(existing), nil
		}
	} else if !errors.Is(existingErr, workspacedomain.ErrNotFound) {
		return nil, publicError(existingErr)
	}
	registration, err := service.feishu.Begin(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Encrypt([]byte(registration.DeviceCode), feishuRegistrationAAD(principal.UserID, definition.ID))
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.BeginFeishuCLIConnectorEnablement(ctx, principal.UserID, definition.ID, registration.ActionURL, registration.ExpiresAt, deviceCode)
	if err != nil {
		return nil, publicError(err)
	}
	return cliEnablementResponse(item), nil
}

func (service *Service) CompleteCLIConnectorEnablement(ctx context.Context, request *workspacev1.CompleteCLIConnectorEnablementRequest) (*workspacev1.CLIConnectorEnablement, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	registration, err := repository.GetFeishuCLIConnectorRegistration(ctx, principal.UserID, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Decrypt(registration.DeviceCodeCiphertext, feishuRegistrationAAD(principal.UserID, registration.DefinitionID))
	if err != nil {
		return nil, publicError(err)
	}
	application, err := service.feishu.Poll(ctx, string(deviceCode))
	if errors.Is(err, feishucli.ErrPending) {
		return cliEnablementResponse(registration.Enablement), nil
	}
	if errors.Is(err, feishucli.ErrDenied) || errors.Is(err, feishucli.ErrExpired) {
		item, invalidErr := repository.InvalidateCLIConnectorEnablement(ctx, principal.UserID, registration.ID)
		if invalidErr != nil {
			return nil, publicError(invalidErr)
		}
		return cliEnablementResponse(item), nil
	}
	if err != nil {
		return nil, publicError(err)
	}
	appID, err := service.box.Encrypt([]byte(application.AppID), feishuApplicationAAD(principal.UserID))
	if err != nil {
		return nil, publicError(err)
	}
	appSecret, err := service.box.Encrypt([]byte(application.AppSecret), feishuApplicationAAD(principal.UserID))
	if err != nil {
		return nil, publicError(err)
	}
	userName := strings.TrimSpace(application.UserName)
	if userName == "" {
		userName = strings.TrimSpace(principal.DisplayName)
	}
	if userName == "" {
		userName = principal.Username
	}
	providerName := userName + "的飞书CLI"
	consoleURL := "https://open.feishu.cn/app/" + url.PathEscape(application.AppID)
	item, err := repository.CompleteFeishuCLIConnectorEnablement(ctx, principal.UserID, registration.ID, appID, appSecret, providerName, consoleURL)
	if err != nil {
		return nil, publicError(err)
	}
	return cliEnablementResponse(item), nil
}

func feishuRegistrationAAD(ownerID, definitionID string) string {
	return "feishu-cli-registration:" + ownerID + ":" + definitionID
}

func feishuApplicationAAD(ownerID string) string { return "feishu-cli-application:" + ownerID }

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

func (service *Service) BeginCLIConnectorAuthorization(ctx context.Context, request *workspacev1.BeginCLIConnectorAuthorizationRequest) (*workspacev1.CLIConnectorAuthorizationFlow, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	identity := cliconnector.Identity(request.Identity)
	if identity != cliconnector.IdentityUser {
		return nil, publicError(fmt.Errorf("%w: interactive authorization only supports user identity", workspacedomain.ErrInvalid))
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	policy, err := repository.GetCLIConnectorAuthorizationPolicy(ctx, principal.UserID, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	allowedScopes := make(map[string]struct{})
	for _, capability := range policy.Capabilities {
		if slices.Contains(capability.Identities, cliconnector.IdentityUser) {
			for _, scope := range capability.Scopes {
				allowedScopes[scope] = struct{}{}
			}
		}
	}
	for _, scope := range request.Scopes {
		if _, allowed := allowedScopes[scope]; !allowed {
			return nil, publicError(fmt.Errorf("%w: requested scope is outside the reviewed Connector policy", workspacedomain.ErrInvalid))
		}
	}
	credentials, err := repository.GetFeishuCLIApplicationCredentials(ctx, principal.UserID, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	appID, appSecret, err := service.decryptFeishuApplication(principal.UserID, credentials)
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := service.feishu.BeginAuthorization(ctx, appID, appSecret, request.Scopes)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Encrypt([]byte(flow.DeviceCode), feishuAuthorizationFlowAAD(principal.UserID, request.EnablementId, identity))
	if err != nil {
		return nil, publicError(err)
	}
	attempt, err := repository.BeginCLIConnectorAuthorization(ctx, principal.UserID, request.EnablementId, identity, flow.Scopes, flow.ActionURL, flow.ExpiresAt, deviceCode)
	if err != nil {
		return nil, publicError(err)
	}
	return cliAuthorizationFlowResponse(attempt, "waiting_for_user", nil), nil
}

func (service *Service) CompleteCLIConnectorAuthorization(ctx context.Context, request *workspacev1.CompleteCLIConnectorAuthorizationRequest) (*workspacev1.CLIConnectorAuthorizationFlow, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	attempt, err := repository.GetCLIConnectorAuthorizationAttempt(ctx, principal.UserID, request.FlowId)
	if err != nil {
		return nil, publicError(err)
	}
	credentials, err := repository.GetFeishuCLIApplicationCredentials(ctx, principal.UserID, attempt.EnablementID)
	if err != nil {
		return nil, publicError(err)
	}
	appID, appSecret, err := service.decryptFeishuApplication(principal.UserID, credentials)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Decrypt(attempt.DeviceCodeCiphertext, feishuAuthorizationFlowAAD(principal.UserID, attempt.EnablementID, attempt.Identity))
	if err != nil {
		return nil, publicError(err)
	}
	result, err := service.feishu.PollAuthorization(ctx, appID, appSecret, string(deviceCode))
	if errors.Is(err, feishucli.ErrPending) {
		return cliAuthorizationFlowResponse(attempt, "waiting_for_user", nil), nil
	}
	if errors.Is(err, feishucli.ErrDenied) || errors.Is(err, feishucli.ErrExpired) {
		_ = repository.DeleteCLIConnectorAuthorizationAttempt(ctx, principal.UserID, attempt.ID)
		return cliAuthorizationFlowResponse(attempt, "invalid", nil), nil
	}
	if err != nil {
		return nil, publicError(err)
	}
	tokenAAD := feishuAuthorizationTokenAAD(principal.UserID, attempt.EnablementID, result.ExternalID)
	token, err := service.box.Encrypt([]byte(result.AccessToken), tokenAAD)
	if err != nil {
		return nil, publicError(err)
	}
	var refreshToken []byte
	if result.RefreshToken != "" {
		refreshToken, err = service.box.Encrypt([]byte(result.RefreshToken), tokenAAD+":refresh")
		if err != nil {
			return nil, publicError(err)
		}
	}
	authorization, err := repository.CompleteCLIConnectorAuthorization(ctx, attempt, result.ExternalID, result.DisplayName, result.Scopes, token, refreshToken, result.ExpiresAt)
	if err != nil {
		return nil, publicError(err)
	}
	return cliAuthorizationFlowResponse(attempt, "completed", &authorization), nil
}

func (service *Service) ListCLIConnectorAuthorizations(ctx context.Context, request *workspacev1.ListCLIConnectorAuthorizationsRequest) (*workspacev1.ListCLIConnectorAuthorizationsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListCLIConnectorAuthorizations(ctx, owner, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.CLIConnectorAuthorization, 0, len(items))
	for _, item := range items {
		response = append(response, cliAuthorizationResponse(item))
	}
	return &workspacev1.ListCLIConnectorAuthorizationsResponse{Items: response}, nil
}

func (service *Service) DisconnectCLIConnectorAuthorization(ctx context.Context, request *workspacev1.DisconnectCLIConnectorAuthorizationRequest) (*workspacev1.CLIConnectorAuthorization, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.DisconnectCLIConnectorAuthorization(ctx, principal.UserID, request.AuthorizationId, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return cliAuthorizationResponse(item), nil
}

func (service *Service) decryptFeishuApplication(ownerID string, credentials cliconnector.FeishuApplicationCredentials) (string, string, error) {
	appID, err := service.box.Decrypt(credentials.AppIDCiphertext, feishuApplicationAAD(ownerID))
	if err != nil {
		return "", "", err
	}
	appSecret, err := service.box.Decrypt(credentials.AppSecretCiphertext, feishuApplicationAAD(ownerID))
	if err != nil {
		return "", "", err
	}
	return string(appID), string(appSecret), nil
}

func feishuAuthorizationFlowAAD(ownerID, enablementID string, identity cliconnector.Identity) string {
	return "feishu-cli-authorization-flow:" + ownerID + ":" + enablementID + ":" + string(identity)
}

func feishuAuthorizationTokenAAD(ownerID, enablementID, externalID string) string {
	return "feishu-cli-authorization-token:" + ownerID + ":" + enablementID + ":" + externalID
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
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if principal.Administrator {
		return nil, publicError(accountdomain.ErrForbidden)
	}
	owner := principal.UserID
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
	value := cliconnector.Definition{Name: input.Name, Package: input.NpmPackage, Version: input.NpmVersion, Integrity: input.NpmIntegrity, Executable: input.Executable, AuthenticationDriver: input.AuthenticationDriver, Capabilities: capabilities, SupportedArchitectures: append([]string(nil), input.SupportedArchitectures...), RecommendedSkillIDs: append([]string(nil), input.RecommendedSkillIds...)}
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
	response := &workspacev1.CLIConnectorDefinition{Id: item.ID, Name: item.Name, NpmPackage: item.Package, NpmVersion: item.Version, NpmIntegrity: item.Integrity, Executable: item.Executable, AuthenticationDriver: item.AuthenticationDriver, Capabilities: capabilities, State: string(item.State), Mutable: mutable, Version: item.VersionNumber, SupportedArchitectures: item.SupportedArchitectures, RecommendedSkillIds: item.RecommendedSkillIDs, ConformanceRuntimeDigests: item.RuntimeDigests}
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
	if item.ProviderName != "" {
		response.ProviderName = &item.ProviderName
	}
	if item.DeveloperConsoleURL != "" {
		response.DeveloperConsoleUrl = &item.DeveloperConsoleURL
	}
	return response
}
func cliAuthorizationResponse(item cliconnector.Authorization) *workspacev1.CLIConnectorAuthorization {
	response := &workspacev1.CLIConnectorAuthorization{
		Id: item.ID, EnablementId: item.EnablementID, Identity: string(item.Identity), ExternalIdentityId: item.ExternalIdentityID,
		ExternalDisplayName: item.ExternalDisplayName, Scopes: item.Scopes, State: item.State, Version: item.Version,
	}
	if item.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*item.ExpiresAt)
	}
	return response
}
func cliAuthorizationFlowResponse(item cliconnector.AuthorizationAttempt, state string, authorization *cliconnector.Authorization) *workspacev1.CLIConnectorAuthorizationFlow {
	response := &workspacev1.CLIConnectorAuthorizationFlow{
		Id: item.ID, EnablementId: item.EnablementID, Identity: string(item.Identity), Scopes: item.Scopes, State: state,
	}
	if state == "waiting_for_user" {
		response.ActionUrl = &item.ActionURL
		response.ExpiresAt = timestamppb.New(item.ExpiresAt)
	}
	if authorization != nil {
		response.Authorization = cliAuthorizationResponse(*authorization)
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
