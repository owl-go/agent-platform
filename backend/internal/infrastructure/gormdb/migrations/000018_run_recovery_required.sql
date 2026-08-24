ALTER TABLE runs DROP CONSTRAINT runs_state_check;
ALTER TABLE runs ADD CONSTRAINT runs_state_check CHECK (state IN (
    'queued', 'provisioning', 'running', 'waiting_confirmation', 'interrupting',
    'interrupted', 'resuming', 'recovery_required', 'completed', 'failed', 'cancelled'
));
