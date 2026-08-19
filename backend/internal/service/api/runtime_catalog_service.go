package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	runtimecatalogv1 "agent-platform/backend/api/runtimecatalog/v1"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
)

func (service *GeneratedServices) ListRuntimeImages(ctx context.Context, request *runtimecatalogv1.ListRuntimeImagesRequest) (*runtimecatalogv1.ListRuntimeImagesResponse, error) {
	actor, err := service.dependencies.RuntimeAccess.AuthorizeRuntimeImageRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "runtime_catalog_access_denied")
	}
	page, err := service.dependencies.RuntimeImages.List(ctx, runtimeapplication.ListQuery{OrganizationID: actor.OrganizationID, PageSize: int(request.PageSize), Token: request.PageToken})
	if errors.Is(err, runtimeapplication.ErrInvalidPage) {
		return nil, publicError(400, "invalid_runtime_image_page")
	}
	if err != nil {
		return nil, publicError(500, "runtime_catalog_query_failed")
	}
	items := make([]runtimeImageResponse, 0, len(page.Items))
	for _, image := range page.Items {
		items = append(items, newRuntimeImageResponse(image))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items, "next_page_token": page.NextToken}, &runtimecatalogv1.ListRuntimeImagesResponse{})
}

func (service *GeneratedServices) GetRuntimeImage(ctx context.Context, request *runtimecatalogv1.GetRuntimeImageRequest) (*runtimecatalogv1.RuntimeImage, error) {
	if _, err := uuid.Parse(request.RuntimeImageId); err != nil {
		return nil, publicError(400, "invalid_runtime_image_id")
	}
	actor, err := service.dependencies.RuntimeAccess.AuthorizeRuntimeImageRead(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "runtime_catalog_access_denied")
	}
	image, err := service.dependencies.RuntimeImages.Get(ctx, actor.OrganizationID, request.RuntimeImageId)
	if errors.Is(err, runtimedomain.ErrRuntimeImageNotFound) {
		return nil, publicError(404, "runtime_image_not_found")
	}
	if err != nil {
		return nil, publicError(500, "runtime_catalog_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newRuntimeImageResponse(image), &runtimecatalogv1.RuntimeImage{})
}

func (service *GeneratedServices) RegisterRuntimeImage(ctx context.Context, request *runtimecatalogv1.RegisterRuntimeImageRequest) (*runtimecatalogv1.RuntimeImage, error) {
	actor, err := service.dependencies.CatalogWriteAccess.AuthorizeModelCatalogWrite(ctx, "")
	if err != nil {
		return nil, mapAuthorizationError(err, "catalog_write_access_denied")
	}
	result, err := service.executeWrite(ctx, actor, "runtime-image.register", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		image, err := services.RuntimeImages.Register(ctx, runtimeapplication.RegisterCommand{
			OrganizationID: actor.OrganizationID, Runtime: runtimedomain.Runtime(request.Runtime), CLIVersion: request.CliVersion,
			AdapterVersion: request.AdapterVersion, ImageDigest: request.ImageDigest,
			Capabilities: request.Capabilities,
		})
		return encodeWriteResult(http.StatusCreated, newRuntimeImageResponse(image), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &runtimecatalogv1.RuntimeImage{})
}

func (service *GeneratedServices) ChangeRuntimeImageStatus(ctx context.Context, request *runtimecatalogv1.ChangeRuntimeImageStatusRequest) (*runtimecatalogv1.RuntimeImage, error) {
	if _, err := uuid.Parse(request.RuntimeImageId); err != nil {
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
	result, err := service.executeWrite(ctx, actor, "runtime-image.status:"+request.RuntimeImageId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		image, err := services.RuntimeImages.ChangeStatus(ctx, runtimeapplication.ChangeStatusCommand{
			OrganizationID: actor.OrganizationID, ID: request.RuntimeImageId, ExpectedVersion: version, Status: runtimedomain.Status(request.Status),
			BlockedReason: request.BlockedReason, ConformanceEvidenceKey: request.ConformanceEvidenceKey,
		})
		return encodeWriteResult(http.StatusOK, newRuntimeImageResponse(image), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &runtimecatalogv1.RuntimeImage{})
}
