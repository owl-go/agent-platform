// Package anythingllm defines the platform-owned boundary to the AnythingLLM
// deployment. Runtime containers never receive this adapter or its credentials.
package anythingllm

import "context"

type Citation struct {
	KnowledgeBaseID string
	CategoryID      *string
	DocumentID      string
	RevisionID      string
	SourceLocation  string
	Relevance       float32
	Text            string
	SHA256          string
}

type Retrieval struct {
	Citations []Citation
	NoHit     bool
}

// Adapter is intentionally narrow: provider IDs and credentials stay inside
// the trusted API/Worker process and are never part of a Workflow snapshot.
type Adapter interface {
	EnsureWorkspace(context.Context, string) error
	DeleteWorkspace(context.Context, string) error
	UpsertRevision(context.Context, string, string, string, []byte) error
	RemoveRevision(context.Context, string, string) error
	Query(context.Context, string, int64, string, int, int) (Retrieval, error)
}
