---
status: superseded by ADR-0038
---

# Use replaceable PostgreSQL vector retrieval

The first Knowledge Base retrieval implementation uses PostgreSQL full-text matching plus the `pgvector` extension behind a `RetrievalProvider` and `EmbeddingProvider` boundary. The platform retains Knowledge Base ownership, Document Revisions, permissions, chunk provenance, Assistant bindings, and Knowledge Citations; the provider owns only indexing and recall. Versioned embedding generations switch atomically, preserving the previous Ready generation during rebuilds and allowing a later Ragflow or other RAG adapter without changing Assistant or conversation semantics.

This keeps the initial deployment self-contained and testable while treating the retrieval engine as an integration seam rather than a domain owner. Embedding credentials remain Administrator-managed protected configuration and never enter ordinary snapshots, browser responses, logs, or artifacts.
