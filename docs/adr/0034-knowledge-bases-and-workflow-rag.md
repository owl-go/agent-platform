# User-owned knowledge bases with workflow-scoped retrieval

Knowledge is modeled as a first-class User or Administrator-owned Knowledge Base with optional first-level Categories, immutable Document Revisions, and asynchronous Ingestion Jobs. A Workflow may bind multiple Knowledge Bases; each Run freezes the selected Knowledge Index Generation in its Workflow Snapshot and records only bounded, permission-checked Retrieval Context citations. The platform owns authorization and lifecycle while a single Administrator-configured AnythingLLM deployment is accessed through a trusted adapter, with one isolated workspace per Knowledge Base.

**Status**: accepted

**Considered Options**

- Reuse Workflow Workspace files: rejected because Workspace is mutable execution state, is only User-visible through a read-only browser, and does not provide durable parsing or retrieval semantics.
- Use one shared AnythingLLM workspace with metadata filters: rejected because workspace isolation and deletion/publication boundaries would depend on provider behavior rather than the platform's owner checks.
- Let Runtime containers call AnythingLLM directly: rejected because Runtime egress and credentials must remain isolated; the trusted API/Worker adapter is the only RAG boundary.

**Consequences**

- Upload acceptance and asynchronous parsing/indexing are separate states. Failed ingestion keeps the original object and can be retried without re-uploading.
- Public Administrator-created Knowledge Bases are readable, searchable, and downloadable by authenticated Users but are never mutable by them. User-owned Knowledge Bases remain private.
- Workflow API, scheduled, follow-up, and manual Runs use the same frozen selection and fail closed when their required generation cannot be queried. A missing match is not a failure and is surfaced as an explicit no-hit condition.
- AnythingLLM image/version/embedding conformance remains a deployment gate; adapter and fake contract tests alone do not claim production readiness.
