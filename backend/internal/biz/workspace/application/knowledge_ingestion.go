package application

import "context"

// KnowledgeIngestionJob is the worker-facing projection of a durable source
// revision. It contains only logical object identity; provider credentials and
// workspace IDs remain inside the AnythingLLM adapter.
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
	FinishKnowledgeIngestionJob(context.Context, KnowledgeIngestionJob, error) error
}
