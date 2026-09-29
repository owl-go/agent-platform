package gormrepo

import (
	"context"
	"fmt"

	"agent-platform/backend/internal/cliconnector"
)

// ListCLIReverificationTargets returns only active managed CLI installations
// that lack passed evidence for the candidate Runtime. Multiple Users of the
// same immutable bundle share one test.
func (repository *Repository) ListCLIReverificationTargets(ctx context.Context, runtimeDigest string) ([]cliconnector.ReverificationTarget, error) {
	var targets []cliconnector.ReverificationTarget
	err := repository.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (revision.runtime_policy->>'cli_bundle_sha256')
			COALESCE((SELECT definition.id::text FROM cli_connector_definitions definition
				WHERE definition.bundle_sha256 = revision.runtime_policy->>'cli_bundle_sha256'
				ORDER BY definition.id LIMIT 1), '') AS definition_id,
			revision.runtime_policy->>'cli_bundle_object_key' AS bundle_object_key,
			revision.runtime_policy->>'cli_bundle_sha256' AS bundle_sha256,
			revision.runtime_policy->'cli'->>'executable' AS executable
		FROM connector_installations installation
		JOIN connector_revisions revision ON revision.id = installation.active_revision_id
		WHERE installation.state = 'active' AND revision.mode = 'cli'
			AND NOT EXISTS (
				SELECT 1 FROM cli_connector_conformance evidence
				WHERE evidence.bundle_sha256 = revision.runtime_policy->>'cli_bundle_sha256'
					AND evidence.runtime_repo_digest = ? AND evidence.passed
			)
		ORDER BY revision.runtime_policy->>'cli_bundle_sha256', revision.created_at DESC`, runtimeDigest).Scan(&targets).Error
	if err != nil {
		return nil, fmt.Errorf("list active CLI Connector Conformance gaps: %w", err)
	}
	return targets, nil
}

func (repository *Repository) RecordCLIReverification(ctx context.Context, definitionID, bundleSHA256, runtimeDigest string) error {
	result := repository.db.WithContext(ctx).Exec(`
		INSERT INTO cli_connector_conformance
			(definition_id, bundle_sha256, runtime_repo_digest, environment, tested_at, passed)
		SELECT id, ?, ?, '{"source":"runtime_reverification"}'::jsonb, now(), true
		FROM cli_connector_definitions WHERE id = ? AND bundle_sha256 = ?
		ON CONFLICT (definition_id, bundle_sha256, runtime_repo_digest)
		DO UPDATE SET environment = EXCLUDED.environment, tested_at = EXCLUDED.tested_at, passed = true`,
		bundleSHA256, runtimeDigest, definitionID, bundleSHA256)
	if result.Error != nil {
		return fmt.Errorf("save exact CLI Connector Conformance: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("CLI Connector Conformance definition no longer matches bundle")
	}
	return nil
}
