ALTER TABLE expert_teams ADD COLUMN icon text NOT NULL DEFAULT 'users';
ALTER TABLE expert_teams ADD COLUMN icon_background text NOT NULL DEFAULT 'sage';
ALTER TABLE expert_teams ADD COLUMN introduction text NOT NULL DEFAULT '';
ALTER TABLE expert_teams ADD COLUMN core_capability text NOT NULL DEFAULT '';
ALTER TABLE expert_teams ADD COLUMN members jsonb NOT NULL DEFAULT '[]'::jsonb;

UPDATE expert_teams
SET introduction = capability_introduction,
    members = COALESCE((
      SELECT jsonb_agg(jsonb_build_object(
        'id', md5(expert_teams.id || ':' || member.ordinality::text),
        'name', COALESCE(experts.name, 'Member ' || member.ordinality::text),
        'expert_id', member.expert_id,
        'labels', '[]'::jsonb
      ) ORDER BY member.ordinality)
      FROM jsonb_array_elements_text(expert_teams.expert_ids) WITH ORDINALITY AS member(expert_id, ordinality)
      LEFT JOIN experts ON experts.id::text = member.expert_id
    ), '[]'::jsonb);
