ALTER TABLE knowledge_chunks
    ADD COLUMN IF NOT EXISTS generation_id uuid REFERENCES knowledge_index_generations(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS knowledge_chunks_generation
    ON knowledge_chunks(generation_id, document_id, position);

CREATE TABLE IF NOT EXISTS ai_application_knowledge_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id uuid NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    state text NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ai_application_knowledge_jobs_pending
    ON ai_application_knowledge_jobs(document_id)
    WHERE state IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS ai_application_knowledge_jobs_queue
    ON ai_application_knowledge_jobs(state, created_at, id);
