-- Existing Assistants keep an empty selection and use the legacy Personal
-- Settings fallback until their owner selects an openai_chat Provider Model.
ALTER TABLE smart_assistants
    ADD COLUMN provider_model_id text NOT NULL DEFAULT '';
