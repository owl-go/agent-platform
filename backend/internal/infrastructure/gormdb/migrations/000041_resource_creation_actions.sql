CREATE TABLE resource_creation_actions (
    id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id bigint NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('skill', 'expert')),
    state text NOT NULL CHECK (state IN ('pending', 'processing', 'confirmed', 'cancelled', 'expired', 'failed')),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    payload jsonb NOT NULL,
    resource_id uuid,
    error text,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (message_id)
);

ALTER TABLE session_messages ADD COLUMN resource_creation_action_id uuid REFERENCES resource_creation_actions(id) ON DELETE SET NULL;
CREATE INDEX resource_creation_actions_owner_state ON resource_creation_actions(owner_user_id, state, expires_at);
