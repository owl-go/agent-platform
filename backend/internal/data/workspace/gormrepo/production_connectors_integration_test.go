package gormrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestProductionDefaultConnectorsVerifyWithoutLegacyDefinitions(t *testing.T) {
	r, owner := defaultResourceRepositoryFixture(t)
	ctx := t.Context()
	image := "example/runtime@sha256:" + strings.Repeat("b", 64)
	digest := "sha256:" + strings.Repeat("b", 64)
	if err := r.ConfigureConnectorExecution("production", "runsc", []string{image, image}); err != nil {
		t.Fatal(err)
	}
	if err := r.ConfigureConnectorExecution("production", "runsc", []string{digest}); err == nil {
		t.Fatal("production accepted a local image ID")
	}
	objects := memory.New()
	store, _ := cliconnector.NewArtifactStore(objects)
	bundle := []byte("immutable test bundle with injected process seam")
	sum := sha256.Sum256(bundle)
	hash := hex.EncodeToString(sum[:])
	key := "cli-connectors/fixture/" + hash + ".tgz"
	if err := store.PutImmutable(ctx, key, bundle, hash); err != nil {
		t.Fatal(err)
	}
	policy := []byte(fmt.Sprintf(`{"cli":{"executable":"tool","runtime":{"digest":"sha256:%s"}},"cli_bundle_sha256":"%s","cli_bundle_object_key":"%s"}`, strings.Repeat("a", 64), hash, key))
	revision, err := r.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "fixture", Version: "1.0.0", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("c", 64), ObjectKey: "connectors/fixture.zip", RuntimePolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("INSERT INTO connector_package_publications(package_source,active_revision_id,state,administrator_user_id,version,local_activation_pending) VALUES(?,?,'disabled',?,1,true)", "fixture", revision.ID, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("INSERT INTO default_resource_seeds(kind,resource_key,resource_id,catalog_version,content_sha256,managed) VALUES('connector',?,?,'1.0.0',?,true)", "fixture", revision.ID, strings.Repeat("c", 64)).Error; err != nil {
		t.Fatal(err)
	}
	suite := &localConnectorSuite{}
	worked, err := r.VerifyNextProductionConnector(ctx, store, suite)
	if err != nil || !worked || suite.calls != 1 {
		t.Fatalf("verification: %v %v calls=%d", worked, err, suite.calls)
	}
	verified, err := r.HasConnectorBundleRuntimeConformance(ctx, hash, digest)
	if err != nil || !verified {
		t.Fatalf("missing exact production evidence: %v %v", verified, err)
	}
	if verified, _ := r.HasConnectorBundleRuntimeConformance(ctx, hash, "sha256:"+strings.Repeat("d", 64)); verified {
		t.Fatal("accepted another image")
	}
	publications, err := r.ListConnectorPublications(ctx, false)
	if err != nil || len(publications) != 1 {
		t.Fatalf("publication not restored: %v %v", publications, err)
	}
	publication := publications[0]
	if _, err := r.SetConnectorPublicationState(ctx, owner, "fixture", domain.ConnectorPublicationDisabled, publication.Version); err != nil {
		t.Fatal(err)
	}
	if worked, err := r.VerifyNextProductionConnector(ctx, store, suite); err != nil || worked {
		t.Fatalf("explicit disable overridden: %v %v", worked, err)
	}
	var count int64
	if err := r.db.Table("local_cli_connector_conformance").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("production wrote local evidence")
	}
	if err := r.db.Exec("DELETE FROM production_cli_connector_conformance WHERE bundle_sha256=?", hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("INSERT INTO local_cli_connector_conformance(bundle_sha256,runtime_image_id,worker_architecture,passed) VALUES(?,?,?,true)", hash, digest, runtime.GOARCH).Error; err != nil {
		t.Fatal(err)
	}
	if verified, err := r.HasConnectorBundleRuntimeConformance(ctx, hash, digest); err != nil || verified {
		t.Fatal("local runc evidence accepted in production")
	}
	suite.failure = errors.New("fixture process rejected")
	if worked, err := r.VerifyNextProductionConnector(ctx, store, suite); err == nil || !worked {
		t.Fatal("failed startup recorded as passed")
	}
	if verified, err := r.HasConnectorBundleRuntimeConformance(ctx, hash, digest); err != nil || verified {
		t.Fatal("failed production evidence accepted")
	}
	calls := suite.calls
	if worked, err := r.VerifyNextProductionConnector(ctx, store, suite); err != nil || worked || suite.calls != calls {
		t.Fatal("failure cooldown ignored")
	}
}
