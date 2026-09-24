package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"github.com/google/uuid"
)

func assistantKnowledgeBaseFromWorkspace(base workspacedomain.KnowledgeBase) aiappdomain.KnowledgeBase {
	state := aiappdomain.KnowledgeReady
	if base.DeletedAt != nil {
		state = aiappdomain.KnowledgeDisabled
	}
	return aiappdomain.KnowledgeBase{ID: base.ID, OwnerID: base.OwnerID, Name: base.Name, Description: base.Description, State: state, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt, Version: base.Version}
}

func assistantKnowledgeDocumentFromWorkspace(document workspacedomain.KnowledgeDocument) aiappdomain.KnowledgeDocument {
	state := document.State
	failure := document.Error
	sha := ""
	if document.LatestRevision != nil {
		state = document.LatestRevision.State
		failure = document.LatestRevision.Error
		sha = document.LatestRevision.SHA256
	}
	appState := aiappdomain.KnowledgeProcessing
	switch state {
	case workspacedomain.KnowledgeReady:
		appState = aiappdomain.KnowledgeReady
	case workspacedomain.KnowledgeFailed, workspacedomain.KnowledgeBlocked:
		appState = aiappdomain.KnowledgeFailed
	}
	return aiappdomain.KnowledgeDocument{ID: document.ID, KnowledgeBaseID: document.KnowledgeBaseID, Name: document.Name, ContentSHA256: sha, State: appState, FailureReason: failure, CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt, Version: document.Version}
}

func (service *Service) createAssistantKnowledgeDocument(ctx context.Context, owner string, administrator bool, baseID string, input knowledgeDocumentPayload) (aiappdomain.KnowledgeDocument, error) {
	document := aiappdomain.KnowledgeDocument{Name: input.Name, Content: input.Content}
	if err := document.Validate(); err != nil {
		return aiappdomain.KnowledgeDocument{}, err
	}
	if aiappdomain.DefaultSafetyPolicy().Decide(input.Content) == aiappdomain.SafetyRefuse {
		return aiappdomain.KnowledgeDocument{}, fmt.Errorf("%w: document contains a protected topic", aiappdomain.ErrInvalid)
	}
	content := []byte(input.Content)
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	objectKey := fmt.Sprintf("knowledge/%s/%s/%s", owner, baseID, uuid.NewString())
	stored, err := service.objects.Put(ctx, objectKey, strings.NewReader(input.Content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "text/plain", Metadata: map[string]string{"name": input.Name}})
	if err != nil {
		return aiappdomain.KnowledgeDocument{}, fmt.Errorf("store Knowledge source: %w", err)
	}
	created, err := service.workspace.Repository().CreateKnowledgeDocument(ctx, owner, administrator, workspacedomain.KnowledgeDocumentInput{KnowledgeBaseID: baseID, Name: input.Name, SourceType: workspacedomain.KnowledgeUpload, NormalizedSource: sha, ObjectKey: stored.Key, SHA256: stored.SHA256, Size: stored.Size, ContentType: stored.ContentType})
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return aiappdomain.KnowledgeDocument{}, err
	}
	return assistantKnowledgeDocumentFromWorkspace(created), nil
}
