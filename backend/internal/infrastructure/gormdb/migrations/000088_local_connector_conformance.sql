-- Personal runc evidence must never satisfy production runsc verification.
CREATE TABLE local_cli_connector_conformance (
    bundle_sha256 text NOT NULL CHECK (bundle_sha256 ~ '^[0-9a-f]{64}$'),
    runtime_image_id text NOT NULL CHECK (runtime_image_id ~ '^sha256:[0-9a-f]{64}$'),
    passed boolean NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bundle_sha256, runtime_image_id)
);

-- Preserve Administrator disablement across seed upgrades and local retries.
-- Only untouched, managed first-install CLI publications are opted in.
ALTER TABLE connector_package_publications
    ADD COLUMN local_activation_pending boolean NOT NULL DEFAULT false;
UPDATE connector_package_publications p SET local_activation_pending=true
FROM default_resource_seeds seed, connector_revisions revision
WHERE seed.kind='connector' AND seed.resource_key=p.package_source
  AND seed.resource_id=p.active_revision_id::text AND seed.managed=true
  AND revision.id=p.active_revision_id AND revision.mode='cli'
  AND p.state='disabled' AND p.version=1;
