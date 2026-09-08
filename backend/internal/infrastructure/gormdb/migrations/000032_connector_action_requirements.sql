CREATE TABLE connector_action_requirements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    execution_kind text NOT NULL CHECK (execution_kind IN ('session', 'run')),
    execution_id text NOT NULL,
    stage_id text NOT NULL,
    operation_id text NOT NULL,
    connector_id uuid NOT NULL REFERENCES cli_connector_definitions(id),
    connector_name text NOT NULL,
    authorization_scheme text NOT NULL,
    identity text NOT NULL CHECK (identity IN ('user', 'bot')),
    enablement_id uuid NOT NULL REFERENCES cli_connector_enablements(id) ON DELETE CASCADE,
    capability_id text NOT NULL,
    operation_phrase jsonb NOT NULL DEFAULT '{}'::jsonb,
    reason text NOT NULL CHECK (reason IN (
        'authorization_required', 'setup_required', 'permissions_missing',
        'expired', 'refresh_failed', 'provider_denied', 'provider_unavailable',
        'scheme_unavailable', 'account_selection_required',
        'cancelled_by_user', 'user_action_expired'
    )),
    permissions jsonb NOT NULL DEFAULT '[]'::jsonb,
    actions jsonb NOT NULL DEFAULT '[]'::jsonb,
    state text NOT NULL CHECK (state IN ('pending', 'resolved', 'cancelled', 'expired')),
    action_url_token_hash bytea,
    action_url_opened_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, operation_id)
);

CREATE INDEX connector_action_requirements_owner_pending
    ON connector_action_requirements (owner_user_id, state, expires_at);
