ALTER TABLE personal_settings
    DROP CONSTRAINT personal_settings_default_runtime_engine_check,
    ADD CONSTRAINT personal_settings_default_runtime_engine_check
        CHECK (default_runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw', 'pi'));

ALTER TABLE workflows
    DROP CONSTRAINT workflows_runtime_engine_check,
    ADD CONSTRAINT workflows_runtime_engine_check
        CHECK (runtime_engine IS NULL OR runtime_engine IN ('claude', 'codex', 'hermes', 'openclaw', 'pi'));
