package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type selectionHTTPRepository struct {
	application.Repository
	application.ConversationRepository
}

func (r *selectionHTTPRepository) GetConversationSelection(_ context.Context, owner string, scope domain.ConversationScope) (domain.ConversationSelection, error) {
	if owner != "owner" || scope.SessionID != "session" {
		return domain.ConversationSelection{}, domain.ErrNotFound
	}
	return domain.ConversationSelection{ID: "revision", Name: "Reviewer", MCPServers: []domain.MCPServerSnapshot{{ID: "server", Name: "Search", SecretCiphertext: []byte("sensitive"), Configuration: json.RawMessage(`{"token":"private-config"}`)}}, Skills: []domain.SkillSnapshot{{ID: "skill", Name: "PDF", ObjectKey: "private/object/key", SHA256: "digest"}}}, nil
}
func TestConversationSelectionHTTPUsesAuthenticatedScopeAndOnlyPublicMetadata(t *testing.T) {
	repository := &selectionHTTPRepository{}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	for _, test := range []struct {
		owner, session string
		status         int
	}{{"owner", "session", 200}, {"other", "session", 404}, {"owner", "other", 404}, {"", "session", 401}} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/conversation-selection?session_id="+test.session, nil)
		if test.owner != "" {
			request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: test.owner}))
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if test.status == 200 {
			if !strings.Contains(response.Body.String(), "Search") || !strings.Contains(response.Body.String(), "digest") {
				t.Fatal("missing display metadata")
			}
			for _, secret := range []string{"sensitive", "private-config", "private/object/key", "secret_ciphertext", "configuration"} {
				if strings.Contains(response.Body.String(), secret) {
					t.Fatalf("exposed %q", secret)
				}
			}
		}
	}
}
