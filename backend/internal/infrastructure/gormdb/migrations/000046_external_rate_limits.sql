CREATE TABLE external_rate_limit_buckets (
    scope text NOT NULL,
    scope_key text NOT NULL,
    bucket_start timestamptz NOT NULL,
    calls integer NOT NULL DEFAULT 0 CHECK (calls >= 0),
    PRIMARY KEY (scope, scope_key, bucket_start)
);
CREATE INDEX external_rate_limit_expiry ON external_rate_limit_buckets(bucket_start);
