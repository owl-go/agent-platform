ALTER TABLE runs
    ADD COLUMN conversation_id uuid,
    ADD COLUMN turn_number integer;

UPDATE runs
SET conversation_id = id,
    turn_number = 1;

ALTER TABLE runs
    ALTER COLUMN conversation_id SET NOT NULL,
    ALTER COLUMN turn_number SET NOT NULL,
    ADD CONSTRAINT runs_conversation_root_fk
        FOREIGN KEY (conversation_id) REFERENCES runs(id) ON DELETE CASCADE,
    ADD CONSTRAINT runs_turn_number_valid CHECK (turn_number > 0),
    ADD CONSTRAINT runs_conversation_turn_unique UNIQUE (conversation_id, turn_number);

CREATE INDEX runs_conversation_order ON runs (owner_user_id, conversation_id, turn_number);
