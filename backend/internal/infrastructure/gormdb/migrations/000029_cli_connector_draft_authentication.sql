ALTER TABLE cli_connector_definitions
    DROP CONSTRAINT cli_connector_definitions_authentication_driver_check;

-- Installation metadata is resolved by the isolated Builder, before availability.
ALTER TABLE cli_connector_definitions
    ADD CONSTRAINT cli_connector_definitions_authentication_driver_check CHECK (
        authentication_driver IN ('none', 'feishu')
        OR (authentication_driver = '' AND state <> 'available')
    );
