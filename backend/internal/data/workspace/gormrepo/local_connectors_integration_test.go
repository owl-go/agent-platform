package gormrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore/memory"
)

type localConnectorSuite struct {
	failure error
	calls   int
}

func TestLocalConnectorMigrationOnlyOptsInUntouchedDefaults(t *testing.T) {
	r, owner := defaultResourceRepositoryFixture(t)
	if err := r.db.Exec("ALTER TABLE connector_package_publications DROP COLUMN local_activation_pending; DROP TABLE local_cli_connector_conformance; DELETE FROM schema_migrations WHERE name IN ('000088_local_connector_conformance.sql','000089_local_connector_architecture.sql')").Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		source  string
		version int
		managed bool
	}{{"feishu", 1, true}, {"dingtalk", 2, true}, {"private", 1, false}} {
		revision, err := r.CreateConnectorRevision(t.Context(), domain.ConnectorRevision{PackageSource: tc.source, Version: "1.0.0", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/fixture.zip", RuntimePolicy: []byte(`{"cli":{"executable":"tool"}}`)})
		if err != nil {
			t.Fatal(err)
		}
		if err := r.db.Exec("INSERT INTO connector_package_publications(package_source,active_revision_id,state,administrator_user_id,version) VALUES(?,?,'disabled',?,?)", tc.source, revision.ID, owner, tc.version).Error; err != nil {
			t.Fatal(err)
		}
		if tc.managed {
			if err := r.db.Exec("INSERT INTO default_resource_seeds(kind,resource_key,resource_id,catalog_version,content_sha256,managed) VALUES('connector',?,?,'1.0.0',?,true)", tc.source, revision.ID, strings.Repeat("a", 64)).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for range 2 {
		if err := gormdb.Migrate(t.Context(), r.db); err != nil {
			t.Fatal(err)
		}
	}
	var rows []connectorPublicationRecord
	if err := r.db.Order("package_source").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.LocalActivationPending != (row.PackageSource == "feishu") {
			t.Fatalf("incorrect migration opt-in: %s = %v", row.PackageSource, row.LocalActivationPending)
		}
	}
}

func (s *localConnectorSuite) Test(context.Context, []byte, string, cliconnector.Definition) error {
	s.calls++
	return s.failure
}

func TestLocalConnectorEvidenceIsIsolatedAndPreservesAdministratorDisable(t *testing.T) {
	repo, owner := defaultResourceRepositoryFixture(t)
	ctx := t.Context()
	image := "sha256:" + strings.Repeat("b", 64)
	if err := repo.ConfigureConnectorExecution("local", "runc", []string{image, image}); err != nil {
		t.Fatal(err)
	}
	objects := memory.New()
	store, _ := cliconnector.NewArtifactStore(objects)
	bundle := []byte("immutable fixture: conformance is injected at the actual process seam")
	sum := sha256.Sum256(bundle)
	hash := hex.EncodeToString(sum[:])
	key := "cli-connectors/" + hash + "/bundle.tgz"
	if err := store.PutImmutable(ctx, key, bundle, hash); err != nil {
		t.Fatal(err)
	}
	policy := []byte(fmt.Sprintf(`{"cli":{"executable":"tool","runtime":{"digest":"sha256:%s"}},"cli_bundle_sha256":"%s","cli_bundle_object_key":"%s"}`, strings.Repeat("a", 64), hash, key))
	revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "feishu", Version: "1.0.0", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("c", 64), ObjectKey: "connectors/fixture.zip", RuntimePolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	// A seed revision upgrade can increment version before first verification.
	if err := repo.db.Exec("INSERT INTO connector_package_publications(package_source,active_revision_id,state,administrator_user_id,version,local_activation_pending) VALUES('feishu',?,'disabled',?,2,true)", revision.ID, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec("INSERT INTO default_resource_seeds(kind,resource_key,resource_id,catalog_version,content_sha256,managed) VALUES('connector','feishu',?,'1.0.0',?,true)", revision.ID, strings.Repeat("c", 64)).Error; err != nil {
		t.Fatal(err)
	}
	suite := &localConnectorSuite{failure: errors.New("wrong architecture")}
	if _, err := repo.VerifyNextLocalConnector(ctx, store, suite); err == nil {
		t.Fatal("failed executable was published")
	}
	publications, _ := repo.ListConnectorPublications(ctx, false)
	if len(publications) != 0 {
		t.Fatal("failed executable visible")
	}
	if err := repo.db.Exec("DELETE FROM local_cli_connector_conformance").Error; err != nil {
		t.Fatal(err)
	}
	suite.failure = nil
	if worked, err := repo.VerifyNextLocalConnector(ctx, store, suite); err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	if ok, err := repo.HasLocalConnectorConformance(ctx, hash, image); err != nil || !ok {
		t.Fatalf("local evidence %v %v", ok, err)
	}
	other := "amd64"
	if runtime.GOARCH == other {
		other = "arm64"
	}
	if err := repo.db.Exec("UPDATE local_cli_connector_conformance SET worker_architecture=?", other).Error; err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.HasLocalConnectorConformance(ctx, hash, image); err != nil || ok {
		t.Fatalf("other architecture evidence trusted: %v %v", ok, err)
	}
	if err := repo.db.Exec("UPDATE local_cli_connector_conformance SET worker_architecture=?", runtime.GOARCH).Error; err != nil {
		t.Fatal(err)
	}
	// Even the same SHA-256 can never satisfy production verification.
	production := New(repo.db, nil)
	if ok, err := production.HasConnectorBundleRuntimeConformance(ctx, hash, image); err != nil || ok {
		t.Fatalf("local evidence promoted to production: %v %v", ok, err)
	}
	publications, _ = repo.ListConnectorPublications(ctx, false)
	if len(publications) != 1 {
		t.Fatal("verified default was not restored")
	}
	if _, err := repo.SetConnectorPublicationState(ctx, owner, "feishu", domain.ConnectorPublicationDisabled, publications[0].Version); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec("DELETE FROM local_cli_connector_conformance").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.VerifyNextLocalConnector(ctx, store, suite); err != nil {
		t.Fatal(err)
	}
	publications, _ = repo.ListConnectorPublications(ctx, false)
	if len(publications) != 0 {
		t.Fatal("administrator disable was overwritten")
	}
	if err := repo.ConfigureConnectorExecution("local", "runc", []string{"sha256:" + strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.HasLocalConnectorConformance(ctx, hash, image); ok {
		t.Fatal("old image evidence reused")
	}
	if err := repo.ConfigureConnectorExecution("local", "runc", nil); err != nil || !repo.LocalConnectorVerification() || len(repo.ConnectorVerificationDigests(image)) != 0 {
		t.Fatalf("missing local runtime must not fall back to production: %v", err)
	}
}
