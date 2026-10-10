package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestPlatformTeamConnectorDeclarationResolvesOnlyExecutingUsersAuthorization(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	repo := fixture.repository
	ctx := context.Background()
	admin := uuid.NewString()
	if err := fixture.db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,true)", admin, admin, admin, admin+"@example.test", admin).Error; err != nil {
		t.Fatal(err)
	}
	policy := []byte(`{"auth_mode":"oauth","mcp":{"transport":"streamable_http","url":"https://example.test/mcp","timeout_seconds":30,"egress_hosts":["example.test"]}}`)
	revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "team-tools", Version: "1.0.0", Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/team-tools/package.zip", RuntimePolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	authorInstallation, err := repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: admin, PackageSource: revision.PackageSource, ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	installAuthorization := func(owner, id, secret string) {
		t.Helper()
		authorizationID := uuid.NewString()
		if err := fixture.db.Exec("INSERT INTO connector_authorizations(id,installation_id,owner_user_id,identity_ref,credential_ciphertext,state) VALUES(?,?,?,'identity',?,'active')", authorizationID, id, owner, []byte(secret)).Error; err != nil {
			t.Fatal(err)
		}
		if err := fixture.db.Exec("UPDATE connector_installations SET authorization_id=? WHERE id=?", authorizationID, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	installAuthorization(admin, authorInstallation.ID, "author-account")
	role := domain.ExpertInput{Name: "Lead", Introduction: "Review", Guidance: "# Lead", MCPServerIDs: []string{authorInstallation.ID}}
	member := domain.ExpertInput{Name: "Member", Introduction: "Review", Guidance: "# Member"}
	team, err := repo.CreateExpertTeam(ctx, admin, domain.ExpertTeamInput{Name: "Team", Introduction: "Review", CoreCapability: "Review", LeadMemberID: "lead", Members: []domain.ExpertTeamMemberInput{{ID: "lead", Name: "Lead", Definition: &role}, {ID: "member", Name: "Member", Definition: &member}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(team.Members[0].Expert.ConnectorDependencies) != 1 || len(team.Members[0].Expert.MCPServerIDs) != 0 {
		t.Fatal("author installation was copied instead of a dependency")
	}
	if _, err := repo.CreateSession(ctx, fixture.ownerID, nil, &team.ID); err == nil {
		t.Fatal("unresolved external access was granted")
	}
	userInstallation, err := repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: fixture.ownerID, PackageSource: revision.PackageSource, ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	installAuthorization(fixture.ownerID, userInstallation.ID, "executing-account")
	session, err := repo.CreateSession(ctx, fixture.ownerID, nil, &team.ID)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := repo.GetConversationSelection(ctx, fixture.ownerID, domain.ConversationScope{SessionID: session.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Defaults) != 2 || len(selection.Defaults[0].MCPServers) != 1 || len(selection.Defaults[1].MCPServers) != 0 {
		t.Fatal("member permissions leaked across roles")
	}
	connector := selection.Defaults[0].MCPServers[0]
	if connector.SecretOwnerID != fixture.ownerID || string(connector.SecretCiphertext) != "executing-account" {
		t.Fatal("wrong account frozen")
	}
	stored, _ := json.Marshal(selection)
	if strings.Contains(string(stored), "author-account") || connector.ID == authorInstallation.ID {
		t.Fatal("author authorization leaked")
	}
	if err := fixture.db.Exec("UPDATE connector_authorizations SET state='disconnected' WHERE installation_id=?", userInstallation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateSession(ctx, fixture.ownerID, nil, &team.ID); err == nil {
		t.Fatal("disconnected authorization still accepted")
	}
}
