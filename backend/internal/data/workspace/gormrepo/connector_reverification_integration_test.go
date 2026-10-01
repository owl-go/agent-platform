package gormrepo

import (
	"context"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestActiveConnectorBundleRequiresCandidateRuntimeConformance(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner, definitionID := uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	bundleSHA := strings.Repeat("b", 64)
	if err := db.Exec("INSERT INTO cli_connector_definitions(id,name,description,installation_type,npm_package,npm_version,npm_integrity,executable,state,authentication_driver,created_by_user_id,bundle_sha256) VALUES(?,?,?,'npm','@larksuite/cli','1.0.93','sha512-test','lark-cli','disabled','feishu',?,?)", definitionID, "Verified CLI", "Test", owner, bundleSHA).Error; err != nil {
		t.Fatal(err)
	}
	oldDigest, newDigest := "sha256:"+strings.Repeat("c", 64), "sha256:"+strings.Repeat("d", 64)
	if err := db.Exec("INSERT INTO cli_connector_conformance(definition_id,bundle_sha256,runtime_repo_digest,tested_at,passed) VALUES(?,?,?,now(),true)", definitionID, bundleSHA, oldDigest).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{
		PackageSource: "feishu", Version: "1.0.93", Mode: domain.ConnectorModeCLI,
		PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"oauth","cli_bundle_object_key":"cli-connectors/verified/bundle.tgz","cli_bundle_sha256":"` + bundleSHA + `","cli":{"executable":"lark-cli","resource_limits":{"cpu_millis":500,"memory_mib":256,"child_processes":16}}}`),
		ObjectKey: "connectors/feishu/package.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive}); err != nil {
		t.Fatal(err)
	}
	targets, err := repository.ListCLIReverificationTargets(ctx, newDigest)
	if err != nil || len(targets) != 1 || targets[0].DefinitionID != definitionID || targets[0].BundleSHA256 != bundleSHA || targets[0].Executable != "lark-cli" || targets[0].CPUMillis != 500 || targets[0].MemoryMiB != 256 || targets[0].ChildProcesses != 16 {
		t.Fatalf("missing exact candidate target: %#v, %v", targets, err)
	}
	if err := repository.RecordCLIReverification(ctx, definitionID, bundleSHA, newDigest); err != nil {
		t.Fatal(err)
	}
	if targets, err = repository.ListCLIReverificationTargets(ctx, newDigest); err != nil || len(targets) != 0 {
		t.Fatalf("recorded evidence did not close gap: %#v, %v", targets, err)
	}
	if err := repository.RecordCLIReverification(ctx, definitionID, strings.Repeat("e", 64), newDigest); err == nil {
		t.Fatal("evidence for a mismatched bundle must be rejected")
	}
}
