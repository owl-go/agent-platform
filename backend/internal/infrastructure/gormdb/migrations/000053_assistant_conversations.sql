-- Assistant conversations are independent of ordinary Sessions. Historical
-- transcripts and frozen configuration survive deletion of the mutable app.
CREATE TABLE assistant_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL,
    assistant_id uuid NOT NULL,
    assistant_name text NOT NULL,
    welcome text NOT NULL DEFAULT '',
    assistant_snapshot jsonb NOT NULL,
    model_snapshot jsonb NOT NULL,
    summary text NOT NULL DEFAULT '',
    summary_through_turn integer NOT NULL DEFAULT 0,
    last_turn integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assistant_conversations_owner ON assistant_conversations(owner_user_id, assistant_id, created_at DESC);

CREATE TABLE assistant_conversation_turns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES assistant_conversations(id) ON DELETE RESTRICT,
    turn_number integer NOT NULL,
    question text NOT NULL,
    answer text NOT NULL DEFAULT '',
    source text NOT NULL DEFAULT '',
    faq_id uuid,
    state text NOT NULL CHECK (state IN ('generating', 'completed', 'failed', 'cancelled')),
    cancel_requested_at timestamptz,
    input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (conversation_id, turn_number)
);

CREATE UNIQUE INDEX assistant_conversation_active_turn ON assistant_conversation_turns(conversation_id) WHERE state = 'generating';
CREATE INDEX assistant_conversation_turns_history ON assistant_conversation_turns(conversation_id, turn_number);
