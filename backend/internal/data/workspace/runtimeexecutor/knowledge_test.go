package runtimeexecutor

import (
	"context"
	"strings"
	"testing"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/retrieval"
)

type retrievalFake struct{ calls int }

func (fake *retrievalFake) Search(_ context.Context, _, _ string, _ int64, _ string, _, _ int) ([]retrieval.Hit, error) {
	fake.calls++
	return []retrieval.Hit{{Text: "same evidence", Relevance: .9, Source: workspacedomain.KnowledgeSearchSource{DocumentName: "source.md", RevisionID: "revision"}}}, nil
}

func TestInjectKnowledgeContextBoundsAndDeduplicatesCitations(t *testing.T) {
	fake := &retrievalFake{}
	executor := &Executor{knowledge: fake}
	result, err := executor.injectKnowledgeContext(context.Background(), "owner", workspacedomain.ExecutionSnapshot{Goal: "answer", KnowledgeBaseIDs: []string{"base-1", "base-2"}, KnowledgeIndexGenerations: map[string]int64{"base-1": 1, "base-2": 1}}, "question")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || strings.Count(result, "same evidence") != 1 {
		t.Fatalf("retrieval result = %q, calls=%d", result, fake.calls)
	}
}
