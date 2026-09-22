ALTER TABLE smart_assistants
    DROP CONSTRAINT IF EXISTS smart_assistants_state_check;

ALTER TABLE smart_assistants
    ADD CONSTRAINT smart_assistants_state_check CHECK (state IN ('draft', 'enabled', 'disabled'));

ALTER TABLE smart_assistants
    ALTER COLUMN state SET DEFAULT 'draft';
