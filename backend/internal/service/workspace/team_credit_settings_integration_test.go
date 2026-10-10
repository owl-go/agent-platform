package workspace

import (
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

func TestUserCanConfigureOptionalTeamResponseAdmissionBudgetThroughHTTP(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO personal_settings(user_id) VALUES(?)", owner).Error; err != nil {
		t.Fatal(err)
	}
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	call := func(method, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "/api/v1/settings", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	initial := call(http.MethodGet, "")
	if initial.Code != http.StatusOK {
		t.Fatal(initial.Body.String())
	}
	var settings map[string]any
	if err := json.Unmarshal(initial.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	settings["expected_version"] = settings["version"]
	settings["team_credit_budget_hundredths"] = 150
	for _, key := range []string{"version", "execution_inherited", "platform_execution_available"} {
		delete(settings, key)
	}
	data, _ := json.Marshal(settings)
	updated := call(http.MethodPatch, string(data))
	if updated.Code != http.StatusOK {
		t.Fatal(updated.Body.String())
	}
	var actual struct {
		TeamCreditBudgetHundredths int64 `json:"team_credit_budget_hundredths"`
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &actual); err != nil || actual.TeamCreditBudgetHundredths != 150 {
		t.Fatalf("budget=%+v %v", actual, err)
	}
	var version int64
	db.Table("personal_settings").Select("version").Where("user_id = ?", owner).Scan(&version)
	settings["expected_version"] = version
	settings["team_credit_budget_hundredths"] = -1
	data, _ = json.Marshal(settings)
	if invalid := call(http.MethodPatch, string(data)); invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative budget accepted: %d %s", invalid.Code, invalid.Body.String())
	}
}
