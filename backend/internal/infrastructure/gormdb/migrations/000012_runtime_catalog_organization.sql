ALTER TABLE runtime_images
    ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations(id),
    ADD COLUMN IF NOT EXISTS conformance_evidence_key text,
    ADD COLUMN IF NOT EXISTS conformance_evidence_sha256 text;

UPDATE runtime_images
SET organization_id = (SELECT id FROM organizations ORDER BY id LIMIT 1)
WHERE organization_id IS NULL
  AND (SELECT count(*) FROM organizations) = 1;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM runtime_images WHERE organization_id IS NULL) THEN
        RAISE EXCEPTION 'cannot infer Runtime Image Organization; apply the documented runtime catalog pre-migration backfill';
    END IF;
END $$;

ALTER TABLE runtime_images ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE runtime_images DROP CONSTRAINT runtime_images_image_digest_key;
ALTER TABLE runtime_images ADD CONSTRAINT runtime_images_organization_digest_key UNIQUE (organization_id, image_digest);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM runtime_images
        WHERE status = 'production'
          AND (conformance_evidence_key IS NULL OR conformance_evidence_sha256 IS NULL)
    ) THEN
        RAISE EXCEPTION 'existing Production Runtime Images require the documented verified Conformance evidence backfill';
    END IF;
END $$;

ALTER TABLE runtime_images ADD CONSTRAINT runtime_images_conformance_evidence_check CHECK (
    status <> 'production' OR (
        conformance_evidence_key IS NOT NULL
        AND length(conformance_evidence_key) BETWEEN 1 AND 512
        AND conformance_evidence_key !~ '(^/|://|\.\.)'
        AND conformance_evidence_sha256 ~ '^[a-f0-9]{64}$'
    )
);

ALTER TABLE runtime_images ADD CONSTRAINT runtime_images_conformance_evidence_pair_check CHECK (
    (conformance_evidence_key IS NULL) = (conformance_evidence_sha256 IS NULL)
);

CREATE INDEX runtime_images_organization_list_idx
    ON runtime_images (organization_id, runtime, created_at DESC, id);
