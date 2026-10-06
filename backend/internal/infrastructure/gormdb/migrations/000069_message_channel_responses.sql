-- Provider handles survive Worker restart; no credentials or answer text here.
ALTER TABLE message_channel_inbox ADD COLUMN response jsonb NOT NULL DEFAULT '{}';
ALTER TABLE message_channel_inbox ADD COLUMN response_revision bigint NOT NULL DEFAULT 0;
