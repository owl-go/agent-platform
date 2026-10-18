ALTER TABLE experts ADD COLUMN bundled_skills jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(bundled_skills) = 'array');
