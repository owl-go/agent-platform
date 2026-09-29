package runtimeexecutor

import (
	"context"
	"math"
	"strings"
	"testing"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/retrieval"
)

type retrievalFake struct{ calls int }

func (fake *retrievalFake) Search(_ context.Context, _, _ string, _ int64, _ string, _, _ int) ([]retrieval.Hit, error) {
	fake.calls++
	return []retrieval.Hit{{Text: "same evidence", Relevance: .9, Source: workspacedomain.KnowledgeSearchSource{DocumentID: "document-1", DocumentName: "source.md", RevisionID: "revision"}}}, nil
}

func TestKnowledgeEvidenceBoundsProviderMetadata(t *testing.T) {
	if got := truncateEvidenceText(strings.Repeat("界", 501), 500); len([]rune(got)) != 500 {
		t.Fatalf("bounded source location has %d runes", len([]rune(got)))
	}
	for input, want := range map[float32]float32{-1: 0, 0.4: 0.4, 2: 1, float32(math.NaN()): 0} {
		if got := boundedRelevance(input); got != want {
			t.Fatalf("boundedRelevance(%v) = %v, want %v", input, got, want)
		}
	}
}

func TestInjectKnowledgeContextBoundsAndDeduplicatesCitations(t *testing.T) {
	fake := &retrievalFake{}
	executor := &Executor{knowledge: fake}
	result, evidence, err := executor.injectKnowledgeContext(context.Background(), "owner", workspacedomain.ExecutionSnapshot{Goal: "answer", KnowledgeBaseIDs: []string{"base-1", "base-2"}, KnowledgeIndexGenerations: map[string]int64{"base-1": 1, "base-2": 1}}, "question", 2)
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || strings.Count(result, "same evidence") != 1 {
		t.Fatalf("retrieval result = %q, calls=%d", result, fake.calls)
	}
	if len(evidence) != 2 || evidence[0].Kind != "knowledge" || evidence[0].State != "succeeded" || evidence[0].ContainerID != "base-1" || evidence[0].StagePosition != 2 || evidence[0].Citation == nil || evidence[1].State != "not_used" {
		t.Fatalf("unexpected Knowledge Evidence: %#v", evidence)
	}
}
