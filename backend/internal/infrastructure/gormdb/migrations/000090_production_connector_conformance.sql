-- Startup verification of distributed managed packages is independent of
-- legacy CLI Definitions and can never consume local runc evidence.
CREATE TABLE production_cli_connector_conformance (
    bundle_sha256 text NOT NULL CHECK (bundle_sha256 ~ '^[0-9a-f]{64}$'),
    runtime_repo_digest text NOT NULL CHECK (runtime_repo_digest ~ '^sha256:[0-9a-f]{64}$'),
    worker_architecture text NOT NULL CHECK (worker_architecture IN ('arm64', 'amd64')),
    passed boolean NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bundle_sha256, runtime_repo_digest, worker_architecture)
);
