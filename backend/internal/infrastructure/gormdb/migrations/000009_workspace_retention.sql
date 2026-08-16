ALTER TABLE sessions
    ADD COLUMN workspace_purged_at timestamptz;

CREATE INDEX sessions_workspace_retention_index
    ON sessions (workspace_purged_at, id)
    WHERE workspace_purged_at IS NULL;

CREATE INDEX coding_tasks_completion_index
    ON coding_tasks (completed_at, id)
    WHERE completed_at IS NOT NULL;

CREATE INDEX artifacts_workspace_snapshot_index
    ON artifacts (run_id, created_at, id)
    WHERE kind = 'workspace_snapshot' AND deleted_at IS NULL;
