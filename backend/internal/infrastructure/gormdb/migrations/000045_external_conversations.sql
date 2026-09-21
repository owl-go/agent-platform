ALTER TABLE sessions ADD COLUMN external boolean NOT NULL DEFAULT false;
CREATE INDEX sessions_external_owner_state ON sessions (owner_user_id, external, archived_at, updated_at DESC);

CREATE TABLE external_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    assistant_id uuid NOT NULL REFERENCES smart_assistants(id) ON DELETE CASCADE,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    execution_session_id uuid NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
    visitor_hash text NOT NULL,
    assistant_snapshot jsonb NOT NULL,
    share_token_revision bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX external_conversations_visitor ON external_conversations (visitor_hash, updated_at DESC);

CREATE TABLE external_conversation_responses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES external_conversations(id) ON DELETE CASCADE,
    assistant_message_id bigint NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, assistant_message_id)
);
