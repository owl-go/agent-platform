CREATE TABLE smart_assistant_sessions (
    session_id uuid PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    assistant_id uuid NOT NULL REFERENCES smart_assistants(id) ON DELETE RESTRICT,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assistant_snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX smart_assistant_sessions_assistant ON smart_assistant_sessions(owner_user_id, assistant_id, created_at DESC);
