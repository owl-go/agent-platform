package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

type KnowledgeRepository interface {
	ListKnowledgeBases(context.Context, string) ([]domain.KnowledgeBase, error)
	GetKnowledgeBase(context.Context, string, string) (domain.KnowledgeBase, error)
	CreateKnowledgeBase(context.Context, string, domain.KnowledgeBase) (domain.KnowledgeBase, error)
	ListKnowledgeDocuments(context.Context, string, string) ([]domain.KnowledgeDocument, error)
	CreateKnowledgeDocument(context.Context, string, string, domain.KnowledgeDocument, []domain.KnowledgeChunk) (domain.KnowledgeDocument, error)
	SearchKnowledge(context.Context, string, []string, string, int) ([]domain.KnowledgeChunk, error)
}

func (service *Service) SetKnowledgeRepository(repository KnowledgeRepository) {
	service.knowledge = repository
}

func (service *Service) ListKnowledgeBases(ctx context.Context, owner string) ([]domain.KnowledgeBase, error) {
	if service.knowledge == nil {
		return nil, fmt.Errorf("Knowledge Base repository is unavailable")
	}
	return service.knowledge.ListKnowledgeBases(ctx, owner)
}
func (service *Service) CreateKnowledgeBase(ctx context.Context, owner string, base domain.KnowledgeBase) (domain.KnowledgeBase, error) {
	if service.knowledge == nil {
		return domain.KnowledgeBase{}, fmt.Errorf("Knowledge Base repository is unavailable")
	}
	if base.State == "" {
		base.State = domain.KnowledgeReady
	}
	if err := base.Validate(); err != nil {
		return domain.KnowledgeBase{}, err
	}
	return service.knowledge.CreateKnowledgeBase(ctx, owner, base)
}
func (service *Service) ListKnowledgeDocuments(ctx context.Context, owner, baseID string) ([]domain.KnowledgeDocument, error) {
	if service.knowledge == nil {
		return nil, fmt.Errorf("Knowledge Base repository is unavailable")
	}
	return service.knowledge.ListKnowledgeDocuments(ctx, owner, baseID)
}
func (service *Service) CreateKnowledgeDocument(ctx context.Context, owner, baseID string, document domain.KnowledgeDocument) (domain.KnowledgeDocument, error) {
	if service.knowledge == nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("Knowledge Base repository is unavailable")
	}
	if err := document.Validate(); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	if domain.DefaultSafetyPolicy().Decide(document.Content) == domain.SafetyRefuse {
		return domain.KnowledgeDocument{}, fmt.Errorf("%w: document contains a protected topic", domain.ErrInvalid)
	}
	digest := sha256.Sum256([]byte(document.Content))
	document.ContentSHA256 = hex.EncodeToString(digest[:])
	document.State = domain.KnowledgeReady
	document.UpdatedAt = time.Now().UTC()
	chunks := chunkDocument(document.Content)
	return service.knowledge.CreateKnowledgeDocument(ctx, owner, baseID, document, chunks)
}
func (service *Service) SearchKnowledge(ctx context.Context, owner string, baseIDs []string, query string, limit int) ([]domain.KnowledgeChunk, error) {
	if service.knowledge == nil {
		return nil, fmt.Errorf("Knowledge Base repository is unavailable")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	return service.knowledge.SearchKnowledge(ctx, owner, baseIDs, query, limit)
}

func chunkDocument(content string) []domain.KnowledgeChunk {
	runes := []rune(content)
	const size = 1800
	chunks := make([]domain.KnowledgeChunk, 0, (len(runes)+size-1)/size)
	for position, start := 0, 0; start < len(runes); position, start = position+1, start+size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, domain.KnowledgeChunk{Position: position, Text: string(runes[start:end])})
	}
	return chunks
}
