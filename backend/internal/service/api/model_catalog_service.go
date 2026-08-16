package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	modelcatalogv1 "agent-platform/backend/api/modelcatalog/v1"
	modelapplication "agent-platform/backend/internal/biz/modelcatalog/application"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
)

func (service *GeneratedServices) ListCredentialProfiles(ctx context.Context, _ *modelcatalogv1.ListCredentialProfilesRequest) (*modelcatalogv1.ListCredentialProfilesResponse, error) {
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	values, err := service.dependencies.ModelCatalog.ListCredentials(ctx, actor.OrganizationID)
	if err != nil {
		return nil, publicError(500, "model_catalog_query_failed")
	}
	items := make([]credentialProfileResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newCredentialProfileResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &modelcatalogv1.ListCredentialProfilesResponse{})
}

func (service *GeneratedServices) GetCredentialProfile(ctx context.Context, request *modelcatalogv1.GetCredentialProfileRequest) (*modelcatalogv1.CredentialProfile, error) {
	if _, err := uuid.Parse(request.CredentialProfileId); err != nil {
		return nil, publicError(400, "invalid_credential_profile_id")
	}
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	value, err := service.dependencies.ModelCatalog.GetCredential(ctx, actor.OrganizationID, request.CredentialProfileId)
	if errors.Is(err, modeldomain.ErrCredentialProfileNotFound) {
		return nil, publicError(404, "credential_profile_not_found")
	}
	if err != nil {
		return nil, publicError(500, "model_catalog_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newCredentialProfileResponse(value), &modelcatalogv1.CredentialProfile{})
}

func (service *GeneratedServices) RegisterCredentialProfile(ctx context.Context, request *modelcatalogv1.RegisterCredentialProfileRequest) (*modelcatalogv1.CredentialProfile, error) {
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	result, err := service.executeWrite(ctx, actor, "credential-profile.register", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Models.RegisterCredential(ctx, modelapplication.RegisterCredentialCommand{
			OrganizationID: actor.OrganizationID, TeamID: request.TeamId, Name: request.Name,
			Kind: modeldomain.CredentialKind(request.Kind), SecretRef: request.SecretRef,
		})
		return encodeWriteResult(http.StatusCreated, newCredentialProfileResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &modelcatalogv1.CredentialProfile{})
}

func (service *GeneratedServices) ChangeCredentialProfileStatus(ctx context.Context, request *modelcatalogv1.ChangeCredentialProfileStatusRequest) (*modelcatalogv1.CredentialProfile, error) {
	if _, err := uuid.Parse(request.CredentialProfileId); err != nil {
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
	result, err := service.executeWrite(ctx, actor, "credential-profile.status:"+request.CredentialProfileId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Models.ChangeCredentialStatus(ctx, modelapplication.ChangeStatusCommand{
			OrganizationID: actor.OrganizationID, ID: request.CredentialProfileId, ExpectedVersion: version, Enabled: request.Enabled,
		})
		return encodeWriteResult(http.StatusOK, newCredentialProfileResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &modelcatalogv1.CredentialProfile{})
}

func (service *GeneratedServices) ListConfiguredModels(ctx context.Context, _ *modelcatalogv1.ListConfiguredModelsRequest) (*modelcatalogv1.ListConfiguredModelsResponse, error) {
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	values, err := service.dependencies.ModelCatalog.ListModels(ctx, actor.OrganizationID)
	if err != nil {
		return nil, publicError(500, "model_catalog_query_failed")
	}
	items := make([]configuredModelResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newConfiguredModelResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &modelcatalogv1.ListConfiguredModelsResponse{})
}

func (service *GeneratedServices) GetConfiguredModel(ctx context.Context, request *modelcatalogv1.GetConfiguredModelRequest) (*modelcatalogv1.ConfiguredModel, error) {
	if _, err := uuid.Parse(request.ConfiguredModelId); err != nil {
		return nil, publicError(400, "invalid_configured_model_id")
	}
	actor, err := service.dependencies.ModelAccess.AuthorizeModelCatalogRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "model_catalog_access_denied")
	}
	value, err := service.dependencies.ModelCatalog.GetModel(ctx, actor.OrganizationID, request.ConfiguredModelId)
	if errors.Is(err, modeldomain.ErrConfiguredModelNotFound) {
		return nil, publicError(404, "configured_model_not_found")
	}
	if err != nil {
		return nil, publicError(500, "model_catalog_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newConfiguredModelResponse(value), &modelcatalogv1.ConfiguredModel{})
}

func (service *GeneratedServices) RegisterConfiguredModel(ctx context.Context, request *modelcatalogv1.RegisterConfiguredModelRequest) (*modelcatalogv1.ConfiguredModel, error) {
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	result, err := service.executeWrite(ctx, actor, "configured-model.register", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Models.RegisterModel(ctx, modelapplication.RegisterModelCommand{
			OrganizationID: actor.OrganizationID, Name: request.Name, ModelID: request.ModelId,
			Endpoint: request.Endpoint, CredentialProfileID: request.CredentialProfileId,
		})
		return encodeWriteResult(http.StatusCreated, newConfiguredModelResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &modelcatalogv1.ConfiguredModel{})
}

func (service *GeneratedServices) ChangeConfiguredModelStatus(ctx context.Context, request *modelcatalogv1.ChangeConfiguredModelStatusRequest) (*modelcatalogv1.ConfiguredModel, error) {
	if _, err := uuid.Parse(request.ConfiguredModelId); err != nil {
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
	result, err := service.executeWrite(ctx, actor, "configured-model.status:"+request.ConfiguredModelId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Models.ChangeModelStatus(ctx, modelapplication.ChangeStatusCommand{
			OrganizationID: actor.OrganizationID, ID: request.ConfiguredModelId, ExpectedVersion: version, Enabled: request.Enabled,
		})
		return encodeWriteResult(http.StatusOK, newConfiguredModelResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &modelcatalogv1.ConfiguredModel{})
}
