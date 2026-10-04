package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

type renewalFixtureCipher struct{}

func (renewalFixtureCipher) Encrypt(raw []byte, _ string) ([]byte, error) {
	return append([]byte(nil), raw...), nil
}
func (renewalFixtureCipher) Decrypt(raw []byte, _ string) ([]byte, error) {
	return append([]byte(nil), raw...), nil
}

func TestChannelExpiredFeishuAuthorizationRenewsBeforeAdmission(t *testing.T) {
	f := newChannelFixture(t)
	ctx := context.Background()
	f.enable(t)
	policy := `{"auth_mode":"oauth","cli_bundle_object_key":"bundle.zip","cli_bundle_sha256":"` + strings.Repeat("b", 64) + `","cli":{"authentication_driver":"feishu","executable":"lark-cli","runtime":{"digest":"sha256:` + strings.Repeat("c", 64) + `"},"capabilities":[{"id":"read","argv_prefix":["read"],"risk":"low","identities":["user"],"timeout_seconds":30}]}}`
	revision, err := f.repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "1", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "package.zip", RuntimePolicy: []byte(policy)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.PublishConnectorRevision(ctx, f.owner, revision.ID, 0); err != nil {
		t.Fatal(err)
	}
	installation, err := f.repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: f.owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	auth, err := f.repo.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: f.owner, InstallationID: installation.ID, IdentityRef: "user", ExternalIdentityID: "account", Scopes: []string{"read"}, CredentialCiphertext: []byte(`{"access_token":"old","refresh_token":"refresh"}`), CredentialAAD: "grant-aad", CredentialFormat: "json", ExpiresAt: &expiry})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.Exec(`INSERT INTO connector_provider_applications(owner_user_id,installation_id,provider_application_id_ciphertext,provider_application_secret_ciphertext,provider_name,developer_console_url) VALUES(?,?,?::bytea,?::bytea,'Feishu','https://open.feishu.cn')`, f.owner, installation.ID, []byte("app"), []byte("secret")).Error; err != nil {
		t.Fatal(err)
	}
	// Retained initiating Snapshot is an actual channel Run with a CLI binding.
	f.receive(t, "first", "alice", "first")
	f.admit(t)
	var root runRecord
	if err = f.db.Where("workflow_id=?", f.workflow).Take(&root).Error; err != nil {
		t.Fatal(err)
	}
	var snap domain.ExecutionSnapshot
	if json.Unmarshal(root.WorkflowSnapshot, &snap) != nil {
		t.Fatal("snapshot")
	}
	connector, err := connectorCLIServerSnapshot(f.db, f.owner, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	snap.Stages[0].CLIConnectors = []domain.CLIConnectorSnapshot{connector}
	encoded, _ := json.Marshal(snap)
	if err = f.db.Model(&runRecord{}).Where("id=?", root.ID).Updates(map[string]any{"workflow_snapshot": encoded, "state": "succeeded"}).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.db.Exec("UPDATE connector_authorizations SET expires_at=now()-interval '1 second' WHERE id=?", auth.ID).Error; err != nil {
		t.Fatal(err)
	}
	if validateQueuedSnapshotAvailability(f.db, snap, f.owner) == nil {
		t.Fatal("expired grant was accepted")
	}
	calls := 0
	renewal := application.NewConnectorAuthorizationRenewal(f.repo, renewalFixtureCipher{}, func(ctx context.Context, app, secret, token string) (application.ConnectorRenewalGrant, error) {
		calls++
		if app != "app" || secret != "secret" || token != "refresh" {
			t.Fatal("credential context")
		}
		return application.ConnectorRenewalGrant{ExternalID: "account", AccessToken: "new", RefreshToken: "rotated", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}, nil)
	f.app.EnableAuthorizationRenewal(renewal)
	f.receive(t, "followup", "alice", "follow-up")
	f.admit(t)
	var inbox channelInboxRecord
	f.db.Where("event_id='followup'").Take(&inbox)
	if inbox.State != "admitted" || inbox.RunID == nil || calls != 1 {
		t.Fatalf("not admitted: %s %s calls=%d", inbox.State, inbox.Reason, calls)
	}
	var turn runRecord
	f.db.Where("id=?", *inbox.RunID).Take(&turn)
	if turn.ConversationID != root.ID || turn.TurnNumber != 2 {
		t.Fatal("conversation continuity lost")
	}
	var saved connectorAuthorizationRecord
	f.db.Where("id=?", auth.ID).Take(&saved)
	if saved.Version != auth.Version+1 || !saved.ExpiresAt.After(time.Now()) {
		t.Fatal("renewal not committed")
	}
	var audits int64
	f.db.Table("connector_package_audit_records").Where("installation_id=? AND operation='refresh_authorization'", installation.ID).Count(&audits)
	if audits != 1 {
		t.Fatal("audit missing")
	}
	// Two independent repository instances share the same cross-process lock.
	release, locked, err := f.repo.LockConnectorAuthorizationRefresh(ctx, f.owner, installation.ID, auth.ID)
	if err != nil || !locked {
		t.Fatal("refresh lock unavailable")
	}
	_, second, err := New(f.db, nil).LockConnectorAuthorizationRefresh(ctx, f.owner, installation.ID, auth.ID)
	if err != nil || second {
		t.Fatal("overlapping refresh allowed")
	}
	release()
	release()
	release, locked, err = f.repo.LockConnectorAuthorizationRefresh(ctx, f.owner, installation.ID, auth.ID)
	if err != nil || !locked {
		t.Fatal("refresh lock leaked")
	}
	release()
	// A provider response arriving after disconnect must not resurrect a grant.
	grant := connectorAuthorizationDomain(saved)
	if err = f.db.Model(&connectorAuthorizationRecord{}).Where("id=?", auth.ID).Update("state", "disconnected").Error; err != nil {
		t.Fatal(err)
	}
	if err = f.repo.SaveRenewedFeishuAuthorization(ctx, grant, grant.Version); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked grant revived: %v", err)
	}
	if items, err := f.repo.ListDueFeishuAuthorizations(ctx, f.owner, time.Now().Add(24*time.Hour)); err != nil || len(items) != 0 {
		t.Fatal("disconnected grant selected")
	}
}

func TestRenewableFeishuAuthorizationEligibility(t *testing.T) {
	f := newChannelFixture(t)
	ctx := context.Background()
	revision, err := f.repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "1", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "package.zip", RuntimePolicy: []byte(`{"auth_mode":"oauth","cli":{"authentication_driver":"feishu"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.PublishConnectorRevision(ctx, f.owner, revision.ID, 0); err != nil {
		t.Fatal(err)
	}
	inst, err := f.repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: f.owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Minute)
	grant, err := f.repo.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: f.owner, InstallationID: inst.ID, IdentityRef: "user", ExternalIdentityID: "account", CredentialCiphertext: []byte(`{"refresh_token":"r"}`), CredentialAAD: "aad", CredentialFormat: "json", ExpiresAt: &expiry})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "2", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("b", 64), ObjectKey: "other.zip", RuntimePolicy: []byte(`{"auth_mode":"none","cli":{"authentication_driver":"none"}}`)})
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name, query string
		args        []any
	}{
		{"owner disabled", "UPDATE users SET disabled_at=now() WHERE id=?", []any{f.owner}},
		{"installation disabled", "UPDATE connector_installations SET state='disabled' WHERE id=?", []any{inst.ID}},
		{"selection removed", "UPDATE connector_installations SET authorization_id=NULL WHERE id=?", []any{inst.ID}},
		{"grant disconnected", "UPDATE connector_authorizations SET state='disconnected' WHERE id=?", []any{grant.ID}},
		{"publication disabled", "UPDATE connector_package_publications SET state='disabled' WHERE package_source='feishu'", nil},
		{"wrong driver", "UPDATE connector_installations SET active_revision_id=? WHERE id=?", []any{other.ID, inst.ID}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			rollback := errors.New("rollback fixture")
			err = f.db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec(scenario.query, scenario.args...).Error; err != nil {
					t.Fatal(err)
				}
				repo := New(tx, nil)
				items, err := repo.ListDueFeishuAuthorizations(ctx, f.owner, time.Now().Add(5*time.Minute))
				if err != nil || len(items) != 0 {
					t.Fatalf("ineligible candidate selected: %v", err)
				}
				if _, err = repo.GetRenewableFeishuAuthorization(ctx, f.owner, inst.ID, grant.ID); !errors.Is(err, domain.ErrNotFound) {
					t.Fatal("ineligible grant reloaded")
				}
				updated := grant
				future := time.Now().Add(time.Hour)
				updated.ExpiresAt = &future
				if err = repo.SaveRenewedFeishuAuthorization(ctx, updated, grant.Version); !errors.Is(err, domain.ErrNotFound) {
					t.Fatal("ineligible grant saved")
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
		})
	}
	if items, err := f.repo.ListDueFeishuAuthorizations(ctx, uuid.NewString(), time.Now().Add(5*time.Minute)); err != nil || len(items) > 0 {
		t.Fatal("owner isolation lost")
	}
}
