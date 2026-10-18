ALTER TABLE experts ADD COLUMN connector_dependencies jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(connector_dependencies)='array');
