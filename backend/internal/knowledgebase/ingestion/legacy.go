package ingestion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/objectstore"
)

// LegacyProcessor copies pre-unification Assistant text documents into the
// same durable Object Store + revision queue used by every other Knowledge
// Document. It never embeds or indexes on its own.
type LegacyProcessor struct {
	repository workspaceapplication.LegacyKnowledgeRepository
	objects    objectstore.Provider
}

func NewLegacy(repository workspaceapplication.LegacyKnowledgeRepository, objects objectstore.Provider) (*LegacyProcessor, error) {
	if repository == nil || objects == nil {
		return nil, fmt.Errorf("legacy Knowledge migration requires a repository and Object Store")
	}
	return &LegacyProcessor{repository: repository, objects: objects}, nil
}

func (processor *LegacyProcessor) ProcessNext(ctx context.Context) (bool, error) {
	document, err := processor.repository.ClaimLegacyKnowledgeDocument(ctx)
	if err != nil || document == nil {
		return false, err
	}
	content := []byte(document.Content)
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	key := strings.Join([]string{"knowledge", document.OwnerID, document.KnowledgeBaseID, document.DocumentID, "legacy.txt"}, "/")
	stored, err := processor.objects.Put(ctx, key, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "text/plain", Metadata: map[string]string{"name": document.Name}})
	if err == nil {
		err = processor.repository.CompleteLegacyKnowledgeDocument(ctx, *document, workspaceapplication.KnowledgeSourceSnapshot{ObjectKey: stored.Key, SHA256: stored.SHA256, Size: stored.Size, ContentType: stored.ContentType})
	}
	if err == nil {
		return true, nil
	}
	if finishErr := processor.repository.FailLegacyKnowledgeDocument(context.WithoutCancel(ctx), *document, err); finishErr != nil {
		return true, fmt.Errorf("migrate legacy Knowledge Document: %v; record failure: %w", err, finishErr)
	}
	return true, nil
}
