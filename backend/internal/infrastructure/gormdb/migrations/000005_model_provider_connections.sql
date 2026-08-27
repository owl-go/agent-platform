CREATE TABLE model_provider_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    provider_type text NOT NULL CHECK (provider_type IN ('openai', 'anthropic', 'google_gemini', 'xai', 'deepseek', 'alibaba_bailian', 'volcengine_ark', 'moonshot', 'zhipu', 'minimax', 'custom_openai')),
    endpoint text NOT NULL,
    protocols jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(protocols) = 'array' AND jsonb_array_length(protocols) > 0),
    api_key_ciphertext bytea NOT NULL,
    verification_status text NOT NULL DEFAULT 'unverified' CHECK (verification_status IN ('verified', 'unverified', 'failed')),
    verification_error text,
    custom_endpoint boolean NOT NULL DEFAULT false,
    last_synced_at timestamptz,
    last_sync_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE TABLE provider_models (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id uuid NOT NULL REFERENCES model_provider_connections(id) ON DELETE CASCADE,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id text NOT NULL,
    display_name text NOT NULL,
    model_type text NOT NULL DEFAULT 'unknown' CHECK (model_type IN ('agent', 'text', 'embedding', 'image', 'audio', 'unknown')),
    available boolean NOT NULL DEFAULT true,
    manually_added boolean NOT NULL DEFAULT false,
    compatibility jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(compatibility) = 'array'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connection_id, model_id)
);

CREATE TABLE model_provider_credential_versions (
    connection_id uuid NOT NULL REFERENCES model_provider_connections(id) ON DELETE CASCADE,
    connection_version bigint NOT NULL CHECK (connection_version > 0),
    api_key_ciphertext bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (connection_id, connection_version)
);

ALTER TABLE personal_settings
    ADD COLUMN runtime_model_defaults jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(runtime_model_defaults) = 'object'),
    DROP COLUMN default_model_profile_id;

ALTER TABLE sessions
    ADD COLUMN current_provider_model_id uuid REFERENCES provider_models(id) ON DELETE RESTRICT;

ALTER TABLE session_messages
    ADD COLUMN response_snapshot jsonb;

ALTER TABLE workflows
    ADD COLUMN provider_model_id uuid REFERENCES provider_models(id) ON DELETE RESTRICT,
    DROP COLUMN model_profile_id;

DROP TABLE model_profiles;

CREATE INDEX provider_connections_owner ON model_provider_connections (owner_user_id, updated_at DESC);
CREATE INDEX provider_models_owner_available ON provider_models (owner_user_id, available, display_name);
