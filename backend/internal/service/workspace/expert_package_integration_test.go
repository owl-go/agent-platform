package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

func expertArchiveFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestExpertPackageImportUsesTheSameAuthenticatedMarkdownDefinition(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	guidance := "# Review\n\nRead the supplied evidence.\n"
	archive := expertArchiveFixture(t, map[string]string{
		".plugin/plugin.json": `{"schema_version":1,"id":"example.reviewer","version":"1.0.0","kind":"expert","expert":{"name":"Reviewer","introduction":"Evidence review","guidance_file":"agents/reviewer.md"}}`,
		"agents/reviewer.md":  guidance,
	})
	body, _ := json.Marshal(map[string]any{"archive": base64.StdEncoding.EncodeToString(archive)})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/expert-packages/import", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Expert struct {
			ID       string `json:"id"`
			Guidance string `json:"guidance"`
		} `json:"expert"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Expert.ID == "" || result.Expert.Guidance != guidance {
		t.Fatalf("lost imported definition: %+v", result)
	}
	originalID := result.Expert.ID
	request = httptest.NewRequest(http.MethodPost, "/api/v1/expert-packages/import", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("reimport status=%d body=%s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Expert.ID != originalID {
		t.Fatal("identical package import created another Expert")
	}
	changed := expertArchiveFixture(t, map[string]string{
		".plugin/plugin.json": `{"schema_version":1,"id":"example.reviewer","version":"1.0.0","kind":"expert","expert":{"name":"Reviewer","introduction":"Evidence review","guidance_file":"agents/reviewer.md"}}`,
		"agents/reviewer.md":  "# Changed",
	})
	body, _ = json.Marshal(map[string]any{"archive": base64.StdEncoding.EncodeToString(changed)})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/expert-packages/import", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("same-version conflict status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExpertPackageRevisionRoundtripAndPrivateExport(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner, other := uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)`, id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := gormrepo.New(db, nil)
	app, _ := workspaceapplication.New(repo)
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	call := func(user, method, path string, payload any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: user}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	makePackage := func(version, guide string) []byte {
		return expertArchiveFixture(t, map[string]string{".plugin/plugin.json": `{"schema_version":1,"id":"example.revision","version":"` + version + `","kind":"expert","expert":{"name":"Revision","introduction":"Review","guidance_file":"agents/reviewer.md"}}`, "agents/reviewer.md": guide})
	}
	first := makePackage("1.0.0", "# Original\n")
	response := call(owner, http.MethodPost, "/api/v1/expert-packages/import", map[string]any{"archive": base64.StdEncoding.EncodeToString(first)})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var imported struct {
		Expert struct {
			ID       string `json:"id"`
			Guidance string `json:"guidance"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	id := imported.Expert.ID
	response = call(other, http.MethodGet, "/api/v1/expert-packages/expert/"+id+"/export", nil)
	if response.Code != 404 {
		t.Fatalf("private export status=%d", response.Code)
	}
	response = call(owner, http.MethodGet, "/api/v1/expert-packages/expert/"+id+"/export", nil)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var exported struct {
		Archive []byte `json:"archive"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	response = call(owner, http.MethodPost, "/api/v1/expert-packages/import", map[string]any{"archive": base64.StdEncoding.EncodeToString(exported.Archive)})
	if response.Code != 200 {
		t.Fatalf("roundtrip: %s", response.Body.String())
	}
	response = call(owner, http.MethodPost, "/api/v1/expert-packages/import", map[string]any{"archive": base64.StdEncoding.EncodeToString(makePackage("1.1.0", "# Upgrade\n")), "expected_version": 1})
	if response.Code != 200 {
		t.Fatalf("upgrade: %s", response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Expert.ID != id || imported.Expert.Guidance != "# Upgrade\n" {
		t.Fatalf("upgrade changed identity or lost text: %+v", imported)
	}
	var count int64
	if err := db.Table("expert_definition_revisions").Where("expert_id=?", id).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("revisions=%d err=%v", count, err)
	}
}

func TestExpertPackageRejectsUnsafeDefinitionsWithoutCreatingResources(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	app, _ := workspaceapplication.New(gormrepo.New(db, nil))
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	valid := `{"schema_version":1,"id":"example.safety","version":"1.0.0","kind":"expert","expert":{"name":"Safety","introduction":"Review","guidance_file":"agents/reviewer.md"}}`
	for _, problem := range []string{"unknown field", "schema", "version", "escape", "duplicate JSON key", "empty guidance", "unreferenced file"} {
		t.Run(problem, func(t *testing.T) {
			manifest := valid
			files := map[string]string{"agents/reviewer.md": "# Review"}
			switch problem {
			case "unknown field":
				manifest = strings.Replace(manifest, `"schema_version":1`, `"secret":"forbidden","schema_version":1`, 1)
			case "schema":
				manifest = strings.Replace(manifest, `"schema_version":1`, `"schema_version":2`, 1)
			case "version":
				manifest = strings.Replace(manifest, `1.0.0`, `01.0.0`, 1)
			case "escape":
				files["../escaped.md"] = "escape"
			case "duplicate JSON key":
				manifest = strings.Replace(manifest, `"schema_version":1`, `"schema_version":2,"schema_version":1`, 1)
			case "empty guidance":
				files["agents/reviewer.md"] = " "
			case "unreferenced file":
				files["agents/extra.md"] = "extra"
			}
			files[".plugin/plugin.json"] = manifest
			body, _ := json.Marshal(map[string]any{"archive": base64.StdEncoding.EncodeToString(expertArchiveFixture(t, files))})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/expert-packages/import", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != 422 {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	var count int64
	if err := db.Table("experts").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unsafe package left %d resources, err=%v", count, err)
	}
}

func TestExpertTeamPackageRequiresAdministratorAndOwnsItsRoster(t *testing.T) {
	db := expertHTTPDatabase(t)
	admin, user := uuid.NewString(), uuid.NewString()
	for _, id := range []string{admin, user} {
		if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,?)`, id, id, id, id+"@example.test", id, id == admin).Error; err != nil {
			t.Fatal(err)
		}
	}
	app, _ := workspaceapplication.New(gormrepo.New(db, nil))
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	archive := expertArchiveFixture(t, map[string]string{".plugin/plugin.json": `{"schema_version":1,"id":"example.team","version":"1.0.0","kind":"expert_team","team":{"name":"Review Team","introduction":"Review evidence","core_capability":"Review and synthesize","lead_member_id":"lead","members":[{"id":"lead","name":"Lead","labels":["synthesis"],"expert":{"name":"Lead","introduction":"Coordinate","guidance_file":"agents/lead.md"}},{"id":"reviewer","name":"Reviewer","labels":["review"],"expert":{"name":"Reviewer","introduction":"Review","guidance_file":"agents/reviewer.md"}}]}}`, "agents/lead.md": "# Coordinate\n", "agents/reviewer.md": "# Review\n"})
	call := func(owner, method, path string, payload any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner, Administrator: owner == admin}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	payload := map[string]any{"archive": base64.StdEncoding.EncodeToString(archive)}
	response := call(user, http.MethodPost, "/api/v1/expert-packages/import", payload)
	if response.Code != 403 {
		t.Fatalf("ordinary import=%d %s", response.Code, response.Body.String())
	}
	response = call(admin, http.MethodPost, "/api/v1/expert-packages/import", payload)
	if response.Code != 200 {
		t.Fatalf("admin import=%d %s", response.Code, response.Body.String())
	}
	var result struct {
		ExpertTeam struct {
			ID           string `json:"id"`
			LeadMemberID string `json:"lead_member_id"`
			Members      []struct {
				ID     string `json:"id"`
				Expert struct {
					Guidance string `json:"guidance"`
				}
			} `json:"members"`
		} `json:"expert_team"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ExpertTeam.LeadMemberID != "lead" || len(result.ExpertTeam.Members) != 2 || result.ExpertTeam.Members[1].Expert.Guidance != "# Review\n" {
		t.Fatalf("lost roster: %+v", result)
	}
	response = call(user, http.MethodGet, "/api/v1/expert-packages/expert_team/"+result.ExpertTeam.ID+"/export", nil)
	if response.Code != 200 {
		t.Fatalf("visible team export=%d %s", response.Code, response.Body.String())
	}
	var exported struct {
		Archive []byte `json:"archive"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	response = call(user, http.MethodPost, "/api/v1/expert-packages/import", map[string]any{"archive": base64.StdEncoding.EncodeToString(exported.Archive), "copy_name": "My Team"})
	if response.Code != 403 {
		t.Fatalf("copy bypass=%d %s", response.Code, response.Body.String())
	}
	response = call(admin, http.MethodPost, "/api/v1/expert-packages/import", map[string]any{"archive": base64.StdEncoding.EncodeToString(exported.Archive)})
	if response.Code != 200 {
		t.Fatalf("team roundtrip=%d %s", response.Code, response.Body.String())
	}
}
