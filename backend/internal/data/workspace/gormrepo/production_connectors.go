package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"agent-platform/backend/internal/cliconnector"
	"gorm.io/gorm"
)

// VerifyNextProductionConnector verifies only immutable managed starter bundles.
// User credentials, private revisions and explicitly disabled publications are
// never changed. The historical pending-column name is retained for migrations.
func (r *Repository) VerifyNextProductionConnector(ctx context.Context, store *cliconnector.ArtifactStore, suite cliconnector.Conformance) (bool, error) {
	if r.localConnectorProfile || len(r.productionConnectorDigests) == 0 {
		return false, fmt.Errorf("production Connector verification is not configured")
	}
	var rows []connectorRevisionRecord
	if err := r.db.WithContext(ctx).Table("connector_revisions revision").Select("revision.*").Joins("JOIN connector_package_publications p ON p.active_revision_id=revision.id").Joins("JOIN default_resource_seeds seed ON seed.kind='connector' AND seed.resource_key=p.package_source AND seed.resource_id=revision.id::text AND seed.managed=true").Where("revision.mode='cli'").Order("revision.package_source").Find(&rows).Error; err != nil {
		return false, err
	}
	for _, row := range rows {
		var policy connectorCLIPolicy
		if err := json.Unmarshal(row.RuntimePolicy, &policy); err != nil {
			return false, err
		}
		if policy.CLI == nil || policy.BundleSHA256 == "" || policy.BundleObjectKey == "" {
			continue
		}
		if changed, err := r.activateProductionConnector(ctx, row, policy.BundleSHA256); err != nil || changed {
			return changed, err
		}
		for _, digest := range r.productionConnectorDigests {
			var count int64
			if err := r.db.WithContext(ctx).Table("production_cli_connector_conformance").Where("bundle_sha256=? AND runtime_repo_digest=? AND worker_architecture=? AND (passed=true OR verified_at>?)", policy.BundleSHA256, digest, runtime.GOARCH, time.Now().Add(-time.Minute)).Count(&count).Error; err != nil {
				return false, err
			}
			if count > 0 {
				continue
			}
			bundle, err := store.GetVerified(ctx, policy.BundleObjectKey, policy.BundleSHA256)
			if err == nil {
				err = suite.Test(ctx, bundle, digest, cliconnector.Definition{Executable: policy.CLI.Executable, CPUMillis: policy.CLI.Limits.CPU, MemoryMiB: policy.CLI.Limits.MemoryMiB, ChildProcesses: policy.CLI.Limits.ChildProcesses})
			}
			if ctx.Err() != nil {
				return true, ctx.Err()
			}
			if saveErr := r.db.WithContext(ctx).Exec("INSERT INTO production_cli_connector_conformance(bundle_sha256,runtime_repo_digest,worker_architecture,passed) VALUES(?,?,?,?) ON CONFLICT(bundle_sha256,runtime_repo_digest,worker_architecture) DO UPDATE SET passed=EXCLUDED.passed,verified_at=now()", policy.BundleSHA256, digest, runtime.GOARCH, err == nil).Error; saveErr != nil {
				return true, saveErr
			}
			if err != nil {
				return true, fmt.Errorf("production Connector %s executable verification failed: %w", row.PackageSource, err)
			}
			_, err = r.activateProductionConnector(ctx, row, policy.BundleSHA256)
			return true, err
		}
	}
	return false, nil
}

func (r *Repository) activateProductionConnector(ctx context.Context, row connectorRevisionRecord, bundle string) (bool, error) {
	for _, digest := range r.productionConnectorDigests {
		verified, err := r.HasConnectorBundleRuntimeConformance(ctx, bundle, digest)
		if err != nil || !verified {
			return false, err
		}
	}
	result := r.db.WithContext(ctx).Table("connector_package_publications").Where("package_source=? AND active_revision_id=? AND state='disabled' AND local_activation_pending=true", row.PackageSource, row.ID).Updates(map[string]any{"state": "available", "local_activation_pending": false, "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()})
	return result.RowsAffected > 0, result.Error
}
