ALTER TABLE cli_connector_definitions
    ADD COLUMN supported_architectures jsonb NOT NULL DEFAULT '["linux-amd64"]'::jsonb;
