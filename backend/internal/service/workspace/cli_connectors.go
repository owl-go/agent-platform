package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/feishucli"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cliConnectorRepository interface {
	ListCLIConnectorDefinitions(context.Context, bool) ([]cliconnector.Definition, error)
	ListCLIConnectorHealth(context.Context, time.Time) ([]cliconnector.Health, error)
	CreateCLIConnectorDefinition(context.Context, string, cliconnector.Definition) (cliconnector.Definition, error)
	UpdateCLIConnectorDefinition(context.Context, string, cliconnector.Definition, int64) (cliconnector.Definition, error)
	PublishCLIConnectorDefinition(context.Context, string, int64) (cliconnector.Definition, error)
	DisableCLIConnectorDefinition(context.Context, string, int64) (cliconnector.Definition, error)
	DeleteCLIConnectorDefinition(context.Context, string, int64) error
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
	ListConnectorActions(context.Context, string, time.Time) ([]cliconnector.ActionRequirement, error)
	GetConnectorAction(context.Context, string, string) (cliconnector.ActionRequirement, error)
	ResetConnectorActionProviderURL(context.Context, string, string) (cliconnector.ActionRequirement, error)
	ConsumeConnectorActionProviderURL(context.Context, string, int64, string) (string, error)
	UpdateConnectorActionReason(context.Context, string, string, cliconnector.ActionReason) (cliconnector.ActionRequirement, error)
	ResolveConnectorAction(context.Context, string, string) (cliconnector.ActionRequirement, error)
	CancelConnectorAction(context.Context, string, string) (cliconnector.ActionRequirement, error)
	GetCLIConnectorAuthorizationAttemptForEnablement(context.Context, string, string) (cliconnector.AuthorizationAttempt, error)
	ListCommandApprovals(context.Context, string, time.Time) ([]workspacedomain.CommandApproval, error)
	DecideCommandApproval(context.Context, string, string, workspacedomain.ApprovalState, workspacedomain.ExecutionIdentity, int64, time.Time) (workspacedomain.CommandApproval, error)
}

func (service *Service) ListCLIConnectorHealth(ctx context.Context, _ *workspacev1.ListCLIConnectorHealthRequest) (*workspacev1.ListCLIConnectorHealthResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListCLIConnectorHealth(ctx, time.Now().UTC())
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.CLIConnectorHealth, 0, len(items))
	for _, item := range items {
		response = append(response, &workspacev1.CLIConnectorHealth{
			DefinitionId: item.DefinitionID, DefinitionName: item.DefinitionName, DefinitionState: string(item.DefinitionState),
			EnablementCount: item.EnablementCount, EnabledCount: item.EnabledCount, WaitingForUserCount: item.WaitingForUserCount,
			ActiveAuthorizationCount: item.ActiveAuthorizationCount, AttentionAuthorizationCount: item.AttentionAuthorizationCount,
		})
	}
	return &workspacev1.ListCLIConnectorHealthResponse{Items: response}, nil
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
		mutable := principal.Administrator && item.State != cliconnector.StateBuilding && item.State != cliconnector.StateTesting
		response = append(response, cliDefinitionResponse(item, mutable))
	}
	return &workspacev1.ListCLIConnectorDefinitionsResponse{Items: response}, nil
}

func (service *Service) CreateCLIConnectorDefinition(ctx context.Context, request *workspacev1.CreateCLIConnectorDefinitionRequest) (*workspacev1.CLIConnectorDefinition, error) {
	principal, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	input, err := service.cliDefinitionInput(ctx, request.Definition, uuid.NewString(), 1)
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
	input, err := service.cliDefinitionInput(ctx, request.Definition, request.DefinitionId, request.ExpectedVersion+1)
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

func (service *Service) DeleteCLIConnectorDefinition(ctx context.Context, request *workspacev1.DeleteCLIConnectorDefinitionRequest) (*workspacev1.DeleteResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	if request.ExpectedVersion < 1 {
		return nil, publicError(workspacedomain.ErrInvalid)
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	if err := repository.DeleteCLIConnectorDefinition(ctx, request.DefinitionId, request.ExpectedVersion); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
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
	item, err := repository.EnableCLIConnector(ctx, principal.UserID, definition.ID)
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
	adapter, err := service.authorizationAdapter("feishu")
	if err != nil {
		return nil, publicError(err)
	}
	application, err := adapter.PollSetup(ctx, deviceCode)
	if errors.Is(err, cliconnector.ErrAuthorizationPending) {
		return cliEnablementResponse(registration.Enablement), nil
	}
	if errors.Is(err, cliconnector.ErrAuthorizationDenied) || errors.Is(err, cliconnector.ErrAuthorizationExpired) {
		item, invalidErr := repository.InvalidateCLIConnectorEnablement(ctx, principal.UserID, registration.ID)
		if invalidErr != nil {
			return nil, publicError(invalidErr)
		}
		return cliEnablementResponse(item), nil
	}
	if err != nil {
		return nil, publicError(err)
	}
	appIDValue, appSecretValue := application.CredentialSlots["app_id"], application.CredentialSlots["app_secret"]
	if appIDValue == "" || appSecretValue == "" {
		return nil, publicError(fmt.Errorf("Connector Setup returned incomplete credential slots"))
	}
	appID, err := service.box.Encrypt([]byte(appIDValue), connectorSetupAAD(principal.UserID, registration.ID))
	if err != nil {
		return nil, publicError(err)
	}
	appSecret, err := service.box.Encrypt([]byte(appSecretValue), connectorSetupAAD(principal.UserID, registration.ID))
	if err != nil {
		return nil, publicError(err)
	}
	userName := strings.TrimSpace(application.DisplayName)
	if userName == "" {
		userName = strings.TrimSpace(principal.DisplayName)
	}
	if userName == "" {
		userName = principal.Username
	}
	providerName := userName + "的飞书CLI"
	consoleURL := application.ManagementURL
	item, err := repository.CompleteFeishuCLIConnectorEnablement(ctx, principal.UserID, registration.ID, appID, appSecret, providerName, consoleURL)
	if err != nil {
		return nil, publicError(err)
	}
	return cliEnablementResponse(item), nil
}

func feishuRegistrationAAD(ownerID, definitionID string) string {
	return "feishu-cli-registration:" + ownerID + ":" + definitionID
}

func connectorSetupAAD(ownerID, enablementID string) string {
	return "connector-setup:" + ownerID + ":" + enablementID
}

func legacyFeishuApplicationAAD(ownerID string) string { return "feishu-cli-application:" + ownerID }

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
	requestedScopes := append([]string(nil), request.Scopes...)
	existing, err := repository.ListCLIConnectorAuthorizations(ctx, principal.UserID, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	for _, authorization := range existing {
		if authorization.State != "active" {
			continue
		}
		for _, scope := range authorization.Scopes {
			if _, allowed := allowedScopes[scope]; allowed && !slices.Contains(requestedScopes, scope) {
				requestedScopes = append(requestedScopes, scope)
			}
		}
	}
	credentials, err := repository.GetFeishuCLIApplicationCredentials(ctx, principal.UserID, request.EnablementId)
	if err != nil {
		return nil, publicError(err)
	}
	appID, appSecret, err := service.decryptFeishuApplication(principal.UserID, request.EnablementId, credentials)
	if err != nil {
		return nil, publicError(err)
	}
	adapter, err := service.authorizationAdapter(policy.AuthenticationDriver)
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := adapter.BeginAuthorization(ctx, map[string]string{"app_id": appID, "app_secret": appSecret}, requestedScopes)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Encrypt(flow.Continuation, feishuAuthorizationFlowAAD(principal.UserID, request.EnablementId, identity))
	if err != nil {
		return nil, publicError(err)
	}
	attempt, err := repository.BeginCLIConnectorAuthorization(ctx, principal.UserID, request.EnablementId, identity, flow.Permissions, flow.ActionURL, flow.ExpiresAt, deviceCode)
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
	appID, appSecret, err := service.decryptFeishuApplication(principal.UserID, attempt.EnablementID, credentials)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Decrypt(attempt.DeviceCodeCiphertext, feishuAuthorizationFlowAAD(principal.UserID, attempt.EnablementID, attempt.Identity))
	if err != nil {
		return nil, publicError(err)
	}
	policy, err := repository.GetCLIConnectorAuthorizationPolicy(ctx, principal.UserID, attempt.EnablementID)
	if err != nil {
		return nil, publicError(err)
	}
	adapter, err := service.authorizationAdapter(policy.AuthenticationDriver)
	if err != nil {
		return nil, publicError(err)
	}
	result, err := adapter.PollAuthorization(ctx, map[string]string{"app_id": appID, "app_secret": appSecret}, deviceCode)
	if errors.Is(err, cliconnector.ErrAuthorizationPending) {
		return cliAuthorizationFlowResponse(attempt, "waiting_for_user", nil), nil
	}
	if errors.Is(err, cliconnector.ErrAuthorizationDenied) || errors.Is(err, cliconnector.ErrAuthorizationExpired) {
		_ = repository.DeleteCLIConnectorAuthorizationAttempt(ctx, principal.UserID, attempt.ID)
		return cliAuthorizationFlowResponse(attempt, "invalid", nil), nil
	}
	if err != nil {
		return nil, publicError(err)
	}
	tokenAAD := feishuAuthorizationTokenAAD(principal.UserID, attempt.EnablementID, result.ExternalIdentityID)
	token, err := service.box.Encrypt([]byte(result.CredentialSlots["access_token"]), tokenAAD)
	if err != nil {
		return nil, publicError(err)
	}
	var refreshToken []byte
	if result.CredentialSlots["refresh_token"] != "" {
		refreshToken, err = service.box.Encrypt([]byte(result.CredentialSlots["refresh_token"]), tokenAAD+":refresh")
		if err != nil {
			return nil, publicError(err)
		}
	}
	authorization, err := repository.CompleteCLIConnectorAuthorization(ctx, attempt, result.ExternalIdentityID, result.ExternalDisplayName, result.Permissions, token, refreshToken, result.ExpiresAt)
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

func (service *Service) ListConnectorActions(ctx context.Context, _ *workspacev1.ListConnectorActionsRequest) (*workspacev1.ListConnectorActionsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorActions(ctx, owner, time.Now().UTC())
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.ConnectorActionRequirement, 0, len(items))
	for _, item := range items {
		response = append(response, connectorActionResponseWithPlatformURL(item))
	}
	return &workspacev1.ListConnectorActionsResponse{Items: response}, nil
}

func (service *Service) StartConnectorAction(ctx context.Context, request *workspacev1.StartConnectorActionRequest) (*workspacev1.ConnectorActionRequirement, error) {
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
	action, err := repository.GetConnectorAction(ctx, principal.UserID, request.ActionId)
	if err != nil {
		return nil, publicError(err)
	}
	if action.State != cliconnector.ActionPending || !time.Now().UTC().Before(action.ExpiresAt) {
		return nil, publicError(workspacedomain.ErrConflict)
	}
	if action.Reason == cliconnector.ReasonSetupRequired {
		adapter, adapterErr := service.authorizationAdapter(action.AuthorizationScheme)
		if adapterErr != nil {
			return nil, publicError(adapterErr)
		}
		registration, beginErr := adapter.BeginSetup(ctx)
		if beginErr != nil {
			return nil, publicError(beginErr)
		}
		deviceCode, encryptErr := service.box.Encrypt(registration.Continuation, feishuRegistrationAAD(principal.UserID, action.ConnectorID))
		if encryptErr != nil {
			return nil, publicError(encryptErr)
		}
		enablement, beginErr := repository.BeginFeishuCLIConnectorEnablement(ctx, principal.UserID, action.ConnectorID, registration.ActionURL, registration.ExpiresAt, deviceCode)
		if beginErr != nil {
			return nil, publicError(beginErr)
		}
		if enablement.ActionURL != "" {
			action.ActionURL = enablement.ActionURL
		}
		action, err = repository.ResetConnectorActionProviderURL(ctx, principal.UserID, action.ID)
		if err != nil {
			return nil, publicError(err)
		}
		action.ActionURL = enablement.ActionURL
		return connectorActionResponseWithPlatformURL(action), nil
	}
	flow, err := service.BeginCLIConnectorAuthorization(ctx, &workspacev1.BeginCLIConnectorAuthorizationRequest{EnablementId: action.EnablementID, Identity: string(action.Identity), Scopes: append([]string(nil), action.Permissions...)})
	if err != nil {
		return nil, err
	}
	if flow.ActionUrl != nil {
		action.ActionURL = *flow.ActionUrl
	}
	action, err = repository.ResetConnectorActionProviderURL(ctx, principal.UserID, action.ID)
	if err != nil {
		return nil, publicError(err)
	}
	if flow.ActionUrl != nil {
		action.ActionURL = *flow.ActionUrl
	}
	return connectorActionResponseWithPlatformURL(action), nil
}

func (service *Service) CheckConnectorAction(ctx context.Context, request *workspacev1.CheckConnectorActionRequest) (*workspacev1.ConnectorActionRequirement, error) {
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
	action, err := repository.GetConnectorAction(ctx, principal.UserID, request.ActionId)
	if err != nil {
		return nil, publicError(err)
	}
	if action.State != cliconnector.ActionPending || !time.Now().UTC().Before(action.ExpiresAt) {
		return nil, publicError(workspacedomain.ErrConflict)
	}
	if action.Reason == cliconnector.ReasonSetupRequired {
		enablement, completeErr := service.CompleteCLIConnectorEnablement(ctx, &workspacev1.CompleteCLIConnectorEnablementRequest{EnablementId: action.EnablementID})
		if completeErr != nil {
			return nil, completeErr
		}
		if enablement.State == "waiting_for_user" {
			if enablement.ActionUrl != nil {
				action.ActionURL = *enablement.ActionUrl
			}
			return connectorActionResponseWithPlatformURL(action), nil
		}
		action, err = repository.UpdateConnectorActionReason(ctx, principal.UserID, action.ID, cliconnector.ReasonAuthorizationRequired)
		if err != nil {
			return nil, publicError(err)
		}
		return service.StartConnectorAction(ctx, &workspacev1.StartConnectorActionRequest{ActionId: action.ID})
	}
	attempt, err := repository.GetCLIConnectorAuthorizationAttemptForEnablement(ctx, principal.UserID, action.EnablementID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return service.StartConnectorAction(ctx, &workspacev1.StartConnectorActionRequest{ActionId: action.ID})
	}
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := service.CompleteCLIConnectorAuthorization(ctx, &workspacev1.CompleteCLIConnectorAuthorizationRequest{FlowId: attempt.ID})
	if err != nil {
		return nil, err
	}
	if flow.State == "waiting_for_user" {
		if flow.ActionUrl != nil {
			action.ActionURL = *flow.ActionUrl
		}
		return connectorActionResponseWithPlatformURL(action), nil
	}
	if flow.State != "completed" {
		return nil, publicError(fmt.Errorf("%w: Connector authorization did not complete", workspacedomain.ErrConflict))
	}
	action, err = repository.ResolveConnectorAction(ctx, principal.UserID, action.ID)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorActionResponseWithPlatformURL(action), nil
}

func (service *Service) CancelConnectorAction(ctx context.Context, request *workspacev1.CancelConnectorActionRequest) (*workspacev1.ConnectorActionRequirement, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.cliConnectors()
	if err != nil {
		return nil, publicError(err)
	}
	action, err := repository.CancelConnectorAction(ctx, owner, request.ActionId)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorActionResponse(action), nil
}

func (service *Service) openConnectorAction(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "connector-actions" || parts[4] != "open" {
		http.NotFound(writer, request)
		return
	}
	repository, err := service.cliConnectors()
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	linkVersion, parseErr := strconv.ParseInt(request.URL.Query().Get("version"), 10, 64)
	token := request.URL.Query().Get("token")
	if parseErr != nil || linkVersion <= 0 || token == "" {
		http.NotFound(writer, request)
		return
	}
	providerURL, consumeErr := repository.ConsumeConnectorActionProviderURL(request.Context(), parts[3], linkVersion, token)
	if consumeErr != nil {
		http.NotFound(writer, request)
		return
	}
	parsed, err := url.Parse(providerURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(writer, request, providerURL, http.StatusSeeOther)
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

func (service *Service) decryptFeishuApplication(ownerID, enablementID string, credentials cliconnector.FeishuApplicationCredentials) (string, string, error) {
	appID, err := service.decryptConnectorSetupValue(credentials.AppIDCiphertext, ownerID, enablementID)
	if err != nil {
		return "", "", err
	}
	appSecret, err := service.decryptConnectorSetupValue(credentials.AppSecretCiphertext, ownerID, enablementID)
	if err != nil {
		return "", "", err
	}
	return string(appID), string(appSecret), nil
}

func (service *Service) decryptConnectorSetupValue(ciphertext []byte, ownerID, enablementID string) ([]byte, error) {
	value, err := service.box.Decrypt(ciphertext, connectorSetupAAD(ownerID, enablementID))
	if err == nil {
		return value, nil
	}
	return service.box.Decrypt(ciphertext, legacyFeishuApplicationAAD(ownerID))
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

func (service *Service) cliDefinitionInput(ctx context.Context, input *workspacev1.CLIConnectorDefinitionInput, definitionID string, sourceVersion int64) (cliconnector.Definition, error) {
	if input == nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: CLI Connector Definition is required", workspacedomain.ErrInvalid)
	}
	capabilities := make([]cliconnector.Capability, 0, len(input.Capabilities))
	for _, item := range input.Capabilities {
		identities := make([]cliconnector.Identity, 0, len(item.Identities))
		for _, identity := range item.Identities {
			identities = append(identities, cliconnector.Identity(identity))
		}
		fields := make([]cliconnector.InputField, 0, len(item.InputFields))
		for _, field := range item.InputFields {
			flag := ""
			if field.Flag != nil {
				flag = *field.Flag
			}
			fields = append(fields, cliconnector.InputField{Name: field.Name, Type: cliconnector.InputType(field.Type), Required: field.Required, Flag: flag, Sensitive: field.Sensitive, Enum: append([]string(nil), field.Enum...)})
		}
		capabilities = append(capabilities, cliconnector.Capability{ID: item.Id, DisplayName: maps.Clone(item.DisplayName), OperationPhrase: maps.Clone(item.OperationPhrase), ArgvPrefix: append([]string(nil), item.ArgvPrefix...), Input: cliconnector.InputSchema{Fields: fields}, Risk: cliconnector.Risk(item.Risk), Identities: identities, Scopes: append([]string(nil), item.Scopes...), EgressHosts: append([]string(nil), item.EgressHosts...), Timeout: time.Duration(item.TimeoutSeconds) * time.Second, Idempotency: cliconnector.Idempotency(item.Idempotency)})
	}
	recommendedSkills := make([]cliconnector.RecommendedSkill, 0, len(input.RecommendedSkills))
	for _, skill := range input.RecommendedSkills {
		recommendedSkills = append(recommendedSkills, cliconnector.RecommendedSkill{Name: skill.Name, GitURL: skill.GitUrl, GitRef: skill.GitRef})
	}
	installationType := input.InstallationType
	if installationType == "" {
		installationType = "npm"
	}
	value := cliconnector.Definition{ID: definitionID, Name: input.Name, Icon: input.Icon, Description: input.Description, InstallationType: installationType, Package: input.NpmPackage, Version: input.NpmVersion, Integrity: input.NpmIntegrity, Executable: input.Executable, AuthenticationDriver: input.AuthenticationDriver, Capabilities: capabilities, SupportedArchitectures: append([]string(nil), input.SupportedArchitectures...), RecommendedSkills: recommendedSkills, ManifestVersion: input.ManifestVersion, UsageGuide: input.UsageGuide, State: cliconnector.StateDraft}
	if value.Icon == "" {
		value.Icon = "terminal"
	}
	if installationType == "upload" {
		if err := cliconnector.ValidateZIPPackage(input.Archive); err != nil {
			return cliconnector.Definition{}, fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err)
		}
		digestValue := sha256.Sum256(input.Archive)
		digest := hex.EncodeToString(digestValue[:])
		key := fmt.Sprintf("cli-connectors/sources/%s/v%d/%s.zip", definitionID, sourceVersion, digest)
		store, err := cliconnector.NewArtifactStore(service.objects)
		if err != nil {
			return cliconnector.Definition{}, err
		}
		if err := store.PutSource(ctx, key, input.Archive, digest); err != nil {
			return cliconnector.Definition{}, err
		}
		value.Package, value.Version = "upload-"+strings.ReplaceAll(definitionID, "-", ""), "0.0.0"
		value.Integrity, value.Executable, value.AuthenticationDriver = "", "", ""
		for index := range value.Capabilities {
			value.Capabilities[index] = cliconnector.Capability{ID: value.Capabilities[index].ID, DisplayName: maps.Clone(value.Capabilities[index].DisplayName), OperationPhrase: maps.Clone(value.Capabilities[index].OperationPhrase)}
		}
		value.SupportedArchitectures = nil
		value.SourceObjectKey, value.SourceSHA256 = key, digest
	}
	if err := value.ValidateDraft(); err != nil {
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
		fields := make([]*workspacev1.CLIInputField, 0, len(value.Input.Fields))
		for _, field := range value.Input.Fields {
			converted := &workspacev1.CLIInputField{Name: field.Name, Type: string(field.Type), Required: field.Required, Sensitive: field.Sensitive, Enum: field.Enum}
			if field.Flag != "" {
				converted.Flag = &field.Flag
			}
			fields = append(fields, converted)
		}
		capabilities = append(capabilities, &workspacev1.CLICapability{Id: value.ID, DisplayName: maps.Clone(value.DisplayName), OperationPhrase: maps.Clone(value.OperationPhrase), ArgvPrefix: value.ArgvPrefix, InputFields: fields, Risk: string(value.Risk), Identities: identities, Scopes: value.Scopes, EgressHosts: value.EgressHosts, TimeoutSeconds: int32(value.Timeout / time.Second), Idempotency: string(value.Idempotency)})
	}
	recommendedSkills := make([]*workspacev1.CLIRecommendedSkill, 0, len(item.RecommendedSkills))
	for _, skill := range item.RecommendedSkills {
		recommendedSkills = append(recommendedSkills, &workspacev1.CLIRecommendedSkill{Name: skill.Name, GitUrl: skill.GitURL, GitRef: skill.GitRef})
	}
	response := &workspacev1.CLIConnectorDefinition{Id: item.ID, Name: item.Name, Icon: item.Icon, Description: item.Description, InstallationType: item.InstallationType, NpmPackage: item.Package, NpmVersion: item.Version, NpmIntegrity: item.Integrity, Executable: item.Executable, AuthenticationDriver: item.AuthenticationDriver, Capabilities: capabilities, State: string(item.State), Mutable: mutable, Version: item.VersionNumber, SupportedArchitectures: item.SupportedArchitectures, ConformanceRuntimeDigests: item.RuntimeDigests, RecommendedSkills: recommendedSkills, ManifestVersion: item.ManifestVersion, UsageGuide: item.UsageGuide}
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
	response := &workspacev1.CommandApproval{Id: item.ID, ExecutionKind: item.ExecutionKind, ExecutionId: item.ExecutionID, ConnectorName: item.ConnectorName, Operation: item.Operation, Target: item.Target, RedactedArguments: item.RedactedArguments, State: string(item.State), ExpiresAt: timestamppb.New(item.ExpiresAt), Version: item.Version, OperationId: item.OperationID, ManifestVersion: item.ManifestVersion, InputDigest: item.InputDigest, AuthorizationId: item.AuthorizationID, ExternalIdentityId: item.ExternalIdentityID, ExternalDisplayName: item.ExternalDisplayName}
	if item.Identity != "" {
		value := string(item.Identity)
		response.Identity = &value
	}
	return response
}

func connectorActionResponse(item cliconnector.ActionRequirement) *workspacev1.ConnectorActionRequirement {
	actions := make([]string, 0, len(item.Actions))
	for _, action := range item.Actions {
		actions = append(actions, string(action))
	}
	response := &workspacev1.ConnectorActionRequirement{
		ContractVersion: int32(item.ContractVersion), Id: item.ID, ExecutionKind: item.ExecutionKind, ExecutionId: item.ExecutionID,
		OperationId: item.OperationID, ConnectorId: item.ConnectorID, ConnectorName: item.ConnectorName,
		EnablementId: item.EnablementID, CapabilityId: item.CapabilityID, OperationPhrase: maps.Clone(item.OperationPhrase),
		Identity: string(item.Identity),
		Reason:   string(item.Reason), Permissions: append([]string(nil), item.Permissions...), Actions: actions,
		State: string(item.State), ExpiresAt: timestamppb.New(item.ExpiresAt), Version: item.Version,
	}
	if item.ActionURL != "" {
		response.ActionUrl = &item.ActionURL
	}
	return response
}

func connectorActionResponseWithPlatformURL(item cliconnector.ActionRequirement) *workspacev1.ConnectorActionRequirement {
	if item.ActionURL != "" && item.ActionURLToken != "" && item.ActionURLOpenedAt == nil {
		query := url.Values{"version": []string{strconv.FormatInt(item.Version, 10)}, "token": []string{item.ActionURLToken}}
		item.ActionURL = "/api/v1/connector-actions/" + url.PathEscape(item.ID) + "/open?" + query.Encode()
	} else {
		item.ActionURL = ""
	}
	return connectorActionResponse(item)
}
