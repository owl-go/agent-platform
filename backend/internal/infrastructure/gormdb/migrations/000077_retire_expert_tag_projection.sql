-- Stop obsolete asynchronous work; preserve historical columns for readers.
UPDATE experts SET tag_projection_status = 'idle', tag_projection_requested_at = NULL, tag_projection_error = NULL;
DROP INDEX experts_tag_projection_queue;
