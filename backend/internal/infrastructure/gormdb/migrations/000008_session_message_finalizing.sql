ALTER TABLE session_messages
    DROP CONSTRAINT session_messages_progress_stage_valid;

ALTER TABLE session_messages
    ADD CONSTRAINT session_messages_progress_stage_valid
    CHECK (progress_stage IN ('', 'preparing', 'thinking', 'using_tool', 'working', 'responding', 'finalizing'));
