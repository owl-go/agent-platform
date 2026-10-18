ALTER TABLE experts ADD COLUMN starter_prompts jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(starter_prompts)='array' AND jsonb_array_length(starter_prompts)<=3);
ALTER TABLE expert_teams ADD COLUMN starter_prompts jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(starter_prompts)='array' AND jsonb_array_length(starter_prompts)<=3);
