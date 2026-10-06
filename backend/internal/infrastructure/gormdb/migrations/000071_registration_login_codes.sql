-- A browser challenge is independent of the single-use OIDC exchange code.
-- Only its hash is searchable; the display value remains in encrypted payload.
ALTER TABLE registration_attempts ADD COLUMN login_code_hash text UNIQUE;
ALTER TABLE registration_attempts ADD CONSTRAINT registration_login_code_hash_valid
 CHECK (login_code_hash IS NULL OR
 (provider = 'wechat_official' AND login_code_hash ~ '^[0-9a-f]{64}$'));
