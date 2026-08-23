ALTER TABLE approvals
    ADD COLUMN decision_actor_type text;

UPDATE approvals
SET decision_actor_type = 'user'
WHERE state <> 'pending';

ALTER TABLE approvals
    ADD CONSTRAINT approvals_decision_actor_type_check CHECK (
        (state = 'pending' AND decision_actor_type IS NULL AND decided_by IS NULL)
        OR
        (state <> 'pending' AND decided_at IS NOT NULL AND (
            (decision_actor_type = 'user' AND decided_by IS NOT NULL)
            OR (decision_actor_type = 'system' AND decided_by IS NULL)
        ))
    );
