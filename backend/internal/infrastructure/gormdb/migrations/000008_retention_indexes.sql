CREATE INDEX run_events_retention_index
    ON run_events (created_at, id);

CREATE INDEX artifacts_retention_index
    ON artifacts (COALESCE(expires_at, created_at), id)
    WHERE deleted_at IS NULL;

CREATE INDEX audit_events_retention_index
    ON audit_events (created_at, id);

CREATE INDEX idempotency_keys_retention_index
    ON idempotency_keys (expires_at);
