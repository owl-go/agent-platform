ALTER TABLE session_messages
    ADD COLUMN cancel_requested_at timestamptz;
