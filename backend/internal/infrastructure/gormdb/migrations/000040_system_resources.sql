ALTER TABLE skills
    ADD COLUMN system_key text,
    ADD COLUMN system_managed boolean NOT NULL DEFAULT false,
    ADD COLUMN name_normalized text NOT NULL DEFAULT '';

ALTER TABLE experts
    ADD COLUMN system_key text,
    ADD COLUMN system_managed boolean NOT NULL DEFAULT false,
    ADD COLUMN name_normalized text NOT NULL DEFAULT '';

UPDATE skills SET name_normalized = lower(trim(name)) WHERE name_normalized = '';
UPDATE experts SET name_normalized = lower(trim(name)) WHERE name_normalized = '';

CREATE UNIQUE INDEX skills_system_key_unique ON skills(system_key) WHERE system_key IS NOT NULL;
CREATE UNIQUE INDEX experts_system_key_unique ON experts(system_key) WHERE system_key IS NOT NULL;
CREATE UNIQUE INDEX skills_owner_normalized_name_unique ON skills(owner_user_id, name_normalized);
CREATE UNIQUE INDEX experts_owner_normalized_name_unique ON experts(owner_user_id, name_normalized);
