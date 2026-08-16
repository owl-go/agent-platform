ALTER TABLE agent_releases
    ADD CONSTRAINT agent_releases_source_draft_unique UNIQUE (source_draft_id);

CREATE TABLE agent_release_approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL UNIQUE REFERENCES agent_drafts(id) ON DELETE CASCADE,
    requested_by uuid NOT NULL REFERENCES users(id),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'rejected')),
    requested_at timestamptz NOT NULL DEFAULT now(),
    decided_by uuid REFERENCES users(id),
    decided_at timestamptz,
    reason text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((state = 'pending' AND decided_by IS NULL AND decided_at IS NULL) OR
           (state IN ('approved', 'rejected') AND decided_by IS NOT NULL AND decided_at IS NOT NULL)),
    CHECK (decided_by IS NULL OR decided_by <> requested_by)
);
