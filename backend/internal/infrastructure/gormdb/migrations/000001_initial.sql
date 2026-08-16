CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    slug text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, slug)
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    oidc_subject text NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, oidc_subject),
    UNIQUE (organization_id, email)
);

CREATE TABLE role_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid REFERENCES teams(id),
    user_id uuid NOT NULL REFERENCES users(id),
    role text NOT NULL CHECK (role IN ('platform_administrator', 'agent_builder', 'agent_user', 'run_operator')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (organization_id, team_id, user_id, role)
);

CREATE TABLE credential_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid REFERENCES teams(id),
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('model', 'git_ssh', 'build', 'object_storage')),
    secret_ref text NOT NULL,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE NULLS NOT DISTINCT (organization_id, team_id, name)
);

CREATE TABLE configured_models (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    name text NOT NULL,
    model_id text NOT NULL,
    endpoint text NOT NULL,
    credential_profile_id uuid NOT NULL REFERENCES credential_profiles(id),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, name)
);

CREATE TABLE runtime_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    runtime text NOT NULL CHECK (runtime IN ('claude', 'codex', 'hermes', 'openclaw')),
    cli_version text NOT NULL,
    adapter_version text NOT NULL,
    image_digest text NOT NULL UNIQUE CHECK (image_digest ~ '^[^[:space:]@]+@sha256:[a-f0-9]{64}$'),
    capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'experimental' CHECK (status IN ('experimental', 'production', 'blocked', 'deprecated')),
    blocked_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE source_control_providers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('github_com', 'gitlab_self_managed')),
    base_url text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, name)
);

CREATE TABLE repository_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid NOT NULL REFERENCES teams(id),
    source_control_provider_id uuid NOT NULL REFERENCES source_control_providers(id),
    name text NOT NULL,
    repository_ssh_url text NOT NULL,
    default_branch text NOT NULL,
    ssh_credential_profile_id uuid NOT NULL REFERENCES credential_profiles(id),
    git_author_name text NOT NULL,
    git_author_email text NOT NULL,
    allowed_runtime_image_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    default_runtime_image_id uuid REFERENCES runtime_images(id),
    default_model_id uuid REFERENCES configured_models(id),
    model_budget jsonb NOT NULL DEFAULT '{}'::jsonb,
    instructions text NOT NULL DEFAULT '',
    quality_commands jsonb NOT NULL DEFAULT '[]'::jsonb,
    egress_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    validation_report jsonb,
    validated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, team_id, name)
);

CREATE TABLE agents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid NOT NULL REFERENCES teams(id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (organization_id, team_id, name)
);

CREATE TABLE agent_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id uuid NOT NULL REFERENCES agents(id),
    revision bigint NOT NULL CHECK (revision > 0),
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'validating', 'ready', 'blocked')),
    configuration jsonb NOT NULL,
    release_risk text NOT NULL DEFAULT 'low' CHECK (release_risk IN ('low', 'high')),
    validation_report jsonb,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (agent_id, revision)
);

CREATE TABLE agent_releases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id uuid NOT NULL REFERENCES agents(id),
    release_number bigint NOT NULL CHECK (release_number > 0),
    source_draft_id uuid NOT NULL REFERENCES agent_drafts(id),
    runtime_image_id uuid NOT NULL REFERENCES runtime_images(id),
    configured_model_id uuid NOT NULL REFERENCES configured_models(id),
    repository_binding_id uuid NOT NULL REFERENCES repository_bindings(id),
    configuration_snapshot jsonb NOT NULL,
    model_budget jsonb NOT NULL,
    execution_limits jsonb NOT NULL,
    status text NOT NULL DEFAULT 'released' CHECK (status IN ('released', 'deprecated', 'blocked')),
    released_by uuid NOT NULL REFERENCES users(id),
    released_at timestamptz NOT NULL DEFAULT now(),
    deprecated_at timestamptz,
    UNIQUE (agent_id, release_number)
);

CREATE TABLE coding_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid NOT NULL REFERENCES teams(id),
    agent_release_id uuid NOT NULL REFERENCES agent_releases(id),
    created_by uuid NOT NULL REFERENCES users(id),
    title text NOT NULL,
    request_text text NOT NULL,
    issue_snapshot jsonb,
    state text NOT NULL DEFAULT 'created' CHECK (state IN ('created', 'active', 'waiting_for_user', 'completed', 'cancelled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coding_task_id uuid NOT NULL UNIQUE REFERENCES coding_tasks(id),
    repository_binding_id uuid NOT NULL REFERENCES repository_bindings(id),
    target_branch text NOT NULL,
    review_branch text NOT NULL,
    workspace_volume text NOT NULL UNIQUE,
    session_memory jsonb NOT NULL DEFAULT '{}'::jsonb,
    run_count integer NOT NULL DEFAULT 0 CHECK (run_count BETWEEN 0 AND 50),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (repository_binding_id, review_branch)
);

CREATE TABLE runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES sessions(id),
    agent_release_id uuid NOT NULL REFERENCES agent_releases(id),
    runtime_image_id uuid NOT NULL REFERENCES runtime_images(id),
    model_binding jsonb NOT NULL,
    credential_bindings jsonb NOT NULL DEFAULT '[]'::jsonb,
    request_text text NOT NULL,
    state text NOT NULL DEFAULT 'queued' CHECK (state IN (
        'queued', 'provisioning', 'running', 'waiting_confirmation', 'interrupting',
        'interrupted', 'resuming', 'completed', 'failed', 'cancelled'
    )),
    model_budget jsonb NOT NULL,
    execution_limits jsonb NOT NULL,
    usage jsonb NOT NULL DEFAULT '{}'::jsonb,
    cost_amount numeric(20, 8) NOT NULL DEFAULT 0 CHECK (cost_amount >= 0),
    terminal_error jsonb,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    ended_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE INDEX runs_queue_index ON runs (created_at, id) WHERE state IN ('queued', 'resuming');
CREATE INDEX runs_session_index ON runs (session_id, created_at DESC);

CREATE TABLE run_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES runs(id),
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    worker_id text NOT NULL,
    state text NOT NULL CHECK (state IN ('provisioning', 'running', 'completed', 'failed', 'cancelled', 'lost')),
    infrastructure_failure boolean NOT NULL DEFAULT false,
    error jsonb,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    UNIQUE (run_id, attempt_number)
);

CREATE TABLE run_leases (
    run_id uuid PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    attempt_id uuid NOT NULL UNIQUE REFERENCES run_attempts(id) ON DELETE CASCADE,
    worker_id text NOT NULL,
    lease_token uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX run_leases_expiry_index ON run_leases (expires_at);

CREATE TABLE workspace_write_leases (
    session_id uuid PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    run_id uuid NOT NULL UNIQUE REFERENCES runs(id) ON DELETE CASCADE,
    lease_token uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE run_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, sequence)
);

CREATE INDEX run_events_replay_index ON run_events (run_id, sequence);

CREATE TABLE approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES runs(id),
    kind text NOT NULL,
    request jsonb NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'rejected', 'expired', 'cancelled')),
    requested_at timestamptz NOT NULL DEFAULT now(),
    decided_by uuid REFERENCES users(id),
    decided_at timestamptz,
    decision_reason text,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE session_messages (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id uuid REFERENCES runs(id) ON DELETE SET NULL,
    author_type text NOT NULL CHECK (author_type IN ('user', 'agent', 'system')),
    author_user_id uuid REFERENCES users(id),
    content jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES runs(id),
    kind text NOT NULL,
    object_key text NOT NULL UNIQUE,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 text NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    content_type text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    expires_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE agent_memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id uuid NOT NULL REFERENCES agents(id),
    content text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    approved_by uuid NOT NULL REFERENCES users(id),
    source_task_id uuid REFERENCES coding_tasks(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE memory_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id uuid NOT NULL REFERENCES agents(id),
    coding_task_id uuid NOT NULL REFERENCES coding_tasks(id),
    proposed_content text NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'rejected')),
    proposed_at timestamptz NOT NULL DEFAULT now(),
    decided_by uuid REFERENCES users(id),
    decided_at timestamptz,
    resulting_memory_id uuid REFERENCES agent_memories(id)
);

CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id),
    team_id uuid REFERENCES teams(id),
    actor_user_id uuid REFERENCES users(id),
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_lookup_index ON audit_events (organization_id, created_at DESC);

CREATE TABLE idempotency_keys (
    organization_id uuid NOT NULL REFERENCES organizations(id),
    key text NOT NULL,
    operation text NOT NULL,
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[a-f0-9]{64}$'),
    response_status integer,
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, key, operation)
);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    target_url text NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'delivering', 'delivered', 'failed', 'cancelled')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX webhook_delivery_queue_index ON webhook_deliveries (next_attempt_at, id) WHERE state IN ('pending', 'failed');
