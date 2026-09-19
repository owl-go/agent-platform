ALTER TABLE workflows
    ADD COLUMN IF NOT EXISTS api_secret_ciphertext bytea;
