package application

import "context"

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
