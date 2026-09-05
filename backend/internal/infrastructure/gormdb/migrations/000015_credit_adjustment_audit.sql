ALTER TABLE credit_ledger
    ADD COLUMN actor_user_id uuid REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX credit_ledger_actor_history
    ON credit_ledger (actor_user_id, created_at DESC)
    WHERE actor_user_id IS NOT NULL;
