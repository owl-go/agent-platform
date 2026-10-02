package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/retrieval"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

type previewRepository struct {
	workspaceapplication.Repository
	base, owner string
}

func (r *previewRepository) ReadyKnowledgeSearchGeneration(_ context.Context, owner, base string, _ bool) (int64, error) {
	if owner != r.owner || base != r.base {
		return 0, workspacedomain.ErrNotFound
	}
	return 1, nil
}

type previewSearcher struct{ calls int }

func (s *previewSearcher) Search(context.Context, string, string, int64, string, int, int) ([]retrieval.Hit, error) {
	s.calls++
	return []retrieval.Hit{{Source: workspacedomain.KnowledgeSearchSource{DocumentID: "document", RevisionID: "revision", DocumentName: "source.txt"}, Text: "indexed source"}}, nil
}

// Exercise the actual Kratos router: it does not populate net/http PathValue.
func TestKnowledgePreviewReadsBaseFromKratosRequestPath(t *testing.T) {
	base, owner := uuid.NewString(), uuid.NewString()
	app, err := workspaceapplication.New(&previewRepository{base: base, owner: owner})
	if err != nil {
		t.Fatal(err)
	}
	searcher := &previewSearcher{}
	service := &Service{accounts: &accountapplication.Service{}, workspace: app, knowledgeSearch: searcher}
	server := kratoshttp.NewServer()
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/search", http.HandlerFunc(service.searchKnowledgeBase))
	for _, test := range []struct {
		base   string
		status int
	}{{base, http.StatusOK}, {uuid.NewString(), http.StatusNotFound}, {"invalid", http.StatusNotFound}} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases/"+test.base+"/search?q=question", nil)
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("preview returned %d, want %d: %s", response.Code, test.status, response.Body.String())
		}
		if test.status == http.StatusOK {
			var result struct {
				IndexReady bool                    `json:"index_ready"`
				Items      []knowledgeSearchResult `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || !result.IndexReady || len(result.Items) != 1 || result.Items[0].Text != "indexed source" {
				t.Fatalf("preview result=%+v err=%v", result, err)
			}
		}
	}
	if searcher.calls != 1 {
		t.Fatalf("searcher called %d times", searcher.calls)
	}
}
