package gormrepo

import (
	"context"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestOnlyAdministratorCanMaintainTeamsAndOrdinaryUserCanReadPlatformTeams(t *testing.T) {
	db := conversationTestDatabase(t)
	admin, user := uuid.NewString(), uuid.NewString()
	for _, owner := range []string{admin, user} {
		if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner, owner == admin).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := New(db, nil)
	ctx := context.Background()
	expert, err := repository.CreateExpert(ctx, admin, domain.ExpertInput{Name: "Reviewer", Introduction: "Display", CoreCapability: "Review", OperatingProcedure: "Inspect", OutputStandard: "Report"})
	if err != nil {
		t.Fatal(err)
	}
	input := domain.ExpertTeamInput{LeadMemberID: "first", Name: "Review Team", Introduction: "Display", CoreCapability: "Review", Members: []domain.ExpertTeamMemberInput{{ID: "first", Name: "First", ExpertID: expert.ID}, {ID: "second", Name: "Second", ExpertID: expert.ID}}}
	if _, err := repository.CreateExpertTeam(ctx, user, input); err == nil {
		t.Fatal("ordinary User created a Team")
	}
	missingLead := input
	missingLead.LeadMemberID = ""
	if _, err := repository.CreateExpertTeam(ctx, admin, missingLead); err == nil {
		t.Fatal("Team without an explicit lead was created")
	}
	input.LeadMemberID = "first"
	team, err := repository.CreateExpertTeam(ctx, admin, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetExpertTeam(ctx, user, team.ID); err != nil {
		t.Fatalf("platform Team is invisible: %v", err)
	}
	items, err := repository.ListExpertTeams(ctx, user)
	if err != nil || len(items) != 1 {
		t.Fatalf("platform list=%v err=%v", items, err)
	}
	if _, err := repository.UpdateExpertTeam(ctx, user, team.ID, input, team.Version); err == nil {
		t.Fatal("ordinary User changed a Team")
	}
	if err := repository.DeleteExpertTeam(ctx, user, team.ID); err == nil {
		t.Fatal("ordinary User deleted a Team")
	}
	if _, err := repository.UpdateExpert(ctx, admin, expert.ID, domain.ExpertInput{Name: "Reviewer", Introduction: "Changed display", Guidance: "# Changed source"}, expert.Version); err != nil {
		t.Fatal(err)
	}
	frozen, err := repository.GetExpertTeam(ctx, user, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.Members) != 2 || frozen.Members[0].Expert.Guidance != "# Core Capability\n\nReview\n\n# Operating Procedure\n\nInspect\n\n# Output Standard\n\nReport" {
		t.Fatalf("Team member followed source edit: %+v", frozen.Members)
	}
	if err := repository.DeleteExpert(ctx, admin, expert.ID); err != nil {
		t.Fatalf("copied Team prevents source deletion: %v", err)
	}
	if _, err := repository.GetExpertTeam(ctx, user, team.ID); err != nil {
		t.Fatalf("Team lost after source deletion: %v", err)
	}
	session, err := repository.CreateSession(ctx, user, nil, &team.ID)
	if err != nil {
		t.Fatalf("ordinary User cannot select the Platform Team: %v", err)
	}
	selection, err := repository.GetConversationSelection(ctx, user, domain.ConversationScope{SessionID: session.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Defaults) != 2 || selection.TeamProfile == nil || selection.TeamProfile.LeadMemberID != "first" || selection.Defaults[0].Expert.Guidance != "# Core Capability\n\nReview\n\n# Operating Procedure\n\nInspect\n\n# Output Standard\n\nReport" {
		t.Fatalf("selection lost owned definition: %+v", selection)
	}
}

func TestNewTeamSessionFreezesCoordinationWhileRetryRetainsIt(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	repo := New(db, nil)
	admin, user, connection, model := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, owner := range []string{admin, user} {
		exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner, owner == admin)
	}
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','test')`, connection, user)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'test')`, connection)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model','Model')`, model, connection)
	exec(`INSERT INTO personal_settings(user_id,default_runtime_engine,runtime_model_defaults) VALUES(?,'codex',jsonb_build_object('codex',?::text))`, user, model)
	exec(`UPDATE personal_settings SET team_credit_budget_hundredths=150 WHERE user_id=?`, user)
	definition := domain.ExpertInput{Name: "Role", Introduction: "Review", Guidance: "# Role"}
	team, err := repo.CreateExpertTeam(ctx, admin, domain.ExpertTeamInput{Name: "Team", Introduction: "Review", CoreCapability: "Synthesize", LeadMemberID: "lead", Members: []domain.ExpertTeamMemberInput{{ID: "lead", Name: "Lead", Definition: &definition}, {ID: "reviewer", Name: "Reviewer", Definition: &definition}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := repo.CreateSession(ctx, user, nil, &team.ID)
	if err != nil {
		t.Fatal(err)
	}
	inputMessage, message, err := repo.CreateMessagePair(ctx, user, session.ID, "Review this", nil)
	if err != nil {
		t.Fatal(err)
	}
	if message.ResponseSnapshot == nil || message.ResponseSnapshot.SchemaVersion != 3 || message.ResponseSnapshot.Coordination == nil || message.ResponseSnapshot.Coordination.LeadMemberID != "lead" {
		t.Fatalf("new response used historical order: %+v", message.ResponseSnapshot)
	}
	if message.ResponseSnapshot.Coordination.CreditBudgetHundredths != 150 {
		t.Fatal("new response lost User admission budget")
	}
	exec(`UPDATE personal_settings SET team_credit_budget_hundredths=300 WHERE user_id=?`, user)
	exec(`UPDATE session_messages SET state='failed' WHERE id=?`, message.ID)
	_, retry, err := repo.RetryMessage(ctx, user, session.ID, inputMessage.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ResponseSnapshot == nil || retry.ResponseSnapshot.SchemaVersion != 3 || retry.ResponseSnapshot.Coordination == nil || retry.ResponseSnapshot.Coordination.LeadMemberID != "lead" {
		t.Fatal("retry lost frozen strategy")
	}
	if retry.ResponseSnapshot.Coordination.CreditBudgetHundredths != 150 {
		t.Fatal("manual retry rewrote frozen admission budget")
	}
	job, err := repo.ClaimNext(ctx)
	if err != nil || job == nil || job.Snapshot.SchemaVersion != 3 || job.Snapshot.Coordination == nil {
		t.Fatalf("Worker did not hydrate coordinated snapshot: %+v err=%v", job, err)
	}
}

func TestPlatformTeamDoesNotCopyAdministratorConnectorCredentialsIntoUserExecution(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	admin, server := uuid.NewString(), uuid.NewString()
	if err := fixture.db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,true)`, admin, admin, admin, admin+"@example.test", admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Exec(`INSERT INTO mcp_servers(id,owner_user_id,name,transport,configuration,secret_ciphertext,tested_at) VALUES(?,?,'Private tool','streamable_http','{"url":"https://example.test/mcp"}'::jsonb,'author-secret',now())`, server, admin).Error; err != nil {
		t.Fatal(err)
	}
	definition := domain.ExpertInput{Name: "Role", Introduction: "Display", Guidance: "# Role", MCPServerIDs: []string{server}}
	team, err := fixture.repository.CreateExpertTeam(context.Background(), admin, domain.ExpertTeamInput{Name: "Team", Introduction: "Display", CoreCapability: "Review", LeadMemberID: "lead", Members: []domain.ExpertTeamMemberInput{{ID: "lead", Name: "Lead", Definition: &definition}, {ID: "reviewer", Name: "Reviewer", Definition: &definition}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.CreateSession(context.Background(), fixture.ownerID, nil, &team.ID); err == nil {
		t.Fatal("ordinary User inherited the author’s Connector credential")
	}
}
