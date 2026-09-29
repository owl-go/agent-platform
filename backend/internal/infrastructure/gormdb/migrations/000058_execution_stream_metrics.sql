ALTER TABLE product_events DROP CONSTRAINT IF EXISTS product_events_name_check;

ALTER TABLE product_events ADD CONSTRAINT product_events_name_check CHECK (name IN (
    'login_completed',
    'default_execution_ready',
    'first_task_started',
    'first_response_succeeded',
    'first_response_failed',
    'workflow_save_started',
    'workflow_created',
    'workflow_second_run_succeeded',
    'execution_stream_reconnected'
));
