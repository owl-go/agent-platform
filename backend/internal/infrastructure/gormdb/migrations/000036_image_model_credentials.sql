ALTER TABLE image_model_revisions
    ALTER COLUMN connection_id DROP NOT NULL,
    ALTER COLUMN connection_version DROP NOT NULL,
    ALTER COLUMN connection_name DROP NOT NULL,
    ADD COLUMN endpoint text,
    ADD COLUMN api_key_ciphertext bytea;

ALTER TABLE image_model_revisions
    DROP CONSTRAINT image_model_revisions_connection_id_fkey;

COMMENT ON COLUMN image_model_revisions.connection_id IS 'Legacy Model Provider Connection reference; new Image Model revisions leave this null.';
COMMENT ON COLUMN image_model_revisions.api_key_ciphertext IS 'Write-only Image Model API Key encrypted with the platform secret box.';

CREATE TABLE prompt_optimization_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    model_id text NOT NULL,
    endpoint text NOT NULL,
    api_key_ciphertext bytea NOT NULL,
    instruction text NOT NULL,
    updated_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Prompt optimization now has one independent configuration rather than a
-- list backed by Model Provider Connections. Existing secrets are not copied.
DROP TABLE prompt_optimization_candidates;
