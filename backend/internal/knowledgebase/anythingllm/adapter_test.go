package anythingllm

import (
	"context"
	"testing"
)

type fakeAdapter struct{ queried bool }

func (fake *fakeAdapter) EnsureWorkspace(context.Context, string) error { return nil }
func (fake *fakeAdapter) DeleteWorkspace(context.Context, string) error { return nil }
func (fake *fakeAdapter) UpsertRevision(context.Context, string, string, string, []byte) error {
	return nil
}
func (fake *fakeAdapter) RemoveRevision(context.Context, string, string) error { return nil }
func (fake *fakeAdapter) Query(_ context.Context, base string, generation int64, query string, limit int, tokens int) (Retrieval, error) {
	fake.queried = true
	return Retrieval{Citations: []Citation{{KnowledgeBaseID: base, RevisionID: "revision", Text: query}}, NoHit: false}, nil
}

func TestAdapterContractKeepsProviderDetailsOutOfRetrieval(t *testing.T) {
	var adapter Adapter = &fakeAdapter{}
	result, err := adapter.Query(context.Background(), "base", 4, "goal + input", 8, 6000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Citations) != 1 || result.Citations[0].KnowledgeBaseID != "base" {
		t.Fatalf("unexpected retrieval: %+v", result)
	}
}
