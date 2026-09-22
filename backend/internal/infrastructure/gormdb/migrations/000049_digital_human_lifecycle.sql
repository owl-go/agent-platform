ALTER TABLE digital_humans
    ADD COLUMN IF NOT EXISTS state text NOT NULL DEFAULT 'enabled';

ALTER TABLE digital_humans
    DROP CONSTRAINT IF EXISTS digital_humans_state_check;

ALTER TABLE digital_humans
    ADD CONSTRAINT digital_humans_state_check CHECK (state IN ('enabled', 'disabled'));
