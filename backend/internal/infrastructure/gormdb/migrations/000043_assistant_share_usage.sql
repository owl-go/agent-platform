CREATE TABLE smart_assistant_share_usage (
    assistant_id uuid NOT NULL REFERENCES smart_assistants(id) ON DELETE CASCADE,
    usage_day date NOT NULL,
    calls integer NOT NULL DEFAULT 0 CHECK (calls >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (assistant_id, usage_day)
);
