ALTER TABLE resource_creation_actions DROP CONSTRAINT resource_creation_actions_kind_check;
ALTER TABLE resource_creation_actions ADD CONSTRAINT resource_creation_actions_kind_check CHECK (kind IN ('skill','expert','expert_team','connector'));
-- Pending previews from the retired authoring contract cannot become new writes.
UPDATE resource_creation_actions SET state='expired', version=version+1, updated_at=now() WHERE kind='expert' AND state IN ('pending','failed') AND NOT (payload->'expert' ? 'guidance');
