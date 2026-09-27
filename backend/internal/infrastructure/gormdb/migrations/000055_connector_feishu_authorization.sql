CREATE TABLE connector_provider_applications (
    owner_user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL UNIQUE REFERENCES connector_installations(id) ON DELETE CASCADE,
    provider_application_id_ciphertext bytea NOT NULL,
    provider_application_secret_ciphertext bytea NOT NULL,
    provider_name text NOT NULL,
    developer_console_url text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

ALTER TABLE connector_authorizations
    ADD COLUMN credential_aad text NOT NULL DEFAULT '',
    ADD COLUMN external_identity_id text NOT NULL DEFAULT '',
    ADD COLUMN external_display_name text NOT NULL DEFAULT '',
    ADD COLUMN refresh_credential_ciphertext bytea,
    ADD COLUMN refresh_credential_aad text NOT NULL DEFAULT '',
    ADD COLUMN credential_format text NOT NULL DEFAULT 'json'
        CHECK (credential_format IN ('json', 'access_token'));

CREATE UNIQUE INDEX connector_authorizations_external_identity_unique
    ON connector_authorizations(owner_user_id, installation_id, identity_ref, external_identity_id)
    WHERE external_identity_id <> '';

CREATE TABLE connector_setup_flows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL REFERENCES connector_installations(id) ON DELETE CASCADE,
    device_code_ciphertext bytea NOT NULL,
    action_url text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, installation_id)
);

CREATE TABLE connector_authorization_flows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL REFERENCES connector_installations(id) ON DELETE CASCADE,
    identity text NOT NULL CHECK (identity = 'user'),
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    device_code_ciphertext bytea NOT NULL,
    action_url text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, installation_id, identity)
);
