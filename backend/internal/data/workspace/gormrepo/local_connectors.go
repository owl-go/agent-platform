package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"time"

	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/sandbox"
	"gorm.io/gorm"
)

// ConfigureConnectorExecution is called once at construction, never per request.
// An empty profile is production and cannot opt in to the local evidence table.
func (r *Repository) ConfigureConnectorExecution(profile, runtime string, images []string) error {
	if err := sandbox.ValidateExecutionProfile(profile, runtime); err != nil {
		return err
	}
	if profile != sandbox.LocalProfile {
		r.localConnectorImages = nil
		r.localConnectorProfile = false
		return nil
	}
	for _, image := range images {
		if !sandbox.ValidExecutionImage(sandbox.LocalProfile, image) || len(image) != 71 {
			return fmt.Errorf("local Connector verification requires exact local Image IDs")
		}
	}
	r.localConnectorImages = slices.Clone(images)
	r.localConnectorProfile = true
	slices.Sort(r.localConnectorImages)
	r.localConnectorImages = slices.Compact(r.localConnectorImages)
	return nil
}

// ConnectorVerificationDigests maps the unchanged package's production runtime
// declaration to the explicitly configured local verification targets only.
func (r *Repository) ConnectorVerificationDigests(declared string) []string {
	if r.localConnectorProfile {
		return slices.Clone(r.localConnectorImages)
	}
	return []string{declared}
}

func (r *Repository) LocalConnectorVerification() bool { return r.localConnectorProfile }

func (r *Repository) HasLocalConnectorConformance(ctx context.Context, bundle, image string) (bool, error) {
	if !slices.Contains(r.localConnectorImages, image) {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Table("local_cli_connector_conformance").Where("bundle_sha256=? AND runtime_image_id=? AND worker_architecture=? AND passed=true", bundle, image, runtime.GOARCH).Count(&count).Error
	return count > 0, err
}

// VerifyNextLocalConnector tests one immutable built-in bundle without any User
// credentials or network. Failures remain unavailable and retry after a minute.
// Only untouched first-install publications are auto-activated; an explicit
// Administrator disable or a private package is never overwritten.
func (r *Repository) VerifyNextLocalConnector(ctx context.Context, store *cliconnector.ArtifactStore, suite cliconnector.Conformance) (bool, error) {
	if len(r.localConnectorImages) == 0 {
		return false, fmt.Errorf("local Connector verification is not configured")
	}
	var rows []connectorRevisionRecord
	if err := r.db.WithContext(ctx).Table("connector_revisions AS revision").Select("revision.*").
		Joins("JOIN connector_package_publications p ON p.active_revision_id=revision.id").
		Joins("JOIN default_resource_seeds seed ON seed.kind='connector' AND seed.resource_key=p.package_source AND seed.resource_id=revision.id::text AND seed.managed=true").
		Where("revision.mode='cli'").Order("CASE revision.package_source WHEN 'feishu' THEN 0 WHEN 'dingtalk' THEN 1 WHEN 'wecom' THEN 2 ELSE 3 END,revision.package_source").Find(&rows).Error; err != nil {
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
		if worked, err := r.activateLocalConnector(ctx, row, policy.BundleSHA256); err != nil || worked {
			return worked, err
		}
		for _, image := range r.localConnectorImages {
			var count int64
			if err := r.db.WithContext(ctx).Table("local_cli_connector_conformance").Where("bundle_sha256=? AND runtime_image_id=? AND worker_architecture=? AND (passed=true OR verified_at>?)", policy.BundleSHA256, image, runtime.GOARCH, time.Now().Add(-time.Minute)).Count(&count).Error; err != nil {
				return false, err
			}
			if count > 0 {
				continue
			}
			bundle, err := store.GetVerified(ctx, policy.BundleObjectKey, policy.BundleSHA256)
			if err == nil {
				err = suite.Test(ctx, bundle, image, cliconnector.Definition{Executable: policy.CLI.Executable, CPUMillis: policy.CLI.Limits.CPU, MemoryMiB: policy.CLI.Limits.MemoryMiB, ChildProcesses: policy.CLI.Limits.ChildProcesses})
			}
			if ctx.Err() != nil {
				return true, ctx.Err()
			}
			saveErr := r.db.WithContext(ctx).Exec("INSERT INTO local_cli_connector_conformance(bundle_sha256,runtime_image_id,worker_architecture,passed) VALUES(?,?,?,?) ON CONFLICT(bundle_sha256,runtime_image_id,worker_architecture) DO UPDATE SET passed=EXCLUDED.passed,verified_at=now()", policy.BundleSHA256, image, runtime.GOARCH, err == nil).Error
			if saveErr != nil {
				return true, saveErr
			}
			if err != nil {
				return true, fmt.Errorf("local Connector %s executable verification failed: %w", row.PackageSource, err)
			}
			_, err = r.activateLocalConnector(ctx, row, policy.BundleSHA256)
			return true, err
		}
	}
	return false, nil
}

func (r *Repository) activateLocalConnector(ctx context.Context, row connectorRevisionRecord, bundle string) (bool, error) {
	for _, image := range r.localConnectorImages {
		verified, err := r.HasLocalConnectorConformance(ctx, bundle, image)
		if err != nil || !verified {
			return false, err
		}
	}
	result := r.db.WithContext(ctx).Table("connector_package_publications").Where("package_source=? AND active_revision_id=? AND state='disabled' AND local_activation_pending=true", row.PackageSource, row.ID).Updates(map[string]any{"state": "available", "local_activation_pending": false, "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()})
	return result.RowsAffected > 0, result.Error
}
