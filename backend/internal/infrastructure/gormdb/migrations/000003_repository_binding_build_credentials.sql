ALTER TABLE repository_bindings
    ADD COLUMN build_credential_profile_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
