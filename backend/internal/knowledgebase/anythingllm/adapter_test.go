package anythingllm

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestUpsertRevisionExplicitlyEmbedsAndVerifiesWorkspaceMembership(t *testing.T) {
	revisionID := "11111111-1111-4111-8111-111111111111"
	location := "custom-documents/" + revisionID + ".txt-hash.json"
	embedded, uploads, updates := false, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer authorization")
		}
		switch request.URL.Path {
		case "/v1/workspace/knowledge-base":
			docs := []any{}
			if embedded {
				docs = append(docs, map[string]string{"docpath": location, "filename": revisionID + ".txt"})
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"workspace": []any{map[string]any{"documents": docs}}})
		case "/v1/document/upload":
			uploads++
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			if _, header, err := request.FormFile("file"); err != nil || header.Filename != revisionID+".txt" {
				t.Errorf("uploaded file = %v, %v", header, err)
			}
			if request.FormValue("addToWorkspaces") != "" {
				t.Error("upload must not claim embedding by itself")
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"success": true, "documents": []any{map[string]string{"location": location}}})
		case "/v1/workspace/knowledge-base/update-embeddings":
			updates++
			var payload map[string][]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if adds := payload["adds"]; len(adds) == 1 && adds[0] == location {
				embedded = true
			} else if deletes := payload["deletes"]; len(deletes) == 1 && deletes[0] == location {
				embedded = false
			} else {
				t.Errorf("unexpected embedding operation: %+v", payload)
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"workspace": map[string]any{"documents": []any{}}, "message": nil})
		default:
			http.Error(writer, fmt.Sprintf("unexpected path %s", request.URL.Path), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 2; repeat++ {
		if err := client.UpsertRevision(context.Background(), "base", revisionID, "text/plain", []byte("content")); err != nil {
			t.Fatal(err)
		}
	}
	if uploads != 2 || updates != 3 {
		t.Fatalf("idempotent indexing uploads=%d, embedding updates=%d", uploads, updates)
	}
	if err := client.RemoveRevision(context.Background(), "base", revisionID); err != nil || embedded || updates != 4 {
		t.Fatalf("remove revision: %v, embedded=%v, updates=%d", err, embedded, updates)
	}
}

func TestEnsureWorkspaceDoesNotCreateOnProviderFailure(t *testing.T) {
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			created = true
		}
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureWorkspace(context.Background(), "base"); err == nil || created {
		t.Fatalf("provider failure created workspace: err=%v created=%t", err, created)
	}
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
