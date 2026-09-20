package application

import (
	"context"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

// EmbeddingProvider and RetrievalProvider deliberately keep model/indexing
// ownership outside the Assistant aggregate so a local PostgreSQL adapter can
// later be replaced by Ragflow without changing Assistant behavior.
type EmbeddingProvider interface {
	Embed(context.Context, []string) ([][]float32, error)
}

type RetrievedChunk struct {
	DocumentID string
	RevisionID string
	ChunkID    string
	Text       string
	Score      float32
}

type RetrievalProvider interface {
	Index(context.Context, string, []RetrievedChunk) error
	Search(context.Context, string, []string, int) ([]RetrievedChunk, error)
}

// VectorKnowledgeRepository is optional so the default PostgreSQL FTS path
// remains usable when pgvector is not installed in a local database.
type VectorKnowledgeRepository interface {
	SearchKnowledgeVector(context.Context, string, []string, []float32, int) ([]domain.KnowledgeChunk, error)
}

type EmbeddingConfigurationRepository interface {
	GetEmbeddingConfiguration(context.Context) (domain.EmbeddingConfiguration, error)
	SaveEmbeddingConfiguration(context.Context, domain.EmbeddingConfiguration, []byte) (domain.EmbeddingConfiguration, error)
}

type ExternalConversationRepository interface {
	CreateExternalConversation(context.Context, domain.ExternalConversation) (domain.ExternalConversation, error)
	GetExternalConversation(context.Context, string, string, string) (domain.ExternalConversation, error)
	CreateExternalResponse(context.Context, domain.ExternalResponse) (domain.ExternalResponse, error)
	GetExternalResponse(context.Context, string, string, string) (domain.ExternalResponse, error)
}

type ExternalRateLimitRepository interface {
	ConsumeExternalRate(context.Context, string, string, time.Time, int) (bool, error)
}
