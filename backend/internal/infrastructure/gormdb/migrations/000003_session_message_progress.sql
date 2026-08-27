ALTER TABLE session_messages
    ADD COLUMN progress_stage text NOT NULL DEFAULT '';

ALTER TABLE session_messages
    ADD CONSTRAINT session_messages_progress_stage_valid
    CHECK (progress_stage IN ('', 'preparing', 'thinking', 'using_tool', 'working', 'responding'));
