ALTER TABLE session_messages
    ADD COLUMN runtime_activities jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT session_message_runtime_activities_array
        CHECK (jsonb_typeof(runtime_activities) = 'array');
