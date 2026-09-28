# User-owned knowledge bases with workflow-scoped retrieval

Knowledge is modeled as a first-class User or Administrator-owned Knowledge Base with optional first-level Categories, immutable Document Revisions, and asynchronous Ingestion Jobs. A Workflow may bind multiple Knowledge Bases; each Run freezes the selected Knowledge Index Generation in its Workflow Snapshot and records only bounded, permission-checked Retrieval Context citations. The platform owns authorization and lifecycle while any external Retrieval Provider remains behind a trusted API/Worker boundary.

**Status**: accepted; provider activation superseded by ADR-0040

**Considered Options**

- Reuse Workflow Workspace files: rejected because Workspace is mutable execution state, is only User-visible through a read-only browser, and does not provide durable parsing or retrieval semantics.
- Use one shared provider index with metadata filters: rejected because isolation and deletion/publication boundaries would depend on provider behavior rather than the platform's owner checks.
- Let Runtime containers call a Retrieval Provider directly: rejected because Runtime egress and credentials must remain isolated; the trusted API/Worker adapter is the only retrieval boundary.

**Consequences**

- Upload acceptance and asynchronous parsing/indexing are separate states. Failed ingestion keeps the original object and can be retried without re-uploading.
- Public Administrator-created Knowledge Bases are readable, searchable, and downloadable by authenticated Users but are never mutable by them. User-owned Knowledge Bases remain private.
- Workflow API, scheduled, follow-up, and manual Runs use the same frozen selection and fail closed when their required generation cannot be queried. A missing match is not a failure and is surfaced as an explicit no-hit condition.
- No Retrieval Provider is active. Provider-backed indexing and search remain unavailable until an exact implementation and deployment artifact pass the required Conformance gate.
