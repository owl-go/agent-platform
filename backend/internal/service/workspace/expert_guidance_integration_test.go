package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func expertHTTPDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKSPACE_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })
	name := "expert_http_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(`CREATE DATABASE "` + name + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`).Error; err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + name
	db, err := gorm.Open(postgres.Open(parsed.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestExpertMarkdownGuidanceRoundTripsThroughAuthenticatedHTTP(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	guidance := "# 审核流程\n\n保留 **Markdown** 与代码：\n```go\nfmt.Println(\"ok\")\n```\n"
	body, _ := json.Marshal(map[string]any{"expert": map[string]any{"name": "Reviewer", "introduction": "展示用介绍", "guidance": guidance}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/experts", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		Guidance     string `json:"guidance"`
		Introduction string `json:"introduction"`
		Available    bool   `json:"available"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Guidance != guidance || created.Introduction != "展示用介绍" || !created.Available {
		t.Fatalf("lost Expert definition: %+v", created)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/experts/"+created.ID, nil)
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var loaded struct {
		Guidance string `json:"guidance"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || loaded.Guidance != guidance {
		t.Fatalf("read status=%d body=%s", response.Code, response.Body.String())
	}
	// Retired structured writes cannot become current authoring inputs.
	body, _ = json.Marshal(map[string]any{"expert": map[string]any{"name": "Legacy", "introduction": "Display", "core_capability": "Review", "operating_procedure": "Inspect", "output_standard": "Report"}})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/experts", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity && response.Code != http.StatusBadRequest {
		t.Fatalf("retired input accepted: %d %s", response.Code, response.Body.String())
	}

}

func TestExpertTeamWritesRequireAdministratorBeforeResolvingContent(t *testing.T) {
	db := expertHTTPDatabase(t)
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	for _, test := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/expert-teams", `{"expert_team":{}}`},
		{http.MethodPatch, "/api/v1/expert-teams/" + uuid.NewString(), `{"expert_team":{},"expected_version":1}`},
		{http.MethodDelete, "/api/v1/expert-teams/" + uuid.NewString(), ""},
	} {
		t.Run(test.method, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: uuid.NewString()}))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestExpertAvatarAndStartersUseAuthorizedCASRevision(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner, other := uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := gormrepo.New(db, nil)
	app, _ := workspaceapplication.New(repo)
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	input := map[string]any{"name": "Avatar expert", "introduction": "Profile", "guidance": "# Guide", "icon": avatar, "starter_prompts": []string{"One", "Two", "Three"}}
	call := func(who, method, path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(accountapplication.WithPrincipal(req.Context(), accountdomain.Principal{UserID: who}))
		out := httptest.NewRecorder()
		server.ServeHTTP(out, req)
		return out
	}
	response := call(owner, http.MethodPost, "/api/v1/experts", map[string]any{"expert": input})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var item struct {
		ID       string   `json:"id"`
		Version  int64    `json:"version"`
		Icon     string   `json:"icon"`
		Starters []string `json:"starter_prompts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err, response.Body.String())
	}
	if item.Icon != avatar || len(item.Starters) != 3 {
		t.Fatal("profile lost")
	}
	path := "/api/v1/experts/" + item.ID
	if r := call(other, http.MethodGet, path, nil); r.Code != 404 {
		t.Fatal("foreign profile exposed")
	}
	input["icon"] = "sparkles"
	if r := call(owner, http.MethodPatch, path, map[string]any{"expert": input, "expected_version": item.Version}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := call(owner, http.MethodPatch, path, map[string]any{"expert": input, "expected_version": item.Version}); r.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale avatar revision status=%d body=%s", r.Code, r.Body.String())
	}
	input["icon"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not an image"))
	if r := call(owner, http.MethodPost, "/api/v1/experts", map[string]any{"expert": input}); r.Code < 400 {
		t.Fatal("invalid image accepted")
	}
	input["icon"] = "sparkles"
	input["starter_prompts"] = []string{"One", "Two", "Three", "Four"}
	if r := call(owner, http.MethodPost, "/api/v1/experts", map[string]any{"expert": input}); r.Code < 400 {
		t.Fatal("four starters accepted")
	}
	input["starter_prompts"] = []string{}
	input["core_capability"] = "retired"
	if r := call(owner, http.MethodPost, "/api/v1/experts", map[string]any{"expert": input}); r.Code < 400 {
		t.Fatal("retired fields accepted beside guidance")
	}
}
