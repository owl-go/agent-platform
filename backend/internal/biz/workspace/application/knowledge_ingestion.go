package application

import "context"

// KnowledgeIngestionJob is the worker-facing projection of a durable source
// revision. It contains only logical object identity; provider credentials and
// index identifiers remain inside a future retrieval provider.
type KnowledgeIngestionJob struct {
	ID              string
	RevisionID      string
	DocumentID      string
	KnowledgeBaseID string
	ObjectKey       string
	ContentType     string
	Attempts        int
}

type KnowledgeIngestionRepository interface {
	ClaimKnowledgeIngestionJob(context.Context) (*KnowledgeIngestionJob, error)
	SupersededKnowledgeRevisions(context.Context, string, string) ([]string, error)
	FinishKnowledgeIngestionJob(context.Context, KnowledgeIngestionJob, error) error
}

// LegacyKnowledgeDocument is a text document created before the platform
// switched Assistant knowledge to the common source-revision pipeline.
type LegacyKnowledgeDocument struct {
	JobID           string
	DocumentID      string
	KnowledgeBaseID string
	OwnerID         string
	Name            string
	Content         string
}

type LegacyKnowledgeRepository interface {
	ClaimLegacyKnowledgeDocument(context.Context) (*LegacyKnowledgeDocument, error)
	CompleteLegacyKnowledgeDocument(context.Context, LegacyKnowledgeDocument, KnowledgeSourceSnapshot) error
	FailLegacyKnowledgeDocument(context.Context, LegacyKnowledgeDocument, error) error
}

type KnowledgeSourceSnapshot struct {
	ObjectKey   string
	SHA256      string
	Size        int64
	ContentType string
}
