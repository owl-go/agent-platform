package ragflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type testStore struct {
	mapping Mapping
	saved   int
	deleted int
}

func (s *testStore) GetMapping(context.Context, string, string) (Mapping, error) {
	if s.mapping.DocumentID == "" {
		return Mapping{}, ErrMappingNotFound
	}
	return s.mapping, nil
}
func (s *testStore) SaveMapping(_ context.Context, _ string, m Mapping) error {
	s.mapping = m
	s.saved++
	return nil
}
func (s *testStore) GenerationMappings(context.Context, string, string, int64) ([]Mapping, error) {
	return []Mapping{s.mapping}, nil
}
func (s *testStore) DeleteMapping(context.Context, string, string) error {
	s.deleted++
	s.mapping = Mapping{}
	return nil
}
func (s *testStore) CleanupMappings(context.Context, string) ([]Mapping, error) { return nil, nil }

func TestUploadParseRetrieveAndResume(t *testing.T) {
	base, revision := uuid.NewString(), uuid.NewString()
	store := &testStore{}
	uploaded, parsed, chunkChecks := 0, 0, 0
	done := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-key" {
			t.Error("missing credential")
		}
		data := any(nil)
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/datasets":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 108})
			return
		case "POST /api/v1/datasets":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["permission"] != "me" || body["embedding_model"] != "bge-m3@Ollama" {
				t.Errorf("dataset body=%v", body)
			}
			data = map[string]any{"id": "dataset"}
		case "POST /api/v1/datasets/dataset/documents":
			uploaded++
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			if header.Filename != revision+".pdf" {
				t.Errorf("filename=%s", header.Filename)
			}
			data = []any{map[string]any{"id": "document"}}
		case "GET /api/v1/datasets/dataset/documents":
			if r.URL.Query().Get("name") != "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 102})
				return
			}
			docs := []any{}
			if r.URL.Query().Get("id") == "document" {
				run, chunks := "UNSTART", 0
				if done {
					run, chunks = "DONE", 2
				}
				docs = append(docs, map[string]any{"id": "document", "run": run, "chunk_count": chunks})
			}
			data = map[string]any{"docs": docs}
		case "GET /api/v1/datasets/dataset/documents/document/chunks":
			chunkChecks++
			chunks := []any{}
			if chunkChecks > 1 {
				chunks = append(chunks, map[string]any{"document_id": "document", "content": "indexed source", "available": true})
			}
			data = map[string]any{"chunks": chunks}
		case "POST /api/v1/datasets/dataset/chunks":
			parsed++
			done = true
		case "POST /api/v1/retrieval":
			var body struct {
				Documents []string `json:"document_ids"`
				Datasets  []string `json:"dataset_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Documents) != 1 || body.Documents[0] != "document" || len(body.Datasets) != 1 || body.Datasets[0] != "dataset" {
				t.Errorf("unbounded retrieval: %+v", body)
			}
			data = map[string]any{"chunks": []any{map[string]any{"document_id": "foreign", "content": "leak"}, map[string]any{"document_id": "document", "dataset_id": "other", "content": "leak"}, map[string]any{"document_id": "document", "dataset_id": "dataset", "content": "检索内容", "similarity": 0.9}}}
		case "DELETE /api/v1/datasets/dataset/documents":
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
	}))
	defer server.Close()
	c, err := New(Config{Endpoint: server.URL, APIKey: "private-key", DeploymentID: "test", EmbeddingModel: "bge-m3@Ollama", PollInterval: time.Millisecond}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := c.UpsertRevision(context.Background(), base, revision, "application/pdf", []byte("pdf source")); err != nil {
			t.Fatal(err)
		}
	}
	if uploaded != 1 || parsed != 1 || store.saved != 1 || chunkChecks != 3 {
		t.Fatalf("upload=%d parse=%d save=%d chunk checks=%d", uploaded, parsed, store.saved, chunkChecks)
	}
	result, err := c.Query(context.Background(), base, 1, "question", 10, 2)
	if err != nil || len(result.Citations) != 1 || result.Citations[0].RevisionID != revision || result.Citations[0].Text != "检索" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := c.RemoveRevision(context.Background(), base, revision); err != nil {
		t.Fatal(err)
	}
	if store.deleted != 1 {
		t.Fatal("mapping was not removed")
	}
}

func TestParseMustCompleteWithChunks(t *testing.T) {
	for _, state := range []string{"RUNNING", "DONE", "FAIL"} {
		t.Run(state, func(t *testing.T) {
			store := &testStore{mapping: Mapping{BaseID: uuid.NewString(), RevisionID: uuid.NewString(), DatasetID: "dataset", DocumentID: "document"}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					fmt.Fprintf(w, `{"code":0,"data":{"docs":[{"id":"document","run":%q,"chunk_count":0}]}}`, state)
				} else {
					fmt.Fprint(w, `{"code":0}`)
				}
			}))
			defer server.Close()
			c, _ := New(Config{Endpoint: server.URL, APIKey: "secret", DeploymentID: "test", EmbeddingModel: "local", PollInterval: time.Millisecond, ParseTimeout: 10 * time.Millisecond}, store, nil)
			if err := c.UpsertRevision(context.Background(), store.mapping.BaseID, store.mapping.RevisionID, "text/plain", []byte("text")); err == nil {
				t.Fatal("unverified parse was accepted")
			}
		})
	}
}

func TestProviderErrorsDoNotExposeSecretsOrFollowRedirects(t *testing.T) {
	for _, response := range []string{`{"code":102,"message":"secret-key source-body"}`, `{"message":"secret-key"}`, "<html>secret-key</html>"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, response) }))
		c, _ := New(Config{Endpoint: server.URL, APIKey: "secret-key", DeploymentID: "test", EmbeddingModel: "local"}, &testStore{}, nil)
		err := c.json(context.Background(), "GET", "/datasets", nil, nil)
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "source-body") {
			t.Fatalf("unsafe error=%v", err)
		}
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect leaked credential") }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	c, _ := New(Config{Endpoint: server.URL, APIKey: "secret", DeploymentID: "test", EmbeddingModel: "local"}, &testStore{}, nil)
	if err := c.json(context.Background(), "GET", "/datasets", nil, nil); err == nil {
		t.Fatal("redirect accepted")
	}
}
