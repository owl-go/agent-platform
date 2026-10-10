package workspace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/secretcrypto"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

func TestProviderNameConflictHTTPUsesProductionDatabase(t *testing.T) {
	fixture := expertHTTPDatabase(t)
	var databaseName string
	if err := fixture.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatal(err)
	}
	dsn, err := url.Parse(os.Getenv("WORKSPACE_TEST_POSTGRES_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	dsn.Path = "/" + databaseName
	database, err := gormdb.OpenWithoutMigrations(context.Background(), gormdb.Config{
		DSN: dsn.String(), MaxOpenConnections: 2, MaxIdleConnections: 1,
		ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	db := database.ORM()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,true)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}, box: box}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	call := func(method, path, name string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]any{"name": name, "provider_type": "openai", "endpoint": "https://example.test/v1", "protocols": []string{"openai_chat"}, "api_key": "fixture-only-key", "expected_version": 1})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner, Administrator: true}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	path := "/api/v1/model-provider-connections"
	first := call(http.MethodPost, path, "白白AI-GPT")
	if first.Code != http.StatusOK {
		t.Fatalf("first create: %d %s", first.Code, first.Body.String())
	}
	second := call(http.MethodPost, path, "Other")
	if second.Code != http.StatusOK {
		t.Fatalf("second create: %d %s", second.Code, second.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, response := range []*httptest.ResponseRecorder{call(http.MethodPost, path, "白白AI-GPT"), call(http.MethodPatch, path+"/"+created.ID, "白白AI-GPT")} {
		var failure struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusConflict || failure.Reason != "model_provider_name_conflict" || failure.Message != "Model Provider Connection name already exists; choose another name" {
			t.Fatalf("duplicate save must return public name conflict: %d %s", response.Code, response.Body.String())
		}
		for _, private := range []string{"SQLSTATE", "model_provider_connections_owner_user_id_name_key", "fixture-only-key"} {
			if strings.Contains(response.Body.String(), private) {
				t.Fatalf("private cause reached HTTP response: %s", private)
			}
		}
	}
}
