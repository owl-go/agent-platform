ALTER TABLE approvals
    ADD COLUMN requested_by uuid REFERENCES users(id);

UPDATE approvals AS approval
SET requested_by = (
    SELECT event.actor_user_id
    FROM audit_events AS event
    WHERE event.action = 'approval.request'
      AND event.resource_type = 'approval'
      AND event.resource_id = approval.id::text
      AND event.actor_user_id IS NOT NULL
    ORDER BY event.created_at, event.id
    LIMIT 1
)
WHERE approval.requested_by IS NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM approvals WHERE requested_by IS NULL) THEN
        RAISE EXCEPTION 'Run Approval requester backfill is incomplete; provide auditable actor evidence before applying migration 000016';
    END IF;
END $$;

ALTER TABLE approvals
    ALTER COLUMN requested_by SET NOT NULL;

CREATE INDEX approvals_requester_lookup
    ON approvals (requested_by, requested_at);
