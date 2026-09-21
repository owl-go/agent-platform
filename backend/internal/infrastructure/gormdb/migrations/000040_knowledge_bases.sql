CREATE TABLE knowledge_bases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'ready' CHECK (state IN ('ready', 'failed', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (owner_user_id, name)
);

CREATE TABLE knowledge_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    name text NOT NULL,
    content text NOT NULL,
    content_sha256 text NOT NULL CHECK (content_sha256 ~ '^[a-f0-9]{64}$'),
    state text NOT NULL DEFAULT 'ready' CHECK (state IN ('ready', 'failed', 'disabled')),
    failure_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1
);

CREATE TABLE knowledge_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id uuid NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_chunks_fts ON knowledge_chunks USING gin (to_tsvector('simple', content));
CREATE INDEX knowledge_documents_base ON knowledge_documents(knowledge_base_id, updated_at DESC);

-- Keep local development usable with stock PostgreSQL while using pgvector
-- automatically when the extension is present in the deployment image.
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
    ALTER TABLE knowledge_chunks ADD COLUMN embedding vector(1536);
EXCEPTION WHEN feature_not_supported OR undefined_file OR insufficient_privilege THEN
    ALTER TABLE knowledge_chunks ADD COLUMN embedding jsonb;
END $$;
