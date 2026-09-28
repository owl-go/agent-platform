UPDATE knowledge_document_revisions
SET error = 'Superseded during unified retrieval reindex'
WHERE state = 'blocked'
  AND error LIKE 'Superseded during unified % reindex';
