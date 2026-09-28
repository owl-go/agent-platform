package retrieval

import (
	"context"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

type searchRepository struct {
	visible    bool
	generation int64
	ready      map[string]domain.KnowledgeSearchSource
}

func (repo *searchRepository) ReadyKnowledgeSearchGeneration(context.Context, string, string, bool) (int64, error) {
	if !repo.visible {
		return 0, domain.ErrNotFound
	}
	return repo.generation, nil
}

func (repo *searchRepository) ResolveKnowledgeSearchSource(_ context.Context, _, _, revisionID string, _ bool) (domain.KnowledgeSearchSource, error) {
	if value, ok := repo.ready[revisionID]; ok {
		return value, nil
	}
	return domain.KnowledgeSearchSource{}, domain.ErrNotFound
}

type searchProvider struct {
	citations []Citation
	err       error
	calls     int
	onQuery   func()
}

func (provider *searchProvider) Query(context.Context, string, int64, string, int, int) (Result, error) {
	provider.calls++
	if provider.onQuery != nil {
		provider.onQuery()
	}
	return Result{Citations: provider.citations}, provider.err
}

func TestSearchUsesOneVerifiedSourcePath(t *testing.T) {
	active, old := uuid.NewString(), uuid.NewString()
	repo := &searchRepository{visible: true, generation: 2, ready: map[string]domain.KnowledgeSearchSource{active: {DocumentID: "document", RevisionID: active, DocumentName: "guide.txt"}}}
	provider := &searchProvider{citations: []Citation{
		{RevisionID: old, Text: "outdated"},
		{RevisionID: "unattributed", Text: "unknown"},
		{RevisionID: active + ".txt", Text: "current", Relevance: .9},
		{RevisionID: active, Text: "current", Relevance: .9},
	}}
	engine, err := New(repo, provider)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := engine.Search(context.Background(), "owner", "base", 0, "question", 10, 6000)
	if err != nil || len(hits) != 1 || hits[0].Source.RevisionID != active || hits[0].Text != "current" {
		t.Fatalf("hits = %+v, err = %v", hits, err)
	}
	repo.visible = false
	if _, err := engine.Search(context.Background(), "other", "base", 0, "question", 10, 6000); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private Knowledge Base access = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider was called despite denied access: %d", provider.calls)
	}
	repo.visible = true
	if _, err := engine.Search(context.Background(), "owner", "base", 1, "question", 10, 6000); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("stale frozen generation = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider queried for a stale generation: %d", provider.calls)
	}
	repo.generation = 0
	if _, err := engine.Search(context.Background(), "owner", "base", 2, "question", 10, 6000); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("missing frozen generation = %v", err)
	}
	repo.generation = 2
	provider.onQuery = func() { repo.generation = 3 }
	if _, err := engine.Search(context.Background(), "owner", "base", 2, "question", 10, 6000); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("generation changed during query = %v", err)
	}
}
