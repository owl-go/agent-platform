package workspace

import (
	"context"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) knowledgePrincipal(ctx context.Context) (string, bool, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return "", false, publicError(err)
	}
	return principal.UserID, principal.Administrator, nil
}

func (service *Service) ListKnowledgeBases(ctx context.Context, request *workspacev1.ListKnowledgeBasesRequest) (*workspacev1.ListKnowledgeBasesResponse, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListKnowledgeBases(ctx, owner, administrator, request.GetDeleted())
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListKnowledgeBasesResponse{Items: make([]*workspacev1.KnowledgeBase, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, knowledgeBaseResponse(item))
	}
	return response, nil
}

func (service *Service) CreateKnowledgeBase(ctx context.Context, request *workspacev1.CreateKnowledgeBaseRequest) (*workspacev1.KnowledgeBase, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request == nil || request.KnowledgeBase == nil {
		return nil, publicError(workspacedomain.ErrInvalid)
	}
	input := knowledgeBaseInput(request.KnowledgeBase)
	item, err := service.workspace.Repository().CreateKnowledgeBase(ctx, owner, administrator, input)
	if err != nil {
		return nil, publicError(err)
	}
	return knowledgeBaseResponse(item), nil
}

func (service *Service) GetKnowledgeBase(ctx context.Context, request *workspacev1.GetKnowledgeBaseRequest) (*workspacev1.KnowledgeBase, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetKnowledgeBase(ctx, owner, request.GetKnowledgeBaseId(), administrator, false)
	if err != nil {
		return nil, publicError(err)
	}
	return knowledgeBaseResponse(item), nil
}

func (service *Service) UpdateKnowledgeBase(ctx context.Context, request *workspacev1.UpdateKnowledgeBaseRequest) (*workspacev1.KnowledgeBase, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request == nil || request.KnowledgeBase == nil {
		return nil, publicError(workspacedomain.ErrInvalid)
	}
	item, err := service.workspace.Repository().UpdateKnowledgeBase(ctx, owner, request.GetKnowledgeBaseId(), administrator, knowledgeBaseInput(request.KnowledgeBase), request.GetExpectedVersion())
	if err != nil {
		return nil, publicError(err)
	}
	return knowledgeBaseResponse(item), nil
}

func (service *Service) DeleteKnowledgeBase(ctx context.Context, request *workspacev1.DeleteKnowledgeBaseRequest) (*workspacev1.DeleteResponse, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteKnowledgeBase(ctx, owner, request.GetKnowledgeBaseId(), administrator); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ListKnowledgeCategories(ctx context.Context, request *workspacev1.ListKnowledgeCategoriesRequest) (*workspacev1.ListKnowledgeCategoriesResponse, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListKnowledgeCategories(ctx, owner, request.GetKnowledgeBaseId(), administrator)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListKnowledgeCategoriesResponse{Items: make([]*workspacev1.KnowledgeCategory, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, knowledgeCategoryResponse(item))
	}
	return response, nil
}

func (service *Service) CreateKnowledgeCategory(ctx context.Context, request *workspacev1.CreateKnowledgeCategoryRequest) (*workspacev1.KnowledgeCategory, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().CreateKnowledgeCategory(ctx, owner, request.GetKnowledgeBaseId(), administrator, request.GetName())
	if err != nil {
		return nil, publicError(err)
	}
	return knowledgeCategoryResponse(item), nil
}

func (service *Service) DeleteKnowledgeCategory(ctx context.Context, request *workspacev1.DeleteKnowledgeCategoryRequest) (*workspacev1.DeleteResponse, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteKnowledgeCategory(ctx, owner, request.GetKnowledgeBaseId(), request.GetCategoryId(), administrator); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ListKnowledgeDocuments(ctx context.Context, request *workspacev1.ListKnowledgeDocumentsRequest) (*workspacev1.ListKnowledgeDocumentsResponse, error) {
	owner, administrator, err := service.knowledgePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListKnowledgeDocuments(ctx, owner, request.GetKnowledgeBaseId(), administrator)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListKnowledgeDocumentsResponse{Items: make([]*workspacev1.KnowledgeDocument, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, knowledgeDocumentResponse(item))
	}
	return response, nil
}

func knowledgeBaseInput(input *workspacev1.KnowledgeBaseInput) workspacedomain.KnowledgeBaseInput {
	return workspacedomain.KnowledgeBaseInput{Name: input.GetName(), Description: input.GetDescription(), Visibility: workspacedomain.KnowledgeVisibility(input.GetVisibility()), Platform: input.GetPlatform()}
}

func knowledgeBaseResponse(item workspacedomain.KnowledgeBase) *workspacev1.KnowledgeBase {
	response := &workspacev1.KnowledgeBase{Id: item.ID, OwnerId: item.OwnerID, Name: item.Name, Description: item.Description, Visibility: string(item.Visibility), Platform: item.Platform, Deleted: item.DeletedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version, DocumentCount: item.DocumentCount, ReadyDocumentCount: item.ReadyDocumentCount}
	if item.LastReadyAt != nil {
		response.LastReadyAt = timestamppb.New(*item.LastReadyAt)
	}
	return response
}

func knowledgeCategoryResponse(item workspacedomain.KnowledgeCategory) *workspacev1.KnowledgeCategory {
	return &workspacev1.KnowledgeCategory{Id: item.ID, KnowledgeBaseId: item.KnowledgeBaseID, Name: item.Name, Deleted: item.DeletedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
}

func knowledgeDocumentResponse(item workspacedomain.KnowledgeDocument) *workspacev1.KnowledgeDocument {
	response := &workspacev1.KnowledgeDocument{Id: item.ID, KnowledgeBaseId: item.KnowledgeBaseID, CategoryId: item.CategoryID, Name: item.Name, SourceType: string(item.SourceType), SourceUri: item.SourceURI, State: string(item.State), Deleted: item.DeletedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
	if item.Error != "" {
		response.Error = &item.Error
	}
	if item.LatestRevision != nil {
		revision := item.LatestRevision
		response.LatestRevision = &workspacev1.KnowledgeDocumentRevision{Id: revision.ID, DocumentId: revision.DocumentID, Revision: int32(revision.Revision), ObjectKey: revision.ObjectKey, Sha256: revision.SHA256, Size: revision.Size, ContentType: revision.ContentType, State: string(revision.State), CreatedAt: timestamppb.New(revision.CreatedAt)}
		if revision.Error != "" {
			response.LatestRevision.Error = &revision.Error
		}
		if revision.ReadyAt != nil {
			response.LatestRevision.ReadyAt = timestamppb.New(*revision.ReadyAt)
		}
	}
	return response
}
