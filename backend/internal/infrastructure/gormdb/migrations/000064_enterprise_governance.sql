DROP INDEX IF EXISTS users_single_administrator;

ALTER TABLE users
    ADD COLUMN bootstrap_administrator boolean NOT NULL DEFAULT false,
    ADD COLUMN resource_publisher boolean NOT NULL DEFAULT false;

UPDATE users
SET bootstrap_administrator = true
WHERE id = (
    SELECT id FROM users WHERE administrator = true ORDER BY created_at, id LIMIT 1
);

CREATE UNIQUE INDEX users_single_bootstrap_administrator
    ON users (bootstrap_administrator) WHERE bootstrap_administrator;

CREATE TABLE identity_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text NOT NULL UNIQUE,
    name text NOT NULL,
    path text NOT NULL,
    department boolean NOT NULL DEFAULT false,
    daily_credit_limit_hundredths bigint CHECK (daily_credit_limit_hundredths IS NULL OR daily_credit_limit_hundredths >= 0),
    last_synced_at timestamptz NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE UNIQUE INDEX identity_groups_active_path ON identity_groups(path) WHERE deleted_at IS NULL;

CREATE TABLE identity_group_memberships (
    group_id uuid NOT NULL REFERENCES identity_groups(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_synced_at timestamptz NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX identity_group_memberships_user ON identity_group_memberships(user_id, group_id);

CREATE TABLE governance_audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    reason text NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(action) BETWEEN 1 AND 100),
    CHECK (length(target_type) BETWEEN 1 AND 100),
    CHECK (length(target_id) BETWEEN 1 AND 200),
    CHECK (length(reason) BETWEEN 1 AND 500)
);

CREATE INDEX governance_audit_events_recent ON governance_audit_events(occurred_at DESC, id DESC);

ALTER TABLE knowledge_bases
    ADD COLUMN scope_type text NOT NULL DEFAULT 'private',
    ADD COLUMN group_id uuid REFERENCES identity_groups(id) ON DELETE RESTRICT;

UPDATE knowledge_bases SET scope_type = 'platform' WHERE platform = true AND visibility = 'public';
UPDATE knowledge_bases SET platform = false, scope_type = 'private' WHERE platform = true AND visibility = 'private';

ALTER TABLE knowledge_bases DROP CONSTRAINT IF EXISTS knowledge_bases_check;
ALTER TABLE knowledge_bases
    ADD CONSTRAINT knowledge_bases_scope_valid CHECK (
        (scope_type = 'private' AND platform = false AND visibility = 'private' AND group_id IS NULL)
        OR (scope_type = 'platform' AND platform = true AND visibility = 'public' AND group_id IS NULL)
        OR (scope_type = 'group' AND platform = false AND visibility = 'private' AND group_id IS NOT NULL)
    );

CREATE INDEX knowledge_bases_group_catalog
    ON knowledge_bases(group_id, deleted_at, updated_at DESC) WHERE scope_type = 'group';
