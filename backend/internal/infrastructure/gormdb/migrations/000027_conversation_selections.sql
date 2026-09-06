CREATE TABLE conversation_selections (
    id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id uuid REFERENCES sessions(id) ON DELETE CASCADE,
    run_id uuid REFERENCES runs(id) ON DELETE CASCADE,
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((session_id IS NOT NULL)::integer + (run_id IS NOT NULL)::integer = 1)
);
CREATE INDEX conversation_selections_session ON conversation_selections(owner_user_id, session_id);
CREATE INDEX conversation_selections_run ON conversation_selections(owner_user_id, run_id);
ALTER TABLE sessions ADD COLUMN selection_id uuid;
ALTER TABLE runs ADD COLUMN selection_id uuid;
