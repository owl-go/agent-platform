package anythingllm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClientUsesAnythingLLMVectorSearchAndBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/workspace/knowledge-base/vector-search" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"results": []any{map[string]any{"text": "grounded answer", "score": 0.9, "metadata": map[string]string{"title": "revision", "chunkSource": "guide.md"}}}})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Query(context.Background(), "base", 1, "question", 8, 6000)
	if err != nil || len(result.Citations) != 1 || !strings.Contains(result.Citations[0].Text, "grounded") {
		t.Fatalf("Query() = %+v, %v", result, err)
	}
}
