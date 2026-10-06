-- Keep the login-code reservation for its original lifetime after OIDC handoff.
ALTER TABLE registration_attempts ADD COLUMN login_code_reserved_until timestamptz;
UPDATE registration_attempts SET login_code_reserved_until=expires_at WHERE login_code_hash IS NOT NULL;

CREATE TABLE registration_verification_limits (
    bucket_key text PRIMARY KEY CHECK (bucket_key = 'global' OR bucket_key ~ '^[0-9a-f]{64}$'),
    attempts integer NOT NULL CHECK (attempts > 0),
    expires_at timestamptz NOT NULL
);
