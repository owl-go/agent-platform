ALTER TABLE expert_teams ADD COLUMN lead_member_id text NOT NULL DEFAULT '';

-- Retained Teams require explicit lead selection. Copy member definitions now,
-- without touching immutable historical execution snapshots or choosing a lead.
UPDATE expert_teams team SET members = COALESCE((
    SELECT jsonb_agg(member.value || jsonb_build_object('definition', jsonb_build_object(
      'name', expert.name, 'icon', expert.icon, 'icon_background', expert.icon_background,
      'introduction', expert.introduction, 'guidance', expert.guidance,
      'mcp_server_ids', expert.mcp_server_ids, 'skill_ids', expert.skill_ids,
      'cli_connector_definition_ids', expert.cli_connector_definition_ids
    )) ORDER BY member.position)
    FROM jsonb_array_elements(team.members) WITH ORDINALITY member(value,position)
    JOIN experts expert ON expert.id::text=member.value->>'expert_id'
), '[]'::jsonb);

CREATE TABLE expert_team_definition_revisions (
    team_id uuid NOT NULL,
    version bigint NOT NULL CHECK(version>0),
    owner_user_id uuid NOT NULL,
    definition jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(team_id,version)
);
INSERT INTO expert_team_definition_revisions(team_id,version,owner_user_id,definition)
SELECT id,version,owner_user_id,to_jsonb(expert_teams) FROM expert_teams;
CREATE FUNCTION preserve_team_definition_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' AND NEW.version=OLD.version THEN RETURN NEW; END IF;
    INSERT INTO expert_team_definition_revisions(team_id,version,owner_user_id,definition)
    VALUES(NEW.id,NEW.version,NEW.owner_user_id,to_jsonb(NEW));
    RETURN NEW;
END;
$$;
CREATE TRIGGER team_definition_revision AFTER INSERT OR UPDATE ON expert_teams
FOR EACH ROW EXECUTE FUNCTION preserve_team_definition_revision();
CREATE TRIGGER immutable_team_definition_revision BEFORE UPDATE OR DELETE ON expert_team_definition_revisions
FOR EACH ROW EXECUTE FUNCTION reject_definition_revision_mutation();
