ALTER TABLE cli_connector_definitions ADD COLUMN deleted_at timestamptz;
ALTER TABLE cli_connector_definitions ADD CONSTRAINT cli_connector_deleted_state
    CHECK (deleted_at IS NULL OR state = 'disabled');

ALTER TABLE cli_connector_definitions DROP CONSTRAINT cli_connector_definitions_name_key;
CREATE UNIQUE INDEX cli_connector_definitions_active_name
    ON cli_connector_definitions (name) WHERE deleted_at IS NULL;
DROP INDEX cli_connector_definitions_exact_npm_package;
CREATE UNIQUE INDEX cli_connector_definitions_exact_npm_package
    ON cli_connector_definitions (npm_package, npm_version)
    WHERE installation_type = 'npm' AND deleted_at IS NULL;
