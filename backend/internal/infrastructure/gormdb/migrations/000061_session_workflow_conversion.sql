ALTER TABLE workflows
    ADD COLUMN execution_template jsonb,
    ADD CONSTRAINT workflows_execution_template_array CHECK (
        execution_template IS NULL OR jsonb_typeof(execution_template) = 'array'
    );

ALTER TABLE runs
    DROP CONSTRAINT runs_trigger_check,
    ADD CONSTRAINT runs_trigger_check CHECK (trigger IN ('manual', 'scheduled', 'api', 'session_conversion'));

CREATE TABLE workflow_session_origins (
    workflow_id uuid PRIMARY KEY REFERENCES workflows(id) ON DELETE CASCADE,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id bigint NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
    validation_run_id uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_user_id, session_id, message_id)
);

CREATE INDEX workflow_session_origins_session_idx
    ON workflow_session_origins(owner_user_id, session_id, created_at DESC);

COMMENT ON COLUMN workflows.execution_template IS 'Frozen, redacted execution stages copied from the successful Session response that created the Workflow.';
COMMENT ON TABLE workflow_session_origins IS 'Owner-scoped, idempotent link between a successful Session response and its converted Workflow plus first validation Run.';
