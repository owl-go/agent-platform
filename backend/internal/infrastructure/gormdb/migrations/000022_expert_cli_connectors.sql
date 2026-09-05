ALTER TABLE experts
    ADD COLUMN cli_connector_definition_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
