ALTER TABLE session_messages DROP CONSTRAINT session_messages_state_check;
ALTER TABLE session_messages ADD CONSTRAINT session_messages_state_check CHECK (state IN ('queued', 'generating', 'waiting_for_user', 'completed', 'failed', 'cancelled'));
ALTER TABLE runs DROP CONSTRAINT runs_state_check;
ALTER TABLE runs ADD CONSTRAINT runs_state_check CHECK (state IN ('queued', 'running', 'waiting_for_user', 'succeeded', 'failed', 'cancelled'));

CREATE TABLE cli_connector_definitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    npm_package text NOT NULL,
    npm_version text NOT NULL,
    npm_integrity text NOT NULL,
    executable text NOT NULL,
    authentication_driver text NOT NULL CHECK (authentication_driver IN ('none', 'feishu')),
    capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    recommended_skill_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'building', 'testing', 'available', 'failed', 'disabled')),
    failure_reason text,
    bundle_object_key text,
    bundle_sha256 text CHECK (bundle_sha256 IS NULL OR bundle_sha256 ~ '^[a-f0-9]{64}$'),
    created_by_user_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (npm_package, npm_version)
);

CREATE TABLE cli_connector_conformance (
    definition_id uuid NOT NULL REFERENCES cli_connector_definitions(id) ON DELETE CASCADE,
    bundle_sha256 text NOT NULL CHECK (bundle_sha256 ~ '^[a-f0-9]{64}$'),
    runtime_repo_digest text NOT NULL CHECK (runtime_repo_digest LIKE 'sha256:%'),
    environment jsonb NOT NULL DEFAULT '{}'::jsonb,
    tested_at timestamptz NOT NULL,
    passed boolean NOT NULL,
    PRIMARY KEY (definition_id, bundle_sha256, runtime_repo_digest)
);

CREATE TABLE cli_connector_enablements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    definition_id uuid NOT NULL REFERENCES cli_connector_definitions(id),
    state text NOT NULL CHECK (state IN ('waiting_for_user', 'enabled', 'invalid', 'disabled')),
    action_url text,
    action_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, definition_id)
);

CREATE TABLE feishu_cli_applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    enablement_id uuid NOT NULL UNIQUE REFERENCES cli_connector_enablements(id) ON DELETE CASCADE,
    provider_application_id_ciphertext bytea NOT NULL,
    provider_application_secret_ciphertext bytea NOT NULL,
    provider_name text NOT NULL,
    developer_console_url text NOT NULL,
    granted_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE cli_connector_authorizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enablement_id uuid NOT NULL REFERENCES cli_connector_enablements(id) ON DELETE CASCADE,
    identity text NOT NULL CHECK (identity IN ('user', 'bot')),
    external_identity_id text NOT NULL,
    external_display_name text NOT NULL,
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    token_ciphertext bytea,
    refresh_token_ciphertext bytea,
    expires_at timestamptz,
    state text NOT NULL CHECK (state IN ('active', 'invalid', 'disconnected')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, enablement_id, identity, external_identity_id)
);

CREATE TABLE cli_command_approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    execution_kind text NOT NULL CHECK (execution_kind IN ('session', 'run')),
    execution_id text NOT NULL,
    stage_id text NOT NULL,
    connector_name text NOT NULL,
    operation text NOT NULL,
    target text NOT NULL,
    redacted_arguments text NOT NULL,
    command_digest text NOT NULL CHECK (command_digest ~ '^[a-f0-9]{64}$'),
    nonce_hash text NOT NULL UNIQUE,
    identity text CHECK (identity IS NULL OR identity IN ('user', 'bot')),
    state text NOT NULL CHECK (state IN ('pending', 'approved', 'rejected', 'consumed', 'expired', 'closed')),
    expires_at timestamptz NOT NULL,
    decided_at timestamptz,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (stage_id, id)
);

CREATE UNIQUE INDEX cli_command_approvals_one_active_per_stage ON cli_command_approvals (stage_id) WHERE state = 'pending';
CREATE INDEX cli_command_approvals_owner_state ON cli_command_approvals (owner_user_id, state, expires_at);
