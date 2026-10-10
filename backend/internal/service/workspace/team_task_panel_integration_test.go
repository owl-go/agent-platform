package workspace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

func TestTeamTaskPanelHTTPReturnsActualInvocationFactsWithoutInternalAccounting(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner, session := uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", session, owner).Error; err != nil {
		t.Fatal(err)
	}
	credit := domain.CreditStageConsumption{StagePosition: 1, AmountHundredths: 17, InputTokens: 98765, InputMultiplierMicros: 3333333, RateRevisionID: "private-rate", ProviderModel: "private-model"}
	stages, _ := json.Marshal([]domain.ExpertStage{{InvocationID: "actual-lead", Role: "lead", TeamMemberID: "lead", TeamMemberName: "Lead", State: "succeeded", Position: 1, ModelInvoked: true, FinalText: `{"action":"complete","response":"Official"}`, StartedAt: time.Now(), CreditConsumption: &credit}, {InvocationID: "actual-member", Role: "member", TeamMemberID: "reviewer", TeamMemberName: "Reviewer", TaskID: "review", State: "succeeded", Position: 2, ModelInvoked: true, FinalText: "Reviewed evidence", RepairOf: "previous-review", WorkspaceConflicts: []domain.TeamWorkspaceConflict{{Path: "report.md", TaskIDs: []string{"review", "previous-review"}, State: "resolved", ResolutionSourceTaskID: "review"}}}})
	consumption, _ := json.Marshal(domain.CreditConsumption{TotalHundredths: 17, Stages: []domain.CreditStageConsumption{credit}})
	if err := db.Exec(`INSERT INTO session_messages(session_id,role,content,state,expert_stages,credit_consumption) VALUES(?,'assistant','Official','completed',?::jsonb,?::jsonb)`, session, string(stages), string(consumption)).Error; err != nil {
		t.Fatal(err)
	}
	app, err := workspaceapplication.New(gormrepo.New(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session+"/messages", nil)
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	content := response.Body.String()
	for _, value := range []string{"actual-lead", "actual-member", "previous-review", "report.md", "Reviewed evidence"} {
		if !strings.Contains(content, value) {
			t.Fatalf("missing Task Panel fact %s", value)
		}
	}
	for _, value := range []string{"98765", "3333333", "private-rate", "private-model", `\"action\"`} {
		if strings.Contains(content, value) {
			t.Fatalf("Task Panel disclosed %s", value)
		}
	}
	var messageID int64
	if err := db.Table("session_messages").Where("session_id=?", session).Select("id").Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/sessions/%s/messages/%d/events", session, messageID), nil)
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: owner}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	for _, value := range []string{"98765", "3333333", "private-rate", "private-model", `\"action\"`} {
		if strings.Contains(response.Body.String(), value) {
			t.Fatalf("stream disclosed %s", value)
		}
	}

}
