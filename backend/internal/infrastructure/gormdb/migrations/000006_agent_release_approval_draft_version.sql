ALTER TABLE agent_release_approvals
    ADD COLUMN draft_version bigint NOT NULL DEFAULT 1 CHECK (draft_version > 0);

ALTER TABLE agent_release_approvals
    DROP CONSTRAINT agent_release_approvals_draft_id_key,
    ADD CONSTRAINT agent_release_approvals_draft_version_unique UNIQUE (draft_id, draft_version);
