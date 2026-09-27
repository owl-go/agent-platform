ALTER TABLE connector_revisions
    ADD CONSTRAINT connector_revisions_source_id_unique UNIQUE (package_source, id);

CREATE TABLE connector_package_publications (
    package_source text PRIMARY KEY,
    active_revision_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('available', 'disabled')),
    administrator_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (package_source, active_revision_id)
        REFERENCES connector_revisions(package_source, id)
);

CREATE INDEX connector_package_publications_state_updated
    ON connector_package_publications(state, updated_at DESC);

CREATE INDEX connector_authorizations_installation_state_updated
    ON connector_authorizations(installation_id, state, updated_at DESC);
