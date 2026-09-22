CREATE TABLE connector_revisions (
    id uuid PRIMARY KEY,
    package_source text NOT NULL,
    version text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('mcp', 'cli')),
    package_sha256 text NOT NULL CHECK (package_sha256 ~ '^[a-f0-9]{64}$'),
    runtime_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    object_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (package_source, version, package_sha256)
);

CREATE TABLE connector_authorizations (
    id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id uuid NOT NULL,
    identity_ref text NOT NULL,
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    credential_ciphertext bytea NOT NULL,
    state text NOT NULL CHECK (state IN ('active', 'expired', 'disconnected', 'revoked')),
    expires_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connector_installations (
    id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_source text NOT NULL,
    active_revision_id uuid NOT NULL REFERENCES connector_revisions(id),
    authorization_id uuid REFERENCES connector_authorizations(id) ON DELETE SET NULL,
    state text NOT NULL CHECK (state IN ('pending', 'active', 'disabled', 'uninstalled')),
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, package_source)
);

ALTER TABLE connector_installations ADD CONSTRAINT connector_installations_owner_id_unique UNIQUE (owner_user_id, id);
ALTER TABLE connector_authorizations ADD CONSTRAINT connector_authorizations_owner_installation_fk FOREIGN KEY (owner_user_id, installation_id) REFERENCES connector_installations(owner_user_id, id) ON DELETE CASCADE;

ALTER TABLE connector_authorizations
    ADD CONSTRAINT connector_authorizations_installation_fk
    FOREIGN KEY (installation_id) REFERENCES connector_installations(id) ON DELETE CASCADE;

CREATE TABLE connector_package_audit_records (
    id uuid PRIMARY KEY,
    owner_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    installation_id uuid REFERENCES connector_installations(id) ON DELETE SET NULL,
    revision_id uuid REFERENCES connector_revisions(id) ON DELETE SET NULL,
    mode text NOT NULL DEFAULT '',
    operation text NOT NULL,
    risk text NOT NULL DEFAULT '',
    identity_ref text NOT NULL DEFAULT '',
    approval_reference text NOT NULL DEFAULT '',
    outcome text NOT NULL,
    error_type text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    policy_revision text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX connector_revisions_source_created ON connector_revisions(package_source, created_at DESC);

CREATE OR REPLACE FUNCTION reject_connector_revision_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'connector revisions are immutable';
END;
$$;
CREATE TRIGGER connector_revisions_immutable BEFORE UPDATE OR DELETE ON connector_revisions FOR EACH ROW EXECUTE FUNCTION reject_connector_revision_mutation();

CREATE INDEX connector_installations_owner_state ON connector_installations(owner_user_id, state);
CREATE INDEX connector_package_audit_records_installation_created ON connector_package_audit_records(installation_id, created_at DESC);
