CREATE TABLE ai_embedding_configurations (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    endpoint text NOT NULL,
    model text NOT NULL,
    dimensions integer NOT NULL CHECK (dimensions BETWEEN 1 AND 8192),
    api_key_ciphertext bytea NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);
