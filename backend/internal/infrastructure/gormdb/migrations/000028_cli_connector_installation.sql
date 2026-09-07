ALTER TABLE cli_connector_definitions
    ADD COLUMN icon text NOT NULL DEFAULT 'terminal',
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN installation_type text NOT NULL DEFAULT 'npm'
        CHECK (installation_type IN ('npm', 'upload')),
    ADD COLUMN source_object_key text,
    ADD COLUMN source_sha256 text
        CHECK (source_sha256 IS NULL OR source_sha256 ~ '^[a-f0-9]{64}$');

ALTER TABLE cli_connector_definitions
    ADD CONSTRAINT cli_connector_source_pair CHECK (
        (source_object_key IS NULL AND source_sha256 IS NULL)
        OR (source_object_key IS NOT NULL AND source_sha256 IS NOT NULL)
    );

ALTER TABLE cli_connector_definitions
    DROP CONSTRAINT cli_connector_definitions_npm_package_npm_version_key;

CREATE UNIQUE INDEX cli_connector_definitions_exact_npm_package
    ON cli_connector_definitions (npm_package, npm_version)
    WHERE installation_type = 'npm';
