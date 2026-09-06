CREATE TABLE cli_connector_authorization_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enablement_id uuid NOT NULL REFERENCES cli_connector_enablements(id) ON DELETE CASCADE,
    identity text NOT NULL CHECK (identity IN ('user')),
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    device_code_ciphertext bytea NOT NULL,
    action_url text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, enablement_id, identity)
);
