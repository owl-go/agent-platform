DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM agent_releases
        WHERE length(btrim(repository_binding_snapshot->>'source_control_provider_id')) = 0
           OR repository_binding_snapshot->>'source_control_provider_id' IS NULL
           OR length(btrim(repository_binding_snapshot->>'ssh_credential_profile_id')) = 0
           OR repository_binding_snapshot->>'ssh_credential_profile_id' IS NULL
           OR jsonb_typeof(repository_binding_snapshot->'build_credential_profile_ids') <> 'array'
           OR repository_binding_snapshot->'build_credential_profile_ids' IS NULL
           OR length(btrim(repository_binding_snapshot->>'git_author_name')) = 0
           OR repository_binding_snapshot->>'git_author_name' IS NULL
           OR length(btrim(repository_binding_snapshot->>'git_author_email')) = 0
           OR repository_binding_snapshot->>'git_author_email' IS NULL
           OR length(btrim(configured_model_snapshot->>'credential_profile_id')) = 0
           OR configured_model_snapshot->>'credential_profile_id' IS NULL
    ) THEN
        RAISE EXCEPTION 'existing Agent Releases require audited credential snapshot backfill before they can launch Runs';
    END IF;
END $$;

ALTER TABLE agent_releases
    ADD CONSTRAINT agent_releases_credential_snapshot_check CHECK (
        COALESCE(length(btrim(repository_binding_snapshot->>'source_control_provider_id')) > 0, false)
        AND COALESCE(length(btrim(repository_binding_snapshot->>'ssh_credential_profile_id')) > 0, false)
        AND COALESCE(jsonb_typeof(repository_binding_snapshot->'build_credential_profile_ids') = 'array', false)
        AND COALESCE(length(btrim(repository_binding_snapshot->>'git_author_name')) > 0, false)
        AND COALESCE(length(btrim(repository_binding_snapshot->>'git_author_email')) > 0, false)
        AND COALESCE(length(btrim(configured_model_snapshot->>'credential_profile_id')) > 0, false)
    );
