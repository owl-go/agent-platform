ALTER TABLE cli_connector_definitions
    ADD COLUMN recommended_skills jsonb NOT NULL DEFAULT '[]'::jsonb;
