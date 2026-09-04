DROP INDEX provider_connections_owner;
DROP INDEX provider_models_owner_available;

ALTER TABLE model_provider_connections
    RENAME COLUMN owner_user_id TO credential_owner_user_id;

ALTER TABLE provider_models
    DROP COLUMN owner_user_id;

CREATE INDEX provider_connections_global
    ON model_provider_connections (updated_at DESC, id DESC);

CREATE INDEX provider_models_global_available
    ON provider_models (available, display_name, model_id);
