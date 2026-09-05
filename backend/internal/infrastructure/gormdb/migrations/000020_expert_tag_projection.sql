ALTER TABLE experts ADD COLUMN tag_projection_status text NOT NULL DEFAULT 'idle' CHECK (tag_projection_status IN ('idle', 'queued', 'running', 'succeeded', 'failed'));
ALTER TABLE experts ADD COLUMN tag_projection_error text;
ALTER TABLE experts ADD COLUMN tag_projection_requested_at timestamptz;

UPDATE experts SET tag_projection_status = 'queued', tag_projection_requested_at = now() WHERE core_capability <> '';
CREATE INDEX experts_tag_projection_queue ON experts (tag_projection_requested_at, id) WHERE tag_projection_status = 'queued';
