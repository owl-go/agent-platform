-- Backfill legacy resources into immutable package projections without copying
-- plaintext credentials or changing the legacy snapshot columns. The marker is
-- intentional: these rows preserve identity for migration and audit, while
-- runtime readers continue to use the already verified legacy snapshot until a
-- real Connector Package revision replaces it.

WITH legacy_mcp AS (
    SELECT
        m.id,
        m.owner_user_id,
        'legacy-mcp-' || m.id::text AS package_source,
        repeat(md5(m.id::text || ':mcp'), 2) AS package_sha256,
        jsonb_build_object(
            'auth_mode', 'none',
            'legacy_projection', true,
            'mcp', m.configuration
        ) AS runtime_policy,
        'connectors/legacy/mcp/' || m.id::text || '/1.0.0.zip' AS object_key,
        COALESCE(m.updated_at, m.created_at, now()) AS created_at
    FROM mcp_servers m
), inserted_mcp_revisions AS (
    INSERT INTO connector_revisions (id, package_source, version, mode, package_sha256, runtime_policy, object_key, created_at)
    SELECT gen_random_uuid(), package_source, '1.0.0', 'mcp', package_sha256, runtime_policy, object_key, created_at
    FROM legacy_mcp
    ON CONFLICT (package_source, version, package_sha256) DO NOTHING
    RETURNING id, package_source
)
INSERT INTO connector_installations (id, owner_user_id, package_source, active_revision_id, state, version, updated_at)
SELECT gen_random_uuid(), legacy.owner_user_id, legacy.package_source, revision.id,
       CASE WHEN m.tested_at IS NOT NULL AND m.test_error IS NULL THEN 'active' ELSE 'disabled' END,
       GREATEST(m.version, 1), legacy.created_at
FROM legacy_mcp legacy
JOIN mcp_servers m ON m.id = legacy.id
JOIN connector_revisions revision ON revision.package_source = legacy.package_source AND revision.version = '1.0.0' AND revision.package_sha256 = legacy.package_sha256
WHERE NOT EXISTS (
    SELECT 1 FROM connector_installations existing
    WHERE existing.owner_user_id = legacy.owner_user_id AND existing.package_source = legacy.package_source
);

WITH legacy_cli AS (
    SELECT
        d.id,
        e.owner_user_id,
        'legacy-cli-' || d.id::text AS package_source,
        repeat(md5(d.id::text || ':' || COALESCE(d.bundle_sha256, 'unbuilt')), 2) AS package_sha256,
        jsonb_build_object(
            'auth_mode', CASE WHEN d.authentication_driver = 'none' THEN 'none' ELSE 'cli' END,
            'legacy_projection', true,
            'cli', jsonb_build_object(
                'runtime', jsonb_build_object('kind', 'node', 'version', d.npm_version, 'digest', COALESCE((SELECT c.runtime_repo_digest FROM cli_connector_conformance c WHERE c.definition_id = d.id AND c.bundle_sha256 = d.bundle_sha256 AND c.passed ORDER BY c.tested_at DESC LIMIT 1), 'sha256:' || repeat('0', 64))),
                'executable', d.executable,
                'capabilities', d.capabilities
            ),
            'cli_bundle_object_key', d.bundle_object_key,
            'cli_bundle_sha256', d.bundle_sha256
        ) AS runtime_policy,
        COALESCE(d.updated_at, d.created_at, now()) AS created_at
    FROM cli_connector_definitions d
    JOIN cli_connector_enablements e ON e.definition_id = d.id AND e.state = 'enabled'
    WHERE d.state = 'available'
), inserted_cli_revisions AS (
    INSERT INTO connector_revisions (id, package_source, version, mode, package_sha256, runtime_policy, object_key, created_at)
    SELECT gen_random_uuid(), package_source, '1.0.0', 'cli', package_sha256, runtime_policy,
           COALESCE(runtime_policy->>'cli_bundle_object_key', 'connectors/legacy/cli/' || id::text || '/bundle.tgz'), created_at
    FROM legacy_cli
    ON CONFLICT (package_source, version, package_sha256) DO NOTHING
    RETURNING id, package_source
)
INSERT INTO connector_installations (id, owner_user_id, package_source, active_revision_id, state, version, updated_at)
SELECT gen_random_uuid(), legacy.owner_user_id, legacy.package_source, revision.id, 'active', 1, legacy.created_at
FROM legacy_cli legacy
JOIN connector_revisions revision ON revision.package_source = legacy.package_source AND revision.version = '1.0.0' AND revision.package_sha256 = legacy.package_sha256
WHERE NOT EXISTS (
    SELECT 1 FROM connector_installations existing
    WHERE existing.owner_user_id = legacy.owner_user_id AND existing.package_source = legacy.package_source
);
