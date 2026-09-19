package runtimeexecutor

import (
	"context"
	"strings"
	"testing"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
)

type retrievalFake struct{ calls int }

func (fake *retrievalFake) EnsureWorkspace(context.Context, string) error { return nil }
func (fake *retrievalFake) DeleteWorkspace(context.Context, string) error { return nil }
func (fake *retrievalFake) UpsertRevision(context.Context, string, string, string, []byte) error {
	return nil
}
func (fake *retrievalFake) RemoveRevision(context.Context, string, string) error { return nil }
func (fake *retrievalFake) Query(_ context.Context, base string, _ int64, _ string, _ int, _ int) (anythingllm.Retrieval, error) {
	fake.calls++
	return anythingllm.Retrieval{Citations: []anythingllm.Citation{{KnowledgeBaseID: base, Text: "same evidence", SourceLocation: "source.md", Relevance: .9}}}, nil
}

func TestInjectKnowledgeContextBoundsAndDeduplicatesCitations(t *testing.T) {
	fake := &retrievalFake{}
	executor := &Executor{knowledge: fake}
	result, err := executor.injectKnowledgeContext(context.Background(), workspacedomain.ExecutionSnapshot{Goal: "answer", KnowledgeBaseIDs: []string{"base-1", "base-2"}, KnowledgeIndexGenerations: map[string]int64{"base-1": 1, "base-2": 1}}, "question")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || strings.Count(result, "same evidence") != 1 {
		t.Fatalf("retrieval result = %q, calls=%d", result, fake.calls)
	}
}
