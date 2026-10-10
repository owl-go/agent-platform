package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"testing"
)

func TestExpertDependenciesPreserveCanceledAndStorageErrors(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	dependencies := []domain.ExpertConnectorDependency{{Source: "tools", Kind: "mcp", Version: "1.0.0"}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := resolveExpertDependencies(fixture.db.WithContext(ctx), fixture.ownerID, dependencies)
	if !errors.Is(err, context.Canceled) || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("cancellation was replaced: %v", err)
	}
	_, _, err = resolveExpertDependencies(fixture.db, fixture.ownerID, dependencies)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("missing installation was not a domain error: %v", err)
	}
	storageFailure := errors.New("dependency storage unavailable")
	if err := fixture.db.Callback().Query().Before("gorm:query").Register("expert_dependency_storage_failure", func(tx *gorm.DB) { tx.AddError(storageFailure) }); err != nil {
		t.Fatal(err)
	}
	defer fixture.db.Callback().Query().Remove("expert_dependency_storage_failure")
	_, _, err = resolveExpertDependencies(fixture.db, fixture.ownerID, dependencies)
	if !errors.Is(err, storageFailure) || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("storage cause was replaced: %v", err)
	}
}

func TestExpertConnectorAuthorizationLookupPreservesStorageCause(t *testing.T) {
	for _, kind := range []domain.ConnectorMode{domain.ConnectorModeMCP, domain.ConnectorModeCLI} {
		t.Run(string(kind), func(t *testing.T) {
			fixture := newWorkerRecoveryFixture(t)
			policy := []byte(`{"auth_mode":"oauth","mcp":{"transport":"streamable_http","url":"https://example.test/mcp","timeout_seconds":30,"egress_hosts":["example.test"]}}`)
			if kind == domain.ConnectorModeCLI {
				policy = []byte(`{"auth_mode":"oauth","cli":{"executable":"tool"}}`)
			}
			revision, err := fixture.repository.CreateConnectorRevision(t.Context(), domain.ConnectorRevision{PackageSource: "tools", Version: "1.0.0", Mode: kind, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/tools/package.zip", RuntimePolicy: policy})
			if err != nil {
				t.Fatal(err)
			}
			installation, err := fixture.repository.InstallConnector(t.Context(), domain.ConnectorInstallation{OwnerID: fixture.ownerID, PackageSource: "tools", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
			if err != nil {
				t.Fatal(err)
			}
			authorizationID := uuid.NewString()
			if err := fixture.db.Exec("INSERT INTO connector_authorizations(id,installation_id,owner_user_id,identity_ref,credential_ciphertext,state) VALUES(?,?,?,'identity',?,'active')", authorizationID, installation.ID, fixture.ownerID, []byte("protected")).Error; err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.Exec("UPDATE connector_installations SET authorization_id=? WHERE id=?", authorizationID, installation.ID).Error; err != nil {
				t.Fatal(err)
			}
			storageFailure := errors.New("authorization storage unavailable")
			if err := fixture.db.Callback().Query().Before("gorm:query").Register("expert_authorization_storage_failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "connector_authorizations" {
					tx.AddError(storageFailure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			defer fixture.db.Callback().Query().Remove("expert_authorization_storage_failure")
			_, _, err = resolveExpertDependencies(fixture.db, fixture.ownerID, []domain.ExpertConnectorDependency{{Source: "tools", Kind: string(kind), Version: "1.0.0"}})
			if !errors.Is(err, storageFailure) || errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("authorization storage cause was replaced: %v", err)
			}
		})
	}
}

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
