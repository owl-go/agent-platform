-- Preserve legacy Assistant text sources while moving all new indexing to
-- immutable Knowledge Document Revisions and the shared retrieval boundary.
ALTER TABLE ai_application_knowledge_jobs
    ADD COLUMN IF NOT EXISTS lease_expires_at timestamptz;

-- Earlier Ready revisions could have been uploaded to an external provider
-- without being attached to its searchable index. No old Ready flag proves that
-- the vector index is usable. Keep revision history but requeue only the
-- latest source for each active document.
UPDATE knowledge_index_generations AS generation
SET state = 'failed'
WHERE generation.state = 'ready';

UPDATE knowledge_document_revisions AS revision
SET state = 'blocked', error = 'Superseded during unified retrieval reindex', ready_at = NULL
WHERE revision.state = 'ready'
  AND EXISTS (
      SELECT 1 FROM knowledge_document_revisions AS newer
      WHERE newer.document_id = revision.document_id AND newer.revision > revision.revision
  );

UPDATE knowledge_document_revisions AS revision
SET state = 'accepted', error = '', ready_at = NULL
WHERE revision.state = 'ready';

INSERT INTO knowledge_ingestion_jobs (id, revision_id, idempotency_key, state)
SELECT gen_random_uuid(), revision.id, 'knowledge-revision:' || revision.id, 'queued'
FROM knowledge_document_revisions AS revision
JOIN knowledge_documents AS document ON document.id = revision.document_id
JOIN knowledge_bases AS base ON base.id = document.knowledge_base_id
WHERE revision.state = 'accepted' AND document.deleted_at IS NULL AND base.deleted_at IS NULL
  AND revision.revision = (
      SELECT MAX(candidate.revision) FROM knowledge_document_revisions AS candidate
      WHERE candidate.document_id = document.id
  )
ON CONFLICT (idempotency_key) DO UPDATE SET
  state = 'queued', error = '', next_attempt_at = now(), lease_expires_at = NULL, updated_at = now();

UPDATE knowledge_documents AS document
SET state = 'accepted', error = '', updated_at = now(), version = version + 1
WHERE document.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM knowledge_document_revisions AS revision
      WHERE revision.document_id = document.id AND revision.state = 'accepted'
        AND revision.revision = (
            SELECT MAX(candidate.revision) FROM knowledge_document_revisions AS candidate
            WHERE candidate.document_id = document.id
        )
  );

INSERT INTO ai_application_knowledge_jobs (id, document_id, state)
SELECT gen_random_uuid(), document.id, 'queued'
FROM knowledge_documents AS document
WHERE document.content <> '' AND document.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM knowledge_document_revisions AS revision WHERE revision.document_id = document.id)
    AND NOT EXISTS (
        SELECT 1 FROM ai_application_knowledge_jobs AS job
        WHERE job.document_id = document.id AND job.state IN ('queued', 'running')
    );

UPDATE knowledge_documents AS document
SET state = 'accepted', error = '', updated_at = now(), version = version + 1
WHERE document.content <> '' AND document.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM knowledge_document_revisions AS revision WHERE revision.document_id = document.id);
