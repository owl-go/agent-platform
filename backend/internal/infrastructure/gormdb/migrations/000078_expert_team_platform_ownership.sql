-- Definition deletion is intentional. Historical messages, accepted inputs,
-- conversation selection revisions, Run snapshots, and Credit records stay private.
CREATE TEMP TABLE removed_private_teams ON COMMIT DROP AS
SELECT team.id FROM expert_teams team JOIN users owner ON owner.id=team.owner_user_id WHERE NOT owner.administrator;

-- Keep identities only, so stale mutable selections cannot restore definitions.
CREATE TABLE retired_private_expert_teams (
    team_id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL,
    retired_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO retired_private_expert_teams(team_id,owner_user_id)
SELECT team.id,team.owner_user_id FROM expert_teams team JOIN removed_private_teams removed ON removed.id=team.id;

UPDATE sessions SET expert_team_id=NULL, expert_snapshot=NULL, selection_id=NULL
WHERE expert_team_id IN (SELECT id FROM removed_private_teams)
   OR selection_id IN (SELECT selection.id FROM conversation_selections selection WHERE selection.snapshot->>'expert_team_id' IN (SELECT id::text FROM removed_private_teams));
UPDATE workflows SET expert_team_id=NULL, execution_template=NULL
WHERE expert_team_id IN (SELECT id FROM removed_private_teams);
UPDATE runs SET selection_id=NULL
WHERE selection_id IN (SELECT selection.id FROM conversation_selections selection WHERE selection.snapshot->>'expert_team_id' IN (SELECT id::text FROM removed_private_teams));
UPDATE smart_assistants SET expert_team_id=NULL WHERE expert_team_id IN (SELECT id FROM removed_private_teams);
DELETE FROM expert_teams WHERE id IN (SELECT id FROM removed_private_teams);

ALTER TABLE expert_teams ADD COLUMN system_key text;
ALTER TABLE expert_teams ADD COLUMN system_managed boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX expert_teams_system_key_unique ON expert_teams(system_key) WHERE system_key IS NOT NULL;
