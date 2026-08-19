ALTER TABLE repository_bindings
    ADD COLUMN required_runtime_capabilities jsonb NOT NULL DEFAULT '[]'::jsonb;
