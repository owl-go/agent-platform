UPDATE smart_assistant_sessions
SET assistant_snapshot = assistant_snapshot - 'digital_human_id'
WHERE assistant_snapshot ? 'digital_human_id';

UPDATE assistant_conversations
SET assistant_snapshot = assistant_snapshot - 'digital_human_id'
WHERE assistant_snapshot ? 'digital_human_id';

UPDATE external_conversations
SET assistant_snapshot = assistant_snapshot - 'digital_human_id'
WHERE assistant_snapshot ? 'digital_human_id';

ALTER TABLE smart_assistants
    DROP COLUMN IF EXISTS digital_human_id;

DROP TABLE IF EXISTS digital_humans;
