-- Deploy this migration atomically with the Worker version that no longer reads
-- credit_execution_leases. Model-stage admissions reserve the frozen fallback
-- before provider execution.
-- Settled rows remain historical and are excluded from the active reservation sum.
ALTER TABLE credit_stage_admissions
    ADD COLUMN reserved_hundredths bigint NOT NULL DEFAULT 0 CHECK (reserved_hundredths >= 0),
    ADD COLUMN daily_reserved_hundredths bigint NOT NULL DEFAULT 0 CHECK (daily_reserved_hundredths >= 0),
    ADD COLUMN persistent_reserved_hundredths bigint NOT NULL DEFAULT 0 CHECK (persistent_reserved_hundredths >= 0),
    ADD CONSTRAINT credit_stage_admission_reservation_split
        CHECK (reserved_hundredths = daily_reserved_hundredths + persistent_reserved_hundredths);

CREATE INDEX credit_stage_admissions_active_user
    ON credit_stage_admissions (user_id, credit_day)
    WHERE settled_at IS NULL;

DROP TABLE credit_execution_leases;
