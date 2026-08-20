ALTER TABLE agent_release_approvals
    ADD COLUMN IF NOT EXISTS risk_reason text;

ALTER TABLE agent_releases
    ADD COLUMN IF NOT EXISTS release_risk text,
    ADD COLUMN IF NOT EXISTS repository_binding_snapshot jsonb,
    ADD COLUMN IF NOT EXISTS runtime_image_snapshot jsonb,
    ADD COLUMN IF NOT EXISTS configured_model_snapshot jsonb,
    ADD COLUMN IF NOT EXISTS approval_evidence jsonb,
    ADD COLUMN IF NOT EXISTS blocked_reason text;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM agent_release_approvals
        WHERE risk_reason IS NULL OR length(btrim(risk_reason)) = 0
    ) THEN
        RAISE EXCEPTION 'existing Release Approvals require the documented audited risk reason backfill';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM agent_releases
        WHERE release_risk IS NULL
           OR repository_binding_snapshot IS NULL
           OR runtime_image_snapshot IS NULL
           OR configured_model_snapshot IS NULL
           OR release_risk = 'high' AND approval_evidence IS NULL
           OR release_risk = 'low' AND approval_evidence IS NOT NULL
           OR status = 'blocked' AND (blocked_reason IS NULL OR length(btrim(blocked_reason)) = 0)
           OR status <> 'blocked' AND blocked_reason IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'existing Agent Releases require the documented audited immutable snapshot backfill';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM agent_releases release
        WHERE release.release_risk = 'high'
          AND NOT EXISTS (
              SELECT 1
              FROM agent_release_approvals approval
              WHERE approval.id = (release.approval_evidence->>'id')::uuid
                AND approval.draft_id = release.source_draft_id
                AND approval.draft_version = (release.approval_evidence->>'draft_version')::bigint
                AND approval.requested_by::text = release.approval_evidence->>'requested_by'
                AND approval.risk_reason = release.approval_evidence->>'risk_reason'
                AND approval.state = 'approved'
                AND approval.decided_by::text = release.approval_evidence->>'approved_by'
                AND approval.decided_at = (release.approval_evidence->>'approved_at')::timestamptz
          )
    ) THEN
        RAISE EXCEPTION 'existing high-risk Agent Releases require exact audited Release Approval evidence';
    END IF;
END $$;

ALTER TABLE agent_release_approvals
    ALTER COLUMN risk_reason SET NOT NULL,
    ADD CONSTRAINT agent_release_approvals_risk_reason_check CHECK (length(btrim(risk_reason)) > 0);

ALTER TABLE agent_releases
    ALTER COLUMN release_risk SET NOT NULL,
    ALTER COLUMN repository_binding_snapshot SET NOT NULL,
    ALTER COLUMN runtime_image_snapshot SET NOT NULL,
    ALTER COLUMN configured_model_snapshot SET NOT NULL,
    ADD CONSTRAINT agent_releases_release_risk_check CHECK (release_risk IN ('low', 'high')),
    ADD CONSTRAINT agent_releases_dependency_snapshot_check CHECK (
        jsonb_typeof(repository_binding_snapshot) = 'object'
        AND jsonb_typeof(runtime_image_snapshot) = 'object'
        AND jsonb_typeof(configured_model_snapshot) = 'object'
    ),
    ADD CONSTRAINT agent_releases_approval_evidence_check CHECK (
        (release_risk = 'low' AND approval_evidence IS NULL) OR
        (release_risk = 'high' AND approval_evidence IS NOT NULL AND jsonb_typeof(approval_evidence) = 'object')
    ),
    ADD CONSTRAINT agent_releases_blocked_reason_check CHECK (
        (status = 'blocked' AND blocked_reason IS NOT NULL AND length(btrim(blocked_reason)) > 0) OR
        (status <> 'blocked' AND blocked_reason IS NULL)
    );
