CREATE TABLE session_artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id bigint NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
    name text NOT NULL,
    path text NOT NULL,
    object_key text NOT NULL,
    text_result jsonb,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 text NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX session_artifacts_message
    ON session_artifacts (owner_user_id, session_id, message_id, created_at, id);
