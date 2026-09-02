ALTER TABLE experts
    RENAME COLUMN description TO capability_introduction;

ALTER TABLE experts
    ADD COLUMN execution_instruction text NOT NULL DEFAULT '',
    ADD COLUMN expertise_tags jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE expert_teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    capability_introduction text NOT NULL,
    expertise_tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    expert_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    UNIQUE (owner_user_id, name)
);

CREATE INDEX expert_teams_owner_updated
    ON expert_teams (owner_user_id, updated_at DESC, id DESC);

ALTER TABLE sessions
    ADD COLUMN expert_team_id uuid REFERENCES expert_teams(id) ON DELETE SET NULL,
    ADD CONSTRAINT sessions_one_expert_selection CHECK (expert_id IS NULL OR expert_team_id IS NULL);

ALTER TABLE workflows
    ADD COLUMN expert_team_id uuid REFERENCES expert_teams(id) ON DELETE SET NULL,
    ADD CONSTRAINT workflows_one_expert_selection CHECK (expert_id IS NULL OR expert_team_id IS NULL);

ALTER TABLE session_messages
    ADD COLUMN expert_stages jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE runs
    ADD COLUMN expert_stages jsonb NOT NULL DEFAULT '[]'::jsonb;
