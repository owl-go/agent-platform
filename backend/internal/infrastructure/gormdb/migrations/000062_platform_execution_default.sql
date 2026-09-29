ALTER TABLE personal_settings
    ADD COLUMN execution_inherited boolean NOT NULL DEFAULT true;

CREATE TABLE platform_execution_defaults (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    runtime_engine text NOT NULL CHECK (runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw', 'pi')),
    provider_model_id uuid NOT NULL REFERENCES provider_models(id) ON DELETE RESTRICT,
    validation_run_id uuid NOT NULL REFERENCES runs(id) ON DELETE RESTRICT,
    updated_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE OR REPLACE FUNCTION inherit_platform_execution_default()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE personal_settings settings
    SET default_runtime_engine = defaults.runtime_engine,
        runtime_model_defaults = jsonb_build_object(defaults.runtime_engine, defaults.provider_model_id::text)
    FROM platform_execution_defaults defaults
    WHERE defaults.singleton
      AND settings.user_id = NEW.user_id
      AND settings.execution_inherited;
    RETURN NEW;
END;
$$;

CREATE TRIGGER personal_settings_inherit_platform_execution_default
AFTER INSERT ON personal_settings
FOR EACH ROW
EXECUTE FUNCTION inherit_platform_execution_default();
