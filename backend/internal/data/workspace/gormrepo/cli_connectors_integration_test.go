package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestCLIConnectorInstallAndReinstallWithDerivedAuthentication(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	input := cliconnector.Definition{Name: "Feishu CLI", Icon: "terminal", Description: "Read Feishu documents", InstallationType: "npm", Package: "@larksuite/cli", Version: "1.0.93"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatalf("install simplified CLI Definition: %v", err)
	}
	if item.AuthenticationDriver != "" {
		t.Fatal("draft must leave authentication unresolved for the package builder")
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET authentication_driver='feishu', state='disabled' WHERE id=?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, item.VersionNumber)
	if err != nil {
		t.Fatalf("reinstall simplified CLI Definition: %v", err)
	}
	if updated.State != cliconnector.StateDraft || updated.AuthenticationDriver != "" {
		t.Fatalf("reinstall did not reset derived metadata: %#v", updated)
	}
	if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, updated.VersionNumber); err != nil {
		t.Fatalf("queue CLI installation: %v", err)
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID).Error; err == nil {
		t.Fatal("unresolved authentication must never become available")
	}
}

func TestCLIConnectorReinstallCanRecordRepeatedConformance(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	input := cliconnector.Definition{Name: "Example CLI", Icon: "terminal", Description: "Read examples", InstallationType: "npm", Package: "example-cli", Version: "1.0.0"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	result := cliconnector.BuildResult{State: cliconnector.StateAvailable, BundleObjectKey: "cli-connectors/bundles/test.zip", BundleSHA256: strings.Repeat("a", 64), RuntimeDigests: []string{"sha256:" + strings.Repeat("b", 64)}, Package: input.Package, Version: input.Version, Integrity: "sha512-test", Executable: "example", AuthenticationDriver: "none", SupportedArchitectures: []string{"linux-amd64"}, ManifestVersion: "1", UsageGuide: "Use read to retrieve an example.", Capabilities: []cliconnector.Capability{{ID: "read", DisplayName: map[string]string{"en": "Read example"}, OperationPhrase: map[string]string{"en": "read an example"}, ArgvPrefix: []string{"read"}, Risk: cliconnector.RiskLow, Identities: []cliconnector.Identity{cliconnector.IdentityUser}, EgressHosts: []string{"example.test"}, Timeout: time.Minute, Idempotency: cliconnector.IdempotencyRetrySafe}}}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, item.VersionNumber); err != nil {
			t.Fatal(err)
		}
		var job *application.ExecutionJob
		if err := db.Transaction(func(tx *gorm.DB) error { var err error; job, err = claimCLIConnectorBuild(tx); return err }); err != nil {
			t.Fatal(err)
		}
		if job == nil {
			t.Fatal("build was not queued")
		}
		if err := repository.FinishCLIConnectorBuild(ctx, *job, result, ""); err != nil {
			t.Fatalf("installation %d could not finish: %v", attempt+1, err)
		}
		items, err := repository.ListCLIConnectorDefinitions(ctx, true)
		if err != nil || len(items) != 1 || items[0].State != cliconnector.StateReview {
			t.Fatalf("installation %d review state: items=%#v err=%v", attempt+1, items, err)
		}
		if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, items[0].VersionNumber); err != nil {
			t.Fatalf("installation %d review publish: %v", attempt+1, err)
		}
		verified, err := repository.HasCLIConnectorRuntimeConformance(ctx, item.ID, result.BundleSHA256, result.RuntimeDigests[0])
		if err != nil || !verified {
			t.Fatalf("installation %d exact Runtime conformance: verified=%v err=%v", attempt+1, verified, err)
		}
		verified, err = repository.HasCLIConnectorRuntimeConformance(ctx, item.ID, strings.Repeat("c", 64), result.RuntimeDigests[0])
		if err != nil || verified {
			t.Fatalf("installation %d mismatched bundle conformance: verified=%v err=%v", attempt+1, verified, err)
		}
		available, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			item, err = repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, available.VersionNumber)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCLIConnectorExecutionRevalidationKeepsFrozenVersionsUntilSecurityDisable(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	input := cliconnector.Definition{Name: "Example CLI", Icon: "terminal", Description: "Read examples", InstallationType: "npm", Package: "example-cli", Version: "1.0.0", ManifestVersion: "1"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO cli_connector_enablements(owner_user_id,definition_id,state) VALUES(?,?,'enabled')", owner, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	available, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.RevalidateCLIConnectorExecution(ctx, owner, item.ID, available.VersionNumber); err != nil {
		t.Fatalf("current frozen version was rejected: %v", err)
	}
	updated, err := repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, available.VersionNumber)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.RevalidateCLIConnectorExecution(ctx, owner, item.ID, available.VersionNumber); err != nil {
		t.Fatalf("normal version evolution rejected an active frozen version: %v", err)
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.DisableCLIConnectorDefinition(ctx, item.ID, updated.VersionNumber); err != nil {
		t.Fatal(err)
	}
	if err := repository.RevalidateCLIConnectorExecution(ctx, owner, item.ID, available.VersionNumber); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("security disable did not reject frozen execution: %v", err)
	}
}

func TestCLIConnectorDeletionRevokesAccessAndPreservesHistory(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, expert, enablement, session := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner)
	input := cliconnector.Definition{Name: "Feishu CLI", Icon: "terminal", Description: "Read Feishu", InstallationType: "npm", Package: "@larksuite/cli", Version: "1.0.93", AuthenticationDriver: "feishu"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID)
	exec("INSERT INTO cli_connector_enablements(id,owner_user_id,definition_id,state,registration_device_code_ciphertext) VALUES(?,?,?,'enabled','test-device')", enablement, owner, item.ID)
	exec("INSERT INTO cli_connector_authorizations(owner_user_id,enablement_id,identity,external_identity_id,external_display_name,token_ciphertext,refresh_token_ciphertext,state) VALUES(?,?,'user','test-user','Test','test-token','test-refresh','active')", owner, enablement)
	exec("INSERT INTO cli_connector_authorization_attempts(owner_user_id,enablement_id,identity,device_code_ciphertext,action_url,expires_at) VALUES(?,?,'user','test-device','https://example.test',now()+interval '5 minutes')", owner, enablement)
	exec("INSERT INTO feishu_cli_applications(owner_user_id,enablement_id,provider_application_id_ciphertext,provider_application_secret_ciphertext,provider_name,developer_console_url) VALUES(?,?,'test-app','test-secret','Test','https://example.test')", owner, enablement)
	exec("INSERT INTO experts(id,owner_user_id,name,cli_connector_definition_ids) VALUES(?,?,'Test',jsonb_build_array(?::text))", expert, owner, item.ID)
	exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", session, owner)
	snapshot := `{"stages":[{"cli_connectors":[{"id":"` + item.ID + `"}]}]}`
	exec("INSERT INTO session_messages(session_id,role,content,state,response_snapshot) VALUES(?,'assistant','Historical result','completed',?::jsonb)", session, snapshot)
	if err := repository.DeleteCLIConnectorDefinition(ctx, item.ID, item.VersionNumber+1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale deletion = %v", err)
	}
	if _, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID); err != nil {
		t.Fatal("stale deletion revoked access")
	}
	if err := repository.DeleteCLIConnectorDefinition(ctx, item.ID, item.VersionNumber); err != nil {
		t.Fatal(err)
	}
	for _, unpublished := range []bool{false, true} {
		items, err := repository.ListCLIConnectorDefinitions(ctx, unpublished)
		if err != nil || len(items) != 0 {
			t.Fatalf("deleted definition remains in catalog: %v", err)
		}
	}
	if _, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted connector is available: %v", err)
	}
	if _, err := repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, item.VersionNumber+1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("deleted connector can be edited: %v", err)
	}
	if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, item.VersionNumber+1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted connector can be published: %v", err)
	}
	if values, err := repository.ListCLIConnectorEnablements(ctx, owner); err != nil || len(values) != 0 {
		t.Fatalf("deleted enablement visible: %v", err)
	}
	if values, err := repository.ListCLIConnectorHealth(ctx, time.Now()); err != nil || len(values) != 0 {
		t.Fatalf("deleted health visible: %v", err)
	}
	var count int64
	checks := []string{
		"SELECT count(*) FROM cli_connector_enablements WHERE state <> 'disabled' OR registration_device_code_ciphertext IS NOT NULL",
		"SELECT count(*) FROM cli_connector_authorizations WHERE state <> 'disconnected' OR token_ciphertext IS NOT NULL OR refresh_token_ciphertext IS NOT NULL",
		"SELECT count(*) FROM cli_connector_authorization_attempts",
		"SELECT count(*) FROM experts WHERE cli_connector_definition_ids <> '[]'::jsonb OR version <> 2",
	}
	for _, query := range checks {
		if err := db.Raw(query).Scan(&count).Error; err != nil || count != 0 {
			t.Fatalf("deletion cleanup failed for %s: count=%d err=%v", query, count, err)
		}
	}
	if err := db.Raw("SELECT count(*) FROM session_messages WHERE response_snapshot=?::jsonb", snapshot).Scan(&count).Error; err != nil || count != 1 {
		t.Fatal("historical snapshot changed")
	}
	if err := db.Raw("SELECT count(*) FROM feishu_cli_applications").Scan(&count).Error; err != nil || count != 1 {
		t.Fatal("User's Feishu Application was removed")
	}
	replacement, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatalf("cannot install the same package after deletion: %v", err)
	}
	exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", replacement.ID)
	restored, err := repository.EnableCLIConnector(ctx, owner, replacement.ID)
	if err != nil || restored.State != "enabled" || restored.ProviderName != "" {
		t.Fatalf("replacement Connector was not enabled independently: %#v, %v", restored, err)
	}
	if _, err := repository.GetFeishuCLIApplicationCredentials(ctx, owner, restored.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("replacement Connector reused another Connector's credentials: %v", err)
	}
	again, err := repository.EnableCLIConnector(ctx, owner, replacement.ID)
	if err != nil || again.ID != restored.ID || again.Version != restored.Version {
		t.Fatalf("enablement is not idempotent: %v", err)
	}
	if _, err := repository.EnableCLIConnector(ctx, uuid.NewString(), replacement.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another User reused the application: %v", err)
	}
}
