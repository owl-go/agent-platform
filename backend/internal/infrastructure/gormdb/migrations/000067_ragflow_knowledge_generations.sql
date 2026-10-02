-- Provider mappings are trusted infrastructure metadata, never authorization keys.
CREATE TABLE knowledge_provider_revisions (
    deployment_id text NOT NULL,
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    revision_id uuid NOT NULL REFERENCES knowledge_document_revisions(id) ON DELETE CASCADE,
    dataset_id text NOT NULL,
    document_id text NOT NULL,
    PRIMARY KEY (deployment_id, revision_id),
    UNIQUE (deployment_id, dataset_id, document_id)
);

-- Empty/missing manifests are never interpreted as an entire provider dataset.
CREATE TABLE knowledge_generation_revisions (
    generation_id uuid NOT NULL REFERENCES knowledge_index_generations(id) ON DELETE CASCADE,
    revision_id uuid NOT NULL REFERENCES knowledge_document_revisions(id) ON DELETE CASCADE,
    PRIMARY KEY (generation_id, revision_id)
);

-- Existing history predates verified RAGFlow indexing. Preserve source and
-- revision history, and reindex active sources before permitting new retrieval.
UPDATE knowledge_index_generations SET state = 'failed' WHERE state = 'ready';
UPDATE knowledge_document_revisions revision SET state = 'accepted', ready_at = NULL, error = ''
FROM knowledge_documents document JOIN knowledge_bases base ON base.id = document.knowledge_base_id
WHERE revision.document_id = document.id AND document.deleted_at IS NULL AND base.deleted_at IS NULL
  AND revision.state = 'ready'
  AND revision.revision = (SELECT MAX(candidate.revision) FROM knowledge_document_revisions candidate WHERE candidate.document_id = document.id AND candidate.state = 'ready');
UPDATE knowledge_document_revisions SET state = 'blocked', error = 'Historical index is unavailable in the RAGFlow deployment', ready_at = NULL WHERE state = 'ready';
INSERT INTO knowledge_ingestion_jobs(revision_id, idempotency_key, state)
SELECT revision.id, 'knowledge-revision:' || revision.id, 'queued'
FROM knowledge_document_revisions revision JOIN knowledge_documents document ON document.id = revision.document_id
JOIN knowledge_bases base ON base.id = document.knowledge_base_id
WHERE revision.state = 'accepted' AND document.deleted_at IS NULL AND base.deleted_at IS NULL
ON CONFLICT (idempotency_key) DO UPDATE SET state = 'queued', attempts = 0, error = '', lease_expires_at = NULL, next_attempt_at = now(), updated_at = now();
UPDATE knowledge_documents document SET state = 'accepted', error = '', version = version + 1
WHERE document.deleted_at IS NULL AND EXISTS (SELECT 1 FROM knowledge_document_revisions revision WHERE revision.document_id = document.id AND revision.state = 'accepted');
