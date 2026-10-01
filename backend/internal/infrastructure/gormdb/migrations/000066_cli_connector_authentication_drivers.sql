ALTER TABLE cli_connector_definitions
    DROP CONSTRAINT cli_connector_definitions_authentication_driver_check;

ALTER TABLE cli_connector_definitions
    ADD CONSTRAINT cli_connector_definitions_authentication_driver_check CHECK (
        authentication_driver IN ('none', 'feishu', 'dingtalk', 'connector_package')
        OR (authentication_driver = '' AND state <> 'available')
    );
