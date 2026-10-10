package workspace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/expertpackage"
	"agent-platform/backend/internal/objectstore/memory"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

func TestImportedBundledSkillStaysInsideExpertAndSurvivesEditing(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	repository := gormrepo.New(db, nil)
	app, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	objects := memory.New()
	service := &Service{workspace: app, accounts: &accountapplication.Service{}, objects: objects}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	archive := expertArchiveFixture(t, map[string]string{
		".plugin/plugin.json":    `{"schema_version":1,"id":"example.bundled","version":"1.0.0","kind":"expert","expert":{"name":"Reviewer","introduction":"Review","guidance_file":"agents/reviewer.md","bundled_skills":["skills/review"]}}`,
		"agents/reviewer.md":     "# Review",
		"skills/review/SKILL.md": "---\nname: review\ndisplay_name: Review skill\ndescription: Review the supplied evidence.\n---\n\n# Review\nUse supplied evidence.\n",
	})
	call := func(path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	response := call("/api/v1/expert-packages/import", map[string]string{"archive": base64.StdEncoding.EncodeToString(archive)})
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	var created struct {
		Expert struct {
			ID string `json:"id"`
		} `json:"expert"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	expert, err := repository.GetExpert(context.Background(), owner, created.Expert.ID)
	if err != nil || len(expert.BundledSkills) != 1 {
		t.Fatalf("bundled skills=%+v %v", expert.BundledSkills, err)
	}
	if _, err := objects.Stat(context.Background(), expert.BundledSkills[0].ObjectKey); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Table("skills").Count(&count)
	if count != 0 {
		t.Fatal("bundled content registered as a catalog Skill")
	}
	input := domain.ExpertInput{Name: expert.Name, Introduction: expert.Introduction, Guidance: expert.Guidance + "\nUpdated"}
	updated, err := repository.UpdateExpert(context.Background(), owner, expert.ID, input, expert.Version)
	if err != nil || len(updated.BundledSkills) != 1 || updated.BundledSkills[0].ObjectKey != expert.BundledSkills[0].ObjectKey {
		t.Fatal("editing discarded the bound immutable bundle", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/expert-packages/expert/"+expert.ID+"/export", nil)
	req = req.WithContext(accountapplication.WithPrincipal(req.Context(), accountdomain.Principal{UserID: owner}))
	exported := httptest.NewRecorder()
	server.ServeHTTP(exported, req)
	if exported.Code != http.StatusOK {
		t.Fatal(exported.Body.String())
	}
	var result struct {
		Archive []byte `json:"archive"`
	}
	if err := json.Unmarshal(exported.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	parsed, err := expertpackage.Parse(context.Background(), result.Archive)
	if err != nil || parsed.Expert.Guidance != updated.Guidance || len(parsed.BundledSkills) != 1 {
		t.Fatalf("edited export lost bound content: %v", err)
	}
	if replay := call("/api/v1/expert-packages/import", map[string]string{"archive": base64.StdEncoding.EncodeToString(archive)}); replay.Code != http.StatusOK {
		t.Fatal(replay.Body.String())
	}
}
