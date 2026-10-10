package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/resourceaction"
	"context"
	"encoding/json"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConversationalExpertConfirmationAndRevisionAreAuthenticatedAndAtomic(t *testing.T) {
	db := expertHTTPDatabase(t)
	owner := uuid.NewString()
	admin := uuid.NewString()
	for _, id := range []string{owner, admin} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,?)", id, id, id, id+"@example.test", id, id == admin).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := gormrepo.New(db, nil)
	app, err := workspaceapplication.New(repo)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}}
	server := kratoshttp.NewServer()
	service.RegisterHTTP(server)
	call := func(who, id string, input any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/resource-creation-actions/"+id+"/decision", strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(accountapplication.WithPrincipal(req.Context(), accountdomain.Principal{UserID: who, Administrator: who == admin}))
		out := httptest.NewRecorder()
		server.ServeHTTP(out, req)
		return out
	}
	session := uuid.NewString()
	if err := db.Exec("INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,?)", session, owner, "Authoring").Error; err != nil {
		t.Fatal(err)
	}
	var messageID int64
	if err := db.Raw("INSERT INTO session_messages(session_id,role,state,content) VALUES(?,'assistant','completed','Preview') RETURNING id", session).Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}
	proposal := resourceaction.Proposal{Kind: resourceaction.ExpertKind, Expert: &resourceaction.ExpertProposal{Name: "Review", Introduction: "Display", Guidance: "# Review", StarterPrompts: []string{"Review evidence"}}}
	action, err := repo.CreateResourceCreationAction(context.Background(), owner, session, messageID, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(admin, action.ID, map[string]string{"decision": "confirm"}); response.Code != http.StatusNotFound {
		t.Fatal("foreign action exposed", response.Body.String())
	}
	proposal.Expert.Name = "Revised Review"
	response := call(owner, action.ID, map[string]any{"decision": "revise", "expected_version": action.Version, "proposal": proposal})
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	if response := call(owner, action.ID, map[string]any{"decision": "revise", "expected_version": action.Version, "proposal": proposal}); response.Code != http.StatusConflict {
		t.Fatal("stale revision accepted", response.Body.String())
	}
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- call(owner, action.ID, map[string]string{"decision": "confirm"}) }()
	}
	wg.Wait()
	close(results)
	for response := range results {
		if response.Code != http.StatusOK {
			t.Fatal(response.Body.String())
		}
	}
	experts, err := repo.ListExperts(context.Background(), owner)
	if err != nil || len(experts) != 1 || experts[0].Name != "Revised Review" || len(experts[0].StarterPrompts) != 1 {
		t.Fatal("confirmation was not atomic", experts, err)
	}
	var sessions int64
	db.Table("sessions").Count(&sessions)
	if sessions != 1 {
		t.Fatal("saving created another conversation")
	}
	team := resourceaction.Proposal{Kind: resourceaction.TeamKind, Team: &resourceaction.TeamProposal{Name: "Team", Introduction: "Review", CoreCapability: "Review", LeadMemberID: "lead", Members: []resourceaction.TeamMemberProposal{{ID: "lead", Name: "Lead", Expert: resourceaction.ExpertProposal{Name: "Lead", Introduction: "Coordinate", Guidance: "# Coordinate"}}, {ID: "review", Name: "Review", Expert: resourceaction.ExpertProposal{Name: "Review", Introduction: "Review", Guidance: "# Review"}}}}}
	if _, err := repo.CreateResourceCreationAction(context.Background(), owner, session, messageID, team); err == nil {
		t.Fatal("ordinary User created a Team proposal")
	}
	adminSession := uuid.NewString()
	if err := db.Exec("INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,?)", adminSession, admin, "Team authoring").Error; err != nil {
		t.Fatal(err)
	}
	var adminMessage int64
	if err := db.Raw("INSERT INTO session_messages(session_id,role,state,content) VALUES(?,'assistant','completed','Team preview') RETURNING id", adminSession).Scan(&adminMessage).Error; err != nil {
		t.Fatal(err)
	}
	teamAction, err := repo.CreateResourceCreationAction(context.Background(), admin, adminSession, adminMessage, team)
	if err != nil {
		t.Fatal(err)
	}
	if r := call(admin, teamAction.ID, map[string]string{"decision": "confirm"}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	teams, err := repo.ListExpertTeams(context.Background(), owner)
	if err != nil || len(teams) != 1 || teams[0].LeadMemberID != "lead" || teams[0].Members[0].Expert.Guidance != "# Coordinate" {
		t.Fatal("confirmed Team differs from validated preview", err)
	}
	// Terminal legacy previews remain readable, but cannot become current input.
	legacyID := uuid.NewString()
	var legacyMessage int64
	if err := db.Raw("INSERT INTO session_messages(session_id,role,state,content) VALUES(?,'assistant','completed','Old preview') RETURNING id", session).Scan(&legacyMessage).Error; err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"kind":"expert","expert":{"name":"Old","core_capability":"Legacy"}}`)
	if err := db.Exec("INSERT INTO resource_creation_actions(id,owner_user_id,session_id,message_id,kind,state,name,description,payload,expires_at) VALUES(?,?,?,?, 'expert','expired','Old','History',?,now())", legacyID, owner, session, legacyMessage, legacy).Error; err != nil {
		t.Fatal(err)
	}
	if _, preview, err := repo.GetResourceCreationAction(context.Background(), owner, legacyID); err != nil || preview.Expert != nil {
		t.Fatal("terminal legacy preview cannot be read inertly", err)
	}
	if r := call(owner, legacyID, map[string]string{"decision": "confirm"}); r.Code != 409 {
		t.Fatal("terminal legacy action became writable")
	}
	// An old or forged stored preview still cannot bypass confirmation permissions.
	forged := action.ID
	payload, _ := team.JSON()
	if err := db.Exec("UPDATE resource_creation_actions SET kind='expert_team',payload=?,state='pending',resource_id=NULL WHERE id=?", payload, action.ID).Error; err != nil {
		t.Fatal(err)
	}
	if response := call(owner, forged, map[string]string{"decision": "confirm"}); response.Code != http.StatusForbidden {
		t.Fatal("Team confirmation bypassed Administrator guard", response.Body.String())
	}
}
