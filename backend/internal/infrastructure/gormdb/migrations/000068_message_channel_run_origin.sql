ALTER TABLE runs ADD COLUMN message_channel_provider text NOT NULL DEFAULT '';

-- Provider is immutable on a channel. Preserve the origin of existing Runs,
-- including those whose channel has been soft-deleted.
UPDATE runs AS r
SET message_channel_provider = c.provider
FROM workflow_message_channels AS c
WHERE r.message_channel_id = c.id;
