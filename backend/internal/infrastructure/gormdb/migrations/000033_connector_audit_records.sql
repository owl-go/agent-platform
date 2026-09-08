CREATE TABLE connector_audit_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_id text NOT NULL,
    owner_user_id uuid NOT NULL REFERENCES users(id),
    connector_id uuid NOT NULL REFERENCES cli_connector_definitions(id),
    manifest_version text NOT NULL,
    capability_id text NOT NULL,
    permissions jsonb NOT NULL DEFAULT '[]'::jsonb,
    execution_identity text NOT NULL CHECK (execution_identity IN ('user', 'bot')),
    authorization_id uuid REFERENCES cli_connector_authorizations(id),
    action text NOT NULL,
    reason text NOT NULL DEFAULT '',
    result text NOT NULL DEFAULT '',
    target_summary text NOT NULL DEFAULT '',
    input_digest text NOT NULL CHECK (input_digest ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL
);

CREATE INDEX connector_audit_records_owner_operation
    ON connector_audit_records (owner_user_id, operation_id, occurred_at);

ALTER TABLE cli_command_approvals
    ADD COLUMN operation_id text NOT NULL DEFAULT '',
    ADD COLUMN manifest_version text NOT NULL DEFAULT '',
    ADD COLUMN input_digest text NOT NULL DEFAULT '',
    ADD COLUMN authorization_id uuid REFERENCES cli_connector_authorizations(id),
    ADD COLUMN external_identity_id text NOT NULL DEFAULT '',
    ADD COLUMN external_display_name text NOT NULL DEFAULT '';
