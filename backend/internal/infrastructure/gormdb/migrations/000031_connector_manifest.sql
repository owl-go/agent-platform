ALTER TABLE cli_connector_definitions
    ADD COLUMN manifest_version text NOT NULL DEFAULT 'legacy-v1',
    ADD COLUMN usage_guide text NOT NULL DEFAULT '';

ALTER TABLE cli_connector_definitions
    ADD CONSTRAINT cli_connector_manifest_version_nonempty CHECK (btrim(manifest_version) <> '');

ALTER TABLE cli_connector_definitions DROP CONSTRAINT cli_connector_definitions_state_check;
ALTER TABLE cli_connector_definitions ADD CONSTRAINT cli_connector_definitions_state_check
    CHECK (state IN ('draft', 'building', 'testing', 'review', 'available', 'failed', 'disabled'));

-- Legacy rows were published without a reviewed Resolved Manifest. Require an
-- explicit rebuild and review instead of treating parser support as evidence.
UPDATE cli_connector_definitions
SET state = 'draft',
    failure_reason = 'Connector Manifest review required',
    updated_at = now(),
    version = version + 1
WHERE state = 'available';

-- Setup and Authorization belong to one Connector. A provider account cannot
-- donate credentials to another CLI or MCP Connector.
ALTER TABLE feishu_cli_applications
    DROP CONSTRAINT feishu_cli_applications_owner_user_id_key;

WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY owner_user_id, enablement_id
               ORDER BY updated_at DESC, id DESC
           ) AS position
    FROM cli_connector_authorizations
    WHERE state = 'active'
)
UPDATE cli_connector_authorizations authorization
SET state = 'invalid',
    token_ciphertext = NULL,
    refresh_token_ciphertext = NULL,
    expires_at = NULL,
    updated_at = now(),
    version = version + 1
FROM ranked
WHERE ranked.id = authorization.id
  AND ranked.position > 1;

CREATE UNIQUE INDEX cli_connector_authorizations_one_active_per_connector
    ON cli_connector_authorizations (owner_user_id, enablement_id)
    WHERE state = 'active';
