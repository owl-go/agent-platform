CREATE UNIQUE INDEX approvals_one_pending_per_run
    ON approvals (run_id)
    WHERE state = 'pending';

CREATE INDEX approvals_run_timeline
    ON approvals (run_id, requested_at, id);
