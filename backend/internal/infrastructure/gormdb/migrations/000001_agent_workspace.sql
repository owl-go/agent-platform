-- Initial schema for Agent Workspace. Legacy deployments are backed up and
-- reset before applying this schema; historical control-plane tables are not
-- migrated into the replacement product.

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    oidc_subject text NOT NULL UNIQUE,
    username text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    display_name text NOT NULL,
    administrator boolean NOT NULL DEFAULT false,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE UNIQUE INDEX users_single_administrator ON users (administrator) WHERE administrator;

CREATE TABLE model_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    model_id text NOT NULL,
    endpoint text NOT NULL,
    secret_ciphertext bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE TABLE personal_settings (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    personality text NOT NULL DEFAULT 'direct_efficient' CHECK (personality IN ('gentle_professional', 'direct_efficient', 'lively_friendly', 'custom')),
    personality_instructions text NOT NULL DEFAULT '',
    default_model_profile_id uuid REFERENCES model_profiles(id) ON DELETE SET NULL,
    default_runtime_engine text NOT NULL DEFAULT 'codex' CHECK (default_runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw')),
    language text NOT NULL DEFAULT 'zh-CN' CHECK (language IN ('zh-CN', 'en-US')),
    timezone text NOT NULL DEFAULT 'Asia/Shanghai',
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE mcp_servers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    transport text NOT NULL CHECK (transport IN ('streamable_http', 'stdio')),
    configuration jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret_ciphertext bytea,
    test_requested_at timestamptz,
    tested_at timestamptz,
    test_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE TABLE skills (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    source text NOT NULL CHECK (source IN ('git', 'upload')),
    git_url text,
    git_ref text,
    object_key text NOT NULL,
    sha256 text NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE TABLE experts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    mcp_server_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    skill_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL DEFAULT 'New session',
    expert_id uuid REFERENCES experts(id) ON DELETE SET NULL,
    expert_snapshot jsonb,
    archived_at timestamptz,
    rolling_summary text NOT NULL DEFAULT '',
    summary_through_message_id bigint,
    runtime_engine text,
    native_checkpoint text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE INDEX sessions_owner_state ON sessions (owner_user_id, archived_at, updated_at DESC);

CREATE TABLE session_messages (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant')),
    state text NOT NULL CHECK (state IN ('queued', 'generating', 'completed', 'failed', 'cancelled')),
    content text NOT NULL DEFAULT '',
    error text,
    elapsed_ms bigint NOT NULL DEFAULT 0 CHECK (elapsed_ms >= 0),
    checkpoint_before text,
    checkpoint_after text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE INDEX session_messages_order ON session_messages (session_id, id);

CREATE TABLE workflows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    goal text NOT NULL,
    expert_id uuid REFERENCES experts(id) ON DELETE SET NULL,
    model_profile_id uuid REFERENCES model_profiles(id) ON DELETE SET NULL,
    runtime_engine text CHECK (runtime_engine IS NULL OR runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw')),
    environment jsonb NOT NULL DEFAULT '[]'::jsonb,
    environment_secret_ciphertext bytea,
    schedule jsonb,
    next_scheduled_at timestamptz,
    git_source jsonb,
    git_secret_ciphertext bytea,
    api_key text UNIQUE,
    api_secret_hash text,
    workspace_path text NOT NULL UNIQUE,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE NULLS NOT DISTINCT (owner_user_id, deleted_at, name)
);

CREATE INDEX workflows_owner_state ON workflows (owner_user_id, deleted_at, updated_at DESC);
CREATE INDEX workflows_schedule_due ON workflows (next_scheduled_at) WHERE deleted_at IS NULL AND next_scheduled_at IS NOT NULL;

CREATE TABLE runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workflow_id uuid REFERENCES workflows(id) ON DELETE SET NULL,
    workflow_name text NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('manual', 'scheduled', 'api')),
    state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    workflow_snapshot jsonb NOT NULL,
    final_result jsonb,
    terminal_error text,
    workspace_base_digest text,
    queued_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    ended_at timestamptz,
    cancel_requested_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE INDEX runs_queue ON runs (queued_at, id) WHERE state = 'queued';
CREATE INDEX runs_workflow_history ON runs (owner_user_id, workflow_id, queued_at DESC);

CREATE TABLE run_events (
    run_id uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, sequence)
);

CREATE TABLE artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workflow_id uuid REFERENCES workflows(id) ON DELETE SET NULL,
    run_id uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('result', 'file')),
    name text NOT NULL,
    path text NOT NULL DEFAULT '',
    object_key text,
    text_result jsonb,
    size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    sha256 text CHECK (sha256 IS NULL OR sha256 ~ '^[a-f0-9]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);

CREATE INDEX artifacts_workflow_history ON artifacts (owner_user_id, workflow_id, created_at DESC);

CREATE TABLE idempotency_records (
    owner_scope text NOT NULL,
    operation text NOT NULL,
    idempotency_key text NOT NULL,
    request_digest text NOT NULL,
    response_status integer,
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_scope, operation, idempotency_key)
);
