-- External users may have no email. Never fabricate a verified address or
-- associate accounts by display name/email returned by an external provider.
ALTER TABLE users DROP CONSTRAINT users_email_key;
CREATE UNIQUE INDEX users_nonempty_email_key ON users(email) WHERE email <> '';
CREATE TABLE registration_settings (
 provider text PRIMARY KEY CHECK (provider IN ('feishu', 'wechat_official')),
 enabled boolean NOT NULL DEFAULT false,
 ready boolean NOT NULL DEFAULT false,
 config_ciphertext bytea NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE registration_attempts (
 id text PRIMARY KEY,
 provider text NOT NULL CHECK (provider IN ('feishu', 'wechat_official')),
 status text NOT NULL CHECK (status IN ('waiting','verified','issued','consumed')),
 code_hash text UNIQUE,
 payload_ciphertext bytea NOT NULL,
 expires_at timestamptz NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE INDEX registration_attempts_expiry ON registration_attempts(expires_at);
