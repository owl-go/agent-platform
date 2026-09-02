package workspace

import (
	"context"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/workspacefs"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListSessions(ctx context.Context, request *workspacev1.ListSessionsRequest) (*workspacev1.ListSessionsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListSessions(ctx, owner, request.Archived)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Session, 0, len(items))
	for _, item := range items {
		response = append(response, sessionResponse(item))
	}
	return &workspacev1.ListSessionsResponse{Items: response}, nil
}

func (service *Service) CreateSession(ctx context.Context, request *workspacev1.CreateSessionRequest) (*workspacev1.Session, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().CreateSession(ctx, owner, request.ExpertId)
	if err != nil {
		return nil, publicError(err)
	}
	return sessionResponse(item), nil
}

func (service *Service) GetSession(ctx context.Context, request *workspacev1.GetSessionRequest) (*workspacev1.Session, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetSession(ctx, owner, request.SessionId)
	if err != nil {
		return nil, publicError(err)
	}
	return sessionResponse(item), nil
}

func (service *Service) UpdateSession(ctx context.Context, request *workspacev1.UpdateSessionRequest) (*workspacev1.Session, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().UpdateSession(ctx, owner, request.SessionId, request.Title, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return sessionResponse(item), nil
}

func (service *Service) SetSessionArchived(ctx context.Context, request *workspacev1.SetSessionArchivedRequest) (*workspacev1.Session, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().SetSessionArchived(ctx, owner, request.SessionId, request.Archived, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return sessionResponse(item), nil
}

func (service *Service) DeleteSession(ctx context.Context, request *workspacev1.DeleteSessionRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteSession(ctx, owner, request.SessionId); err != nil {
		return nil, publicError(err)
	}
	if err := workspacefs.RemoveNativeSessionState(service.config.Workspace.Root, owner, request.SessionId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ListSessionMessages(ctx context.Context, request *workspacev1.ListSessionMessagesRequest) (*workspacev1.ListSessionMessagesResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	after, limit := int64(0), 100
	if request.After != nil {
		after = *request.After
	}
	if request.Limit != nil {
		limit = int(*request.Limit)
	}
	items, err := service.workspace.Repository().ListMessages(ctx, owner, request.SessionId, after, limit)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.SessionMessage, 0, len(items))
	for _, item := range items {
		response = append(response, messageResponse(item))
	}
	return &workspacev1.ListSessionMessagesResponse{Items: response}, nil
}

func (service *Service) SendSessionMessage(ctx context.Context, request *workspacev1.SendSessionMessageRequest) (*workspacev1.SendSessionMessageResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	attachments, err := service.resolveAttachments(ctx, owner, request.AttachmentIds)
	if err != nil {
		return nil, publicError(err)
	}
	user, assistant, err := service.workspace.Repository().CreateMessagePair(ctx, owner, request.SessionId, request.Content, request.ProviderModelId, attachments)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.SendSessionMessageResponse{UserMessage: messageResponse(user), AssistantMessage: messageResponse(assistant)}, nil
}

func (service *Service) RetrySessionMessage(ctx context.Context, request *workspacev1.RetrySessionMessageRequest) (*workspacev1.SendSessionMessageResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	user, assistant, err := service.workspace.Repository().RetryMessage(ctx, owner, request.SessionId, request.MessageId)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.SendSessionMessageResponse{UserMessage: messageResponse(user), AssistantMessage: messageResponse(assistant)}, nil
}

func (service *Service) CancelSessionMessage(ctx context.Context, request *workspacev1.CancelSessionMessageRequest) (*workspacev1.SessionMessage, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	message, err := service.workspace.Repository().CancelMessage(ctx, owner, request.SessionId, request.MessageId)
	if err != nil {
		return nil, publicError(err)
	}
	return messageResponse(message), nil
}

func sessionResponse(item workspacedomain.Session) *workspacev1.Session {
	response := &workspacev1.Session{Id: item.ID, Title: item.Title, ExpertId: item.ExpertID, CurrentProviderModelId: item.CurrentProviderModelID, Archived: item.ArchivedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
	return response
}

func messageResponse(item workspacedomain.Message) *workspacev1.SessionMessage {
	response := &workspacev1.SessionMessage{Id: item.ID, Role: item.Role, State: item.State, Content: item.Content, ElapsedMs: item.ElapsedMS, CreatedAt: timestamppb.New(item.CreatedAt), ProgressStage: item.ProgressStage}
	if item.Error != "" {
		response.Error = &item.Error
	}
	if item.ResponseSnapshot != nil {
		snapshot := item.ResponseSnapshot
		response.ResponseSnapshot = &workspacev1.ResponseSnapshot{ProviderModelId: snapshot.ProviderModelID, ConnectionId: snapshot.ConnectionID, ConnectionName: snapshot.ConnectionName, ProviderType: snapshot.ProviderType, ModelId: snapshot.ModelID, ModelName: snapshot.ModelName, Endpoint: snapshot.Endpoint, Protocols: snapshot.Protocols, RuntimeEngine: string(snapshot.RuntimeEngine), Compatibility: snapshot.Compatibility, ConnectionVersion: snapshot.ConnectionVersion}
	}
	for _, attachment := range item.Attachments {
		response.Attachments = append(response.Attachments, attachmentResponse(attachment))
	}
	return response
}

func attachmentResponse(item workspacedomain.Attachment) *workspacev1.Attachment {
	return &workspacev1.Attachment{Id: item.ID, Name: item.Name, ContentType: item.ContentType, Size: item.Size, Sha256: item.SHA256, Image: item.Image}
}
