package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

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
	if err := repository.DeleteConnectorAuthorizationFlow(ctx, owner, flow.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetConnectorAuthorizationFlow(ctx, owner, flow.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted flow error = %v", err)
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
	if err != nil || string(material.AppIDCiphertext) != "app" || string(material.AppSecretCiphertext) != "secret" {
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
