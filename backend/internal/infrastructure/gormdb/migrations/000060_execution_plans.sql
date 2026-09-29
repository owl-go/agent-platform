ALTER TABLE session_messages
    ADD COLUMN execution_plan jsonb,
    ADD CONSTRAINT session_messages_execution_plan_object CHECK (execution_plan IS NULL OR jsonb_typeof(execution_plan) = 'object');

ALTER TABLE runs
    ADD COLUMN execution_plan jsonb,
    ADD CONSTRAINT runs_execution_plan_object CHECK (execution_plan IS NULL OR jsonb_typeof(execution_plan) = 'object');

COMMENT ON COLUMN session_messages.execution_plan IS 'Frozen conditional Execution Plan and user-visible step state; no private reasoning.';
COMMENT ON COLUMN runs.execution_plan IS 'Frozen conditional Execution Plan and user-visible step state; no private reasoning.';
