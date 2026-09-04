ALTER TABLE experts
    ADD COLUMN provider_model_id uuid REFERENCES provider_models(id) ON DELETE RESTRICT,
    ADD COLUMN runtime_engine text CHECK (runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw', 'pi'));

ALTER TABLE runs ADD COLUMN native_checkpoint text NOT NULL DEFAULT '';

CREATE INDEX experts_provider_model
    ON experts (provider_model_id)
    WHERE provider_model_id IS NOT NULL;

-- Session and Workflow execution now resolve from Settings or an Expert snapshot.
UPDATE sessions SET current_provider_model_id = NULL WHERE current_provider_model_id IS NOT NULL;
UPDATE workflows SET provider_model_id = NULL, runtime_engine = NULL
    WHERE provider_model_id IS NOT NULL OR runtime_engine IS NOT NULL;
