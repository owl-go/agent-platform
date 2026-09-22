ALTER TABLE smart_assistants
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN prompt text NOT NULL DEFAULT '',
    ADD COLUMN preprocess_prompt text NOT NULL DEFAULT '';

UPDATE smart_assistants
SET description = service_goal
WHERE description = '' AND service_goal <> '';

UPDATE smart_assistants
SET prompt = operating_rules
WHERE prompt = '' AND operating_rules <> '';

UPDATE smart_assistants
SET icon = ''
WHERE icon IN ('sparkles', 'compass', 'code', 'terminal', 'users', 'team');
