ALTER TABLE session_messages
    ADD COLUMN attachments jsonb NOT NULL DEFAULT '[]'::jsonb;
