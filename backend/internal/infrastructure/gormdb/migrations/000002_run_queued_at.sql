UPDATE runs
SET queued_at = COALESCE(started_at, ended_at, now())
WHERE queued_at < TIMESTAMPTZ '2000-01-01 00:00:00+00';

ALTER TABLE runs
    ADD CONSTRAINT runs_queued_at_valid
    CHECK (queued_at >= TIMESTAMPTZ '2000-01-01 00:00:00+00');
