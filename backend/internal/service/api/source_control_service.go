package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	sourcecontrolv1 "agent-platform/backend/api/sourcecontrol/v1"
	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
)

func (service *GeneratedServices) ListSourceControlProviders(ctx context.Context, _ *sourcecontrolv1.ListSourceControlProvidersRequest) (*sourcecontrolv1.ListSourceControlProvidersResponse, error) {
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	values, err := service.dependencies.SourceControl.List(ctx, actor.OrganizationID)
	if err != nil {
		return nil, publicError(500, "source_control_catalog_query_failed")
	}
	items := make([]sourceControlProviderResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newSourceControlProviderResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &sourcecontrolv1.ListSourceControlProvidersResponse{})
}

func (service *GeneratedServices) GetSourceControlProvider(ctx context.Context, request *sourcecontrolv1.GetSourceControlProviderRequest) (*sourcecontrolv1.SourceControlProvider, error) {
	if _, err := uuid.Parse(request.SourceControlProviderId); err != nil {
		return nil, publicError(400, "invalid_source_control_provider_id")
	}
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	value, err := service.dependencies.SourceControl.Get(ctx, actor.OrganizationID, request.SourceControlProviderId)
	if errors.Is(err, sourcedomain.ErrProviderNotFound) {
		return nil, publicError(404, "source_control_provider_not_found")
	}
	if err != nil {
		return nil, publicError(500, "source_control_catalog_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newSourceControlProviderResponse(value), &sourcecontrolv1.SourceControlProvider{})
}

func (service *GeneratedServices) RegisterSourceControlProvider(ctx context.Context, request *sourcecontrolv1.RegisterSourceControlProviderRequest) (*sourcecontrolv1.SourceControlProvider, error) {
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	result, err := service.executeWrite(ctx, actor, "source-control-provider.register", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.SourceControl.Register(ctx, sourceapplication.RegisterCommand{OrganizationID: actor.OrganizationID, Name: request.Name, Kind: sourcedomain.Kind(request.Kind), BaseURL: request.BaseUrl})
		return encodeWriteResult(http.StatusCreated, newSourceControlProviderResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &sourcecontrolv1.SourceControlProvider{})
}

func (service *GeneratedServices) ChangeSourceControlProviderStatus(ctx context.Context, request *sourcecontrolv1.ChangeSourceControlProviderStatusRequest) (*sourcecontrolv1.SourceControlProvider, error) {
	if _, err := uuid.Parse(request.SourceControlProviderId); err != nil {
		return nil, publicError(400, "invalid_resource_id")
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	result, err := service.executeWrite(ctx, actor, "source-control-provider.status:"+request.SourceControlProviderId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.SourceControl.ChangeStatus(ctx, sourceapplication.ChangeStatusCommand{OrganizationID: actor.OrganizationID, ID: request.SourceControlProviderId, ExpectedVersion: version, Enabled: request.Enabled})
		return encodeWriteResult(http.StatusOK, newSourceControlProviderResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &sourcecontrolv1.SourceControlProvider{})
}

func (service *GeneratedServices) ListRepositoryBindings(ctx context.Context, request *sourcecontrolv1.ListRepositoryBindingsRequest) (*sourcecontrolv1.ListRepositoryBindingsResponse, error) {
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.RepositoryBindings.List(ctx, actor.OrganizationID, actor.TeamID)
	if err != nil {
		return nil, publicError(500, "repository_binding_query_failed")
	}
	items := make([]repositoryBindingResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newRepositoryBindingResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &sourcecontrolv1.ListRepositoryBindingsResponse{})
}

func (service *GeneratedServices) GetRepositoryBinding(ctx context.Context, request *sourcecontrolv1.GetRepositoryBindingRequest) (*sourcecontrolv1.RepositoryBinding, error) {
	if _, err := uuid.Parse(request.RepositoryBindingId); err != nil {
		return nil, publicError(400, "invalid_repository_binding_id")
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.RepositoryBindings.Get(ctx, actor.OrganizationID, actor.TeamID, request.RepositoryBindingId)
	if errors.Is(err, sourcedomain.ErrBindingNotFound) {
		return nil, publicError(404, "repository_binding_not_found")
	}
	if err != nil {
		return nil, publicError(500, "repository_binding_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newRepositoryBindingResponse(value), &sourcecontrolv1.RepositoryBinding{})
}

func (service *GeneratedServices) RegisterRepositoryBinding(ctx context.Context, request *sourcecontrolv1.RegisterRepositoryBindingRequest) (*sourcecontrolv1.RepositoryBinding, error) {
	if request.Binding == nil {
		return nil, publicError(400, "invalid_request_body")
	}
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	actor.TeamID = request.Binding.TeamId
	command := bindingCommand(request.Binding, actor.OrganizationID, actor.TeamID)
	result, err := service.executeWrite(ctx, actor, "repository-binding.register", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Bindings.Register(ctx, command)
		return encodeWriteResult(http.StatusCreated, newRepositoryBindingResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &sourcecontrolv1.RepositoryBinding{})
}

func (service *GeneratedServices) UpdateRepositoryBinding(ctx context.Context, request *sourcecontrolv1.UpdateRepositoryBindingRequest) (*sourcecontrolv1.RepositoryBinding, error) {
	if _, err := uuid.Parse(request.RepositoryBindingId); err != nil || request.Binding == nil {
		return nil, publicError(400, "invalid_resource_id")
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	actor.TeamID = request.Binding.TeamId
	command := sourceapplication.UpdateBindingCommand{ID: request.RepositoryBindingId, ExpectedVersion: version, RegisterBindingCommand: bindingCommand(request.Binding, actor.OrganizationID, actor.TeamID)}
	result, err := service.executeWrite(ctx, actor, "repository-binding.update:"+request.RepositoryBindingId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Bindings.Update(ctx, command)
		return encodeWriteResult(http.StatusOK, newRepositoryBindingResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &sourcecontrolv1.RepositoryBinding{})
}

func (service *GeneratedServices) ValidateRepositoryBinding(ctx context.Context, request *sourcecontrolv1.ValidateRepositoryBindingRequest) (*sourcecontrolv1.RepositoryBinding, error) {
	if _, err := uuid.Parse(request.RepositoryBindingId); err != nil {
		return nil, publicError(400, "invalid_resource_id")
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	actor.TeamID = request.TeamId
	result, err := service.executeWrite(ctx, actor, "repository-binding.validate:"+request.RepositoryBindingId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Bindings.Validate(ctx, actor.OrganizationID, actor.TeamID, request.RepositoryBindingId, version)
		return encodeWriteResult(http.StatusOK, newRepositoryBindingResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &sourcecontrolv1.RepositoryBinding{})
}

func bindingCommand(input *sourcecontrolv1.RepositoryBindingInput, organizationID, teamID string) sourceapplication.RegisterBindingCommand {
	command := sourceapplication.RegisterBindingCommand{OrganizationID: organizationID, TeamID: teamID, SourceControlProviderID: input.SourceControlProviderId, Name: input.Name, RepositorySSHURL: input.RepositorySshUrl, DefaultBranch: input.DefaultBranch, SSHCredentialProfileID: input.SshCredentialProfileId, BuildCredentialProfileIDs: input.BuildCredentialProfileIds, GitAuthorName: input.GitAuthorName, GitAuthorEmail: input.GitAuthorEmail, AllowedRuntimeImageIDs: input.AllowedRuntimeImageIds, DefaultRuntimeImageID: input.DefaultRuntimeImageId, RequiredRuntimeCapabilities: input.RequiredRuntimeCapabilities, DefaultModelID: input.DefaultModelId, Instructions: input.Instructions}
	if input.ModelBudget != nil {
		command.ModelBudget = sourcedomain.ModelBudget{MaxInputTokens: input.ModelBudget.MaxInputTokens, MaxOutputTokens: input.ModelBudget.MaxOutputTokens, MaxCostAmount: input.ModelBudget.MaxCostAmount}
	}
	for _, quality := range input.QualityCommands {
		command.QualityCommands = append(command.QualityCommands, sourcedomain.QualityCommand{Name: quality.Name, Kind: sourcedomain.QualityCommandKind(quality.Kind), Executable: quality.Executable, Arguments: quality.Arguments, TimeoutSeconds: int(quality.TimeoutSeconds)})
	}
	if input.EgressPolicy != nil {
		command.EgressPolicy = sourcedomain.EgressPolicy{Mode: input.EgressPolicy.Mode}
	}
	return command
}
