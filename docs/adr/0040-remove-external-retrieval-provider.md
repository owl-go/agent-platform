---
status: accepted
---

# Remove the external retrieval provider without selecting a replacement

Remove the external Retrieval Provider from application wiring, strict configuration, and the production Compose stack because the product does not currently need external indexing or retrieval. Keep Knowledge Base ownership, source objects, Document Revisions, ingestion records, generation history, and provider-neutral ingestion/retrieval seams; do not silently reactivate the older PostgreSQL embedding path or claim that stored sources are searchable.

Until a replacement passes the shared contract and Production Conformance, new ingestion jobs remain unprocessed, Knowledge Base search is unavailable, and a Workflow or Smart Assistant that requires Knowledge Selection fails closed. Deployment removes the former service, credentials, and data volume. This supersedes the provider-specific parts of ADR-0034 without changing Knowledge Base ownership or lifecycle semantics.
