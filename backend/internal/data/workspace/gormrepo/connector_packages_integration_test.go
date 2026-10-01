package gormrepo

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestConnectorInstallationCanBeReinstalledAfterUninstall(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "notion", Version: "0.23.13", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"cli"}`), ObjectKey: "connectors/notion/package.zip"})
	if err != nil {
		t.Fatal(err)
	}
	install := func() (domain.ConnectorInstallation, error) {
		return repository.InstallConnectorWithAudit(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "notion", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive}, domain.ConnectorAuditRecord{OwnerID: owner, RevisionID: revision.ID, Operation: "install_published", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	}
	first, err := install()
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: first.ID, IdentityRef: "user", CredentialCiphertext: []byte("encrypted-test-token"), CredentialAAD: "test-aad", CredentialFormat: "json"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := repository.ListConnectorInstallations(ctx, owner)
	if err != nil || len(current) != 1 || current[0].AuthorizationID != authorization.ID {
		t.Fatalf("authorized installation = %#v, %v", current, err)
	}
	uninstalled, err := repository.SetConnectorInstallationStateWithAudit(ctx, owner, first.ID, domain.ConnectorInstallationUninstalled, current[0].Version, domain.ConnectorAuditRecord{OwnerID: owner, InstallationID: first.ID, Operation: "uninstall", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil || uninstalled.State != domain.ConnectorInstallationUninstalled {
		t.Fatalf("uninstall = %#v, %v", uninstalled, err)
	}
	reinstalled, err := install()
	if err != nil {
		t.Fatalf("reinstall after uninstall: %v", err)
	}
	if reinstalled.ID != first.ID || reinstalled.State != domain.ConnectorInstallationActive || reinstalled.Version != uninstalled.Version+1 || reinstalled.AuthorizationID != "" {
		t.Fatalf("reinstalled installation = %#v", reinstalled)
	}
	installations, err := repository.ListConnectorInstallations(ctx, owner)
	if err != nil || len(installations) != 1 || installations[0].Authorized {
		t.Fatalf("reinstalled catalog entry = %#v, %v", installations, err)
	}
	var stored connectorAuthorizationRecord
	if err := db.Where("id = ?", authorization.ID).Take(&stored).Error; err != nil || stored.State != string(domain.ConnectorAuthorizationDisconnected) || len(stored.CredentialCiphertext) != 0 {
		t.Fatalf("previous authorization was reused: state=%q credential_length=%d error=%v", stored.State, len(stored.CredentialCiphertext), err)
	}
	var installAudits int64
	if err := db.Table("connector_package_audit_records").Where("installation_id = ? AND operation = ?", first.ID, "install_published").Count(&installAudits).Error; err != nil || installAudits != 2 {
		t.Fatalf("reinstall audit count = %d, %v", installAudits, err)
	}
}

func TestConnectorPublicationAndMultipleAuthorizationSelection(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	administrator, owner := uuid.NewString(), uuid.NewString()
	for _, id := range []string{administrator, owner} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	definition := uuid.NewString()
	if err := db.Exec("INSERT INTO cli_connector_definitions(id,name,description,installation_type,npm_package,npm_version,npm_integrity,executable,state,authentication_driver,created_by_user_id,bundle_sha256) VALUES(?,?,?,'npm','@larksuite/cli','1.0.93','sha512-test','lark-cli','available','feishu',?,?)", definition, "Verified Feishu", "Test", administrator, strings.Repeat("b", 64)).Error; err != nil {
		t.Fatal(err)
	}
	runtimeDigest := "sha256:" + strings.Repeat("c", 64)
	if err := db.Exec("INSERT INTO cli_connector_conformance(definition_id,bundle_sha256,runtime_repo_digest,tested_at,passed) VALUES(?,?,?,now(),true)", definition, strings.Repeat("b", 64), runtimeDigest).Error; err != nil {
		t.Fatal(err)
	}
	verified, err := repository.HasConnectorBundleRuntimeConformance(ctx, strings.Repeat("b", 64), runtimeDigest)
	if err != nil || !verified {
		t.Fatalf("exact Conformance evidence = %v, %v", verified, err)
	}
	verified, err = repository.HasConnectorBundleRuntimeConformance(ctx, strings.Repeat("d", 64), runtimeDigest)
	if err != nil || verified {
		t.Fatalf("mismatched bundle Conformance evidence = %v, %v", verified, err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{
		PackageSource: "feishu", Version: "1.0.0", Mode: domain.ConnectorModeCLI,
		PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"oauth"}`), ObjectKey: "connectors/feishu/1.0.0/package.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	publication, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if publication.State != domain.ConnectorPublicationAvailable || publication.Version != 1 {
		t.Fatalf("unexpected publication: %#v", publication)
	}
	if _, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale publication update must conflict: %v", err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	firstExpiry := time.Now().UTC().Add(time.Hour)
	first, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", ExternalIdentityID: "account-a", ExternalDisplayName: "Account A", Scopes: []string{"contact:read"}, CredentialCiphertext: []byte("cipher-a"), CredentialAAD: "aad-a", CredentialFormat: "json", ExpiresAt: &firstExpiry})
	if err != nil {
		t.Fatal(err)
	}
	installation.Version++
	second, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", ExternalIdentityID: "account-b", ExternalDisplayName: "Account B", Scopes: []string{"im:write"}, CredentialCiphertext: []byte("cipher-b"), CredentialAAD: "aad-b", CredentialFormat: "json", ExpiresAt: &firstExpiry})
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListConnectorAuthorizations(ctx, owner, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].State != domain.ConnectorAuthorizationActive || items[1].State != domain.ConnectorAuthorizationActive {
		t.Fatalf("multiple active authorizations were not preserved: %#v", items)
	}
	health, err := repository.ListConnectorPublicationHealth(ctx)
	if err != nil || len(health) != 1 || health[0].InstallationCount != 1 || health[0].ActiveInstallationCount != 1 || health[0].ActiveAuthorizationCount != 2 {
		t.Fatalf("Connector publication health = %#v, %v", health, err)
	}
	installation.Version++
	selected, err := repository.SelectConnectorAuthorization(ctx, owner, installation.ID, first.ID, installation.Version)
	if err != nil {
		t.Fatal(err)
	}
	if selected.AuthorizationID != first.ID {
		t.Fatalf("selected authorization mismatch: %#v", selected)
	}
	refreshedExpiry := time.Now().UTC().Add(2 * time.Hour)
	first.CredentialCiphertext = []byte("cipher-a-refreshed")
	first.ExpiresAt = &refreshedExpiry
	refreshed, err := repository.RefreshConnectorAuthorization(ctx, first, first.Version, domain.ConnectorAuditRecord{OwnerID: owner, InstallationID: installation.ID, Operation: "refresh_authorization", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil || refreshed.Version != first.Version+1 || string(refreshed.CredentialCiphertext) != "cipher-a-refreshed" || refreshed.ExternalDisplayName != "Account A" {
		t.Fatalf("refreshed authorization = %#v, %v", refreshed, err)
	}
	// Providers may return opaque grants without an external account identifier.
	// Refresh remains bound to owner, installation, authorization and version.
	refreshed.ExternalIdentityID = ""
	refreshed.RefreshCredentialCiphertext = []byte("platform-only-refresh")
	refreshed.RefreshCredentialAAD = "refresh-aad"
	opaque, err := repository.RefreshConnectorAuthorization(ctx, refreshed, refreshed.Version, domain.ConnectorAuditRecord{OwnerID: owner, InstallationID: installation.ID, Operation: "refresh_authorization", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil || string(opaque.RefreshCredentialCiphertext) != "platform-only-refresh" || opaque.RefreshCredentialAAD != "refresh-aad" {
		t.Fatalf("opaque grant refresh lost platform-only material: %v", err)
	}
	if _, err := repository.RefreshConnectorAuthorization(ctx, first, first.Version, domain.ConnectorAuditRecord{OwnerID: owner, InstallationID: installation.ID, Operation: "refresh_authorization", Outcome: "succeeded", CreatedAt: time.Now().UTC()}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale authorization refresh must conflict: %v", err)
	}
	if _, err := repository.SelectConnectorAuthorization(ctx, owner, installation.ID, second.ID, installation.Version); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale authorization selection must conflict: %v", err)
	}
}

func TestConnectorInstallationFeishuSetupAndAuthorizationFlows(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	administrator, owner := uuid.NewString(), uuid.NewString()
	for _, id := range []string{administrator, owner} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "1.0.93", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"oauth","cli":{"authentication_driver":"feishu"}}`), ObjectKey: "connectors/feishu/package.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0); err != nil {
		t.Fatal(err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	setup, err := repository.BeginConnectorSetup(ctx, domain.ConnectorSetup{OwnerID: owner, InstallationID: installation.ID, ActionURL: "https://open.feishu.cn/setup", ExpiresAt: &expires, DeviceCodeCiphertext: []byte("registration")})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := repository.CompleteConnectorSetup(ctx, owner, setup.ID, domain.ConnectorProviderApplication{AppIDCiphertext: []byte("app"), AppSecretCiphertext: []byte("secret"), ProviderName: "Test Feishu", DeveloperConsoleURL: "https://open.feishu.cn/app/test"})
	if err != nil || completed.State != "completed" {
		t.Fatalf("complete setup = %#v, %v", completed, err)
	}
	application, err := repository.GetConnectorProviderApplication(ctx, owner, installation.ID)
	if err != nil || application.ProviderName != "Test Feishu" {
		t.Fatalf("application = %#v, %v", application, err)
	}
	flow, err := repository.BeginConnectorAuthorizationFlow(ctx, domain.ConnectorAuthorizationAttempt{OwnerID: owner, InstallationID: installation.ID, Identity: "user", Scopes: []string{"im:message"}, ActionURL: "https://open.feishu.cn/authorize", ExpiresAt: expires, DeviceCodeCiphertext: []byte("device")})
	if err != nil {
		t.Fatal(err)
	}
	read, err := repository.GetConnectorAuthorizationFlow(ctx, owner, flow.ID)
	if err != nil || len(read.Scopes) != 1 || read.Scopes[0] != "im:message" {
		t.Fatalf("authorization flow = %#v, %v", read, err)
	}
	if err := repository.UpdateConnectorAuthorizationFlow(ctx, uuid.NewString(), flow.ID, []byte("device"), []byte("code"), flow.ActionURL); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-owner callback update: %v", err)
	}
	if err := repository.UpdateConnectorAuthorizationFlow(ctx, owner, flow.ID, []byte("device"), []byte("code"), flow.ActionURL); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateConnectorAuthorizationFlow(ctx, owner, flow.ID, []byte("device"), []byte("replay"), flow.ActionURL); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale callback update: %v", err)
	}
	if err := repository.ConsumeConnectorAuthorizationFlow(ctx, uuid.NewString(), flow.ID, []byte("code")); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-owner exchange: %v", err)
	}
	if err := repository.ConsumeConnectorAuthorizationFlow(ctx, owner, flow.ID, []byte("code")); err != nil {
		t.Fatal(err)
	}
	if err := repository.ConsumeConnectorAuthorizationFlow(ctx, owner, flow.ID, []byte("code")); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate exchange: %v", err)
	}
	if err := repository.DeleteConnectorAuthorizationFlow(ctx, owner, flow.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetConnectorAuthorizationFlow(ctx, owner, flow.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted flow error = %v", err)
	}
	first, err := repository.BeginConnectorAuthorizationFlow(ctx, domain.ConnectorAuthorizationAttempt{OwnerID: owner, InstallationID: installation.ID, Identity: "user", ActionURL: "https://account.teambition.com/oauth2/mcp/authorize", ExpiresAt: expires, DeviceCodeCiphertext: []byte("first")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.BeginConnectorAuthorizationFlow(ctx, domain.ConnectorAuthorizationAttempt{OwnerID: owner, InstallationID: installation.ID, Identity: "user", ActionURL: "https://account.teambition.com/oauth2/mcp/authorize", ExpiresAt: expires, DeviceCodeCiphertext: []byte("second")})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateConnectorAuthorizationFlow(ctx, owner, first.ID, []byte("first"), []byte("code"), first.ActionURL); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("superseded callback: %v", err)
	}
	if err := db.Model(&connectorAuthorizationFlowRecord{}).Where("id = ?", second.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateConnectorAuthorizationFlow(ctx, owner, second.ID, []byte("second"), []byte("code"), second.ActionURL); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expired callback: %v", err)
	}
	if err := repository.ConsumeConnectorAuthorizationFlow(ctx, owner, second.ID, []byte("second")); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expired exchange: %v", err)
	}
}

func TestFeishuPublicationProjectsLegacyUsersAfterMigration(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	administrator, owner := uuid.NewString(), uuid.NewString()
	for _, id := range []string{administrator, owner} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	definition, enablement, expert := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO cli_connector_definitions(id,name,description,installation_type,npm_package,npm_version,npm_integrity,executable,state,authentication_driver,created_by_user_id) VALUES(?,?,?,'npm','@larksuite/cli','1.0.93','sha512-test','lark','available','feishu',?)", definition, "Legacy Feishu", "Test", administrator).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO cli_connector_enablements(id,owner_user_id,definition_id,state) VALUES(?,?,?,'enabled')", enablement, owner, definition).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO feishu_cli_applications(owner_user_id,enablement_id,provider_application_id_ciphertext,provider_application_secret_ciphertext,provider_name,developer_console_url) VALUES(?,?,'app','secret','Legacy App','https://open.feishu.cn/app/test')", owner, enablement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO cli_connector_authorizations(owner_user_id,enablement_id,identity,external_identity_id,external_display_name,scopes,token_ciphertext,refresh_token_ciphertext,state,expires_at) VALUES(?,?,'user','ou_legacy','Legacy User','[\"im:message\"]','token','refresh','active',now()+interval '1 hour')", owner, enablement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO experts(id,owner_user_id,name,cli_connector_definition_ids) VALUES(?,?,'Legacy Expert',jsonb_build_array(?::text))", expert, owner, definition).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "1.0.93", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"oauth","cli":{"authentication_driver":"feishu"},"cli_bundle_object_key":"bundle","cli_bundle_sha256":"` + strings.Repeat("b", 64) + `"}`), ObjectKey: "connectors/feishu/package.zip"})
	if err != nil {
		t.Fatal(err)
	}
	publication, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var installationID, selectedAuthorizationID string
	if err := db.Raw("SELECT id,authorization_id FROM connector_installations WHERE owner_user_id=? AND package_source='feishu'", owner).Row().Scan(&installationID, &selectedAuthorizationID); err != nil {
		t.Fatal(err)
	}
	var externalID, refreshAAD, providerName string
	if err := db.Raw("SELECT external_identity_id,refresh_credential_aad FROM connector_authorizations WHERE id=?", selectedAuthorizationID).Row().Scan(&externalID, &refreshAAD); err != nil || externalID != "ou_legacy" || !strings.HasSuffix(refreshAAD, ":refresh") {
		t.Fatalf("projected authorization = %q, %q, %v", externalID, refreshAAD, err)
	}
	if err := db.Raw("SELECT provider_name FROM connector_provider_applications WHERE installation_id=?", installationID).Scan(&providerName).Error; err != nil || providerName != "Legacy App" {
		t.Fatalf("projected provider application = %q, %v", providerName, err)
	}
	// A legacy application can still be the source of truth when the new
	// provider-application projection is absent. Runtime materialization must
	// use the same owner-scoped fallback as the authorization flow.
	if err := db.Exec("DELETE FROM connector_provider_applications WHERE installation_id=?", installationID).Error; err != nil {
		t.Fatal(err)
	}
	material, err := repository.ResolveConnectorPackageAuthorization(ctx, owner, installationID, revision.ID, selectedAuthorizationID, "user")
	if err != nil || string(material.AppIDCiphertext) != "app" || string(material.AppSecretCiphertext) != "secret" || !slices.Contains(material.Scopes, "im:message") {
		t.Fatalf("legacy application was not materialized with the selected authorization: app=%t secret=%t err=%v", string(material.AppIDCiphertext) == "app", string(material.AppSecretCiphertext) == "secret", err)
	}
	var connectorIDs string
	if err := db.Raw("SELECT cli_connector_definition_ids FROM experts WHERE id=?", expert).Scan(&connectorIDs).Error; err != nil || !strings.Contains(string(connectorIDs), installationID) || strings.Contains(string(connectorIDs), definition) {
		t.Fatalf("rewritten Expert connectors = %s, %v", connectorIDs, err)
	}
	if _, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, publication.Version); err != nil {
		t.Fatal(err)
	}
	var installations, authorizations int64
	if err := db.Model(&connectorInstallationRecord{}).Where("owner_user_id=? AND package_source='feishu'", owner).Count(&installations).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&connectorAuthorizationRecord{}).Where("owner_user_id=? AND installation_id=?", owner, installationID).Count(&authorizations).Error; err != nil {
		t.Fatal(err)
	}
	if installations != 1 || authorizations != 1 {
		t.Fatalf("projection is not idempotent: installations=%d authorizations=%d", installations, authorizations)
	}
}

func TestLinearMCPSnapshotUsesAuthorizationAADAndExcludesRefreshMaterial(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "linear", Version: "1.0.0", Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/linear/package.zip", RuntimePolicy: []byte(`{"auth_mode":"oauth","metadata":{"source":"linear"},"mcp":{"transport":"streamable_http","url":"https://mcp.linear.app/mcp","egress_hosts":["mcp.linear.app"],"timeout_seconds":60}}`)})
	if err != nil {
		t.Fatal(err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "linear", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	aad := "connector-authorization:" + owner + ":" + installation.ID + ":"
	authorization, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", CredentialCiphertext: []byte("encrypted-access"), CredentialAAD: aad, CredentialFormat: "json", RefreshCredentialCiphertext: []byte("platform-only-refresh"), RefreshCredentialAAD: aad + ":refresh", ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := connectorMCPServerSnapshot(db, owner, installation.ID)
	if err != nil || snapshot.SecretAAD != aad || string(snapshot.SecretCiphertext) != "encrypted-access" || strings.Contains(string(snapshot.Configuration), "refresh") {
		t.Fatalf("MCP snapshot failed to preserve authorization boundary: %v", err)
	}
	if _, err := connectorMCPServerSnapshot(db, uuid.NewString(), installation.ID); err == nil {
		t.Fatal("another owner read authorization")
	}
	if err := db.Model(&connectorAuthorizationRecord{}).Where("id = ?", authorization.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := connectorMCPServerSnapshot(db, owner, installation.ID); err == nil {
		t.Fatal("expired grant was materialized")
	}
}

func TestPixsoMCPSnapshotUsesAuthorizationAADAndExcludesRefreshMaterial(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "pixso", Version: "1.0.0", Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/pixso/package.zip", RuntimePolicy: []byte(`{"auth_mode":"oauth","metadata":{"source":"pixso"},"mcp":{"transport":"streamable_http","url":"https://pixso.net/mcp","egress_hosts":["pixso.net"],"timeout_seconds":60}}`)})
	if err != nil {
		t.Fatal(err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "pixso", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	aad := "connector-authorization:" + owner + ":" + installation.ID + ":"
	authorization, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", CredentialCiphertext: []byte("encrypted-access"), CredentialAAD: aad, CredentialFormat: "json", RefreshCredentialCiphertext: []byte("platform-only-refresh"), RefreshCredentialAAD: aad + ":refresh", ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := connectorMCPServerSnapshot(db, owner, installation.ID)
	if err != nil || snapshot.SecretAAD != aad || string(snapshot.SecretCiphertext) != "encrypted-access" || strings.Contains(string(snapshot.Configuration), "refresh") {
		t.Fatalf("MCP snapshot failed to preserve authorization boundary: %v", err)
	}
	if _, err := connectorMCPServerSnapshot(db, uuid.NewString(), installation.ID); err == nil {
		t.Fatal("another owner read authorization")
	}
	if err := db.Model(&connectorAuthorizationRecord{}).Where("id = ?", authorization.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := connectorMCPServerSnapshot(db, owner, installation.ID); err == nil {
		t.Fatal("expired grant was materialized")
	}
}
