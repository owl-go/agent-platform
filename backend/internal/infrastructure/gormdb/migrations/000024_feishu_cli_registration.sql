ALTER TABLE cli_connector_enablements
    ADD COLUMN registration_device_code_ciphertext bytea;

CREATE INDEX cli_connector_enablements_waiting_registration
    ON cli_connector_enablements (owner_user_id, action_expires_at)
    WHERE state = 'waiting_for_user';
