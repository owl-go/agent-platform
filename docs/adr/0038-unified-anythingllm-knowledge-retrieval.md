---
status: accepted
---

# Use one AnythingLLM retrieval path for Knowledge Bases

ADR-0035's PostgreSQL full-text/pgvector implementation introduced a second index beside the existing AnythingLLM-backed Knowledge Base. That made a document's `Ready` label and the preview search insufficient evidence that a Smart Assistant or Workflow would retrieve the same content. It also left the AI Applications embedding settings in the UI even when they had no effect on platform Knowledge Base search.

Use the platform-controlled AnythingLLM deployment for every new Knowledge Base ingestion and retrieval path: interactive test search, Workflow Runs, authenticated Smart Assistant conversations, and new public shared conversations. Keep source bytes, immutable Document Revisions, permissions, lifecycle state, and citation metadata in the platform. A single retrieval module asks AnythingLLM for candidates, then resolves each candidate to a currently accessible, latest Ready platform revision before use. Upload alone is not indexing: the Worker explicitly attaches uploaded documents to the workspace embedding index and checks workspace membership before recording Ready. Existing text documents without platform revisions enter the same source-revision queue. Previously Ready revisions and generations are invalidated and requeued because their old status did not prove AnythingLLM workspace indexing; they are not considered Ready merely because a legacy upload or embedding job once succeeded.

The standalone AI Applications embedding configuration is retired; deployment configuration for AnythingLLM owns the effective embedding model and credentials. Historical settings and old pgvector rows are retained for safe rollback/audit, but no active product query uses them. Source reindexing creates a new revision and preserves the previous Ready revision until ingestion succeeds. Failed ingestion is visible and retryable.

AnythingLLM workspaces are mutable and cannot query an older `KnowledgeIndexGeneration`. A Workflow Run frozen to a superseded generation must fail closed instead of using current content under an old generation number. Exact historical replay would require immutable provider workspaces or another verified snapshot-capable provider; this decision does not claim it. Live end-to-end conformance against the pinned AnythingLLM deployment is still required before claiming production verification.

This supersedes ADR-0035's initial PostgreSQL retrieval and administrator-managed embedding-provider decision. It does not change Knowledge Base ownership or Smart Assistant model selection.
