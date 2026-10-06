CREATE TABLE default_resource_seeds (
    kind text NOT NULL CHECK (kind IN ('skill', 'expert', 'connector')),
    resource_key text NOT NULL,
    resource_id text NOT NULL,
    catalog_version text NOT NULL,
    content_sha256 text NOT NULL CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    managed boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, resource_key)
);
