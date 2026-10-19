-- Docker's containerd image store can expose a multi-platform index as Image
-- ID. A copied data volume must not reuse ARM evidence on an AMD Worker.
ALTER TABLE local_cli_connector_conformance
    ADD COLUMN worker_architecture text NOT NULL DEFAULT ''
    CHECK (worker_architecture IN ('', 'arm64', 'amd64'));
ALTER TABLE local_cli_connector_conformance
    DROP CONSTRAINT local_cli_connector_conformance_pkey;
ALTER TABLE local_cli_connector_conformance
    ADD PRIMARY KEY (bundle_sha256, runtime_image_id, worker_architecture);
-- Older rows deliberately remain unscoped and cannot satisfy new queries.
