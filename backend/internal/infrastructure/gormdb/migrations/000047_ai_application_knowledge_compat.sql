-- AI Applications reuse the platform Knowledge Base tables. Add only the
-- bounded text/chunk columns needed by the assistant retrieval path; never
-- create a second Knowledge Base schema with the same table names.
ALTER TABLE knowledge_documents
    ADD COLUMN IF NOT EXISTS content text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS content_sha256 text NOT NULL DEFAULT repeat('0', 64);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id uuid NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS knowledge_chunks_document ON knowledge_chunks(document_id, position);
CREATE INDEX IF NOT EXISTS knowledge_chunks_fts ON knowledge_chunks USING gin (to_tsvector('simple', content));

DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
    ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS embedding vector(1536);
EXCEPTION WHEN feature_not_supported OR undefined_file OR insufficient_privilege THEN
    ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS embedding jsonb;
END $$;
