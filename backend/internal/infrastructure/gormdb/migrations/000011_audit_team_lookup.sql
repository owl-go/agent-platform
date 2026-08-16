CREATE INDEX audit_events_team_lookup_index
    ON audit_events (organization_id, team_id, created_at DESC, id DESC);
