CREATE TABLE expert_package_imports (
    owner_user_id uuid NOT NULL REFERENCES users(id),
    package_id text NOT NULL,
    package_version text NOT NULL,
    content_sha256 text NOT NULL CHECK(content_sha256 ~ '^[0-9a-f]{64}$'),
    kind text NOT NULL CHECK(kind IN ('expert','expert_team')),
    resource_id uuid NOT NULL,
    resource_version bigint NOT NULL CHECK(resource_version>0),
    archive bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(owner_user_id,package_id,package_version)
);
