CREATE TABLE product_events (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (name IN (
        'login_completed',
        'default_execution_ready',
        'first_task_started',
        'first_response_succeeded',
        'first_response_failed',
        'workflow_save_started',
        'workflow_created',
        'workflow_second_run_succeeded'
    )),
    anonymous_user_key char(64) NOT NULL CHECK (anonymous_user_key ~ '^[0-9a-f]{64}$'),
    anonymous_subject_key char(64) CHECK (anonymous_subject_key IS NULL OR anonymous_subject_key ~ '^[0-9a-f]{64}$'),
    object_type text NOT NULL CHECK (object_type IN ('account', 'session', 'workflow', 'run')),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    dedup_key char(64) NOT NULL UNIQUE CHECK (dedup_key ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX product_events_name_occurred_at ON product_events(name, occurred_at DESC);
CREATE INDEX product_events_user_occurred_at ON product_events(anonymous_user_key, occurred_at DESC);
CREATE INDEX product_events_subject_occurred_at ON product_events(anonymous_subject_key, occurred_at DESC) WHERE anonymous_subject_key IS NOT NULL;

COMMENT ON TABLE product_events IS 'Privacy-bounded product analytics. Never store prompts, replies, filenames, object keys, external accounts, credentials, or signed URLs.';
