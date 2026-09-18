# Knowledge Base and Workflow Retrieval

This document defines the technical seam for the Knowledge Base feature described by ADR-0034. It is an implementation contract, not evidence that a real AnythingLLM deployment has passed production conformance.

## Ownership and model

- `KnowledgeBase` is owned by a User or by the Administrator. User-owned bases are private. Administrator-created bases carry an explicit private/public visibility; public bases can be read, searched, and downloaded by authenticated Users but cannot be mutated by them.
- `KnowledgeCategory` is an optional, non-nested first-level grouping. A `KnowledgeDocument` has at most one Category and may remain unclassified.
- A normalized upload SHA-256 or normalized public URL identifies one logical document inside one Category. A later upload or URL refresh creates a new immutable `DocumentRevision`; the same source in a different Category is a different document.
- Source bytes remain in private Object Storage. The database stores logical object keys, lower-case SHA-256, size, MIME, source metadata, lifecycle state, and revision references; provider URLs and AnythingLLM IDs never enter ordinary API snapshots.

## Ingestion

Object persistence and validation make a revision `accepted` immediately. An independent durable `IngestionJob` then performs format validation, extraction/OCR, bounded chunking, and embedding/indexing. The revision state is `accepted`, `processing`, `ready`, `failed`, or `blocked`; only `ready` revisions are searchable. Jobs use leases, idempotency keys, bounded retries, exponential backoff, and a terminal failure reason. A failed refresh does not displace the previous ready revision.

The first supported source allowlist is PDF, DOC/DOCX, XLS/XLSX, Markdown, TXT, PNG, JPEG, WebP, and one-time public HTTP(S) URL snapshots. URL fetching rejects loopback, link-local, private, cloud-metadata, unsafe redirects, excessive size, and excessive duration. Unsupported MIME/magic combinations are rejected before persistence. PDF citations retain page locations; Office/Markdown citations retain paragraph or sheet/cell locations; web citations retain the normalized URL; image sources use OCR and remain downloadable when OCR is unavailable.

The existing 100 MiB single-file limit remains. Initial configurable defaults are 10,000 documents, 10 GiB of source bytes per Knowledge Base, and 1,000 ingestion revisions per day. Failed retries count against the daily task quota.

## AnythingLLM adapter

Administrator configuration supplies one controlled AnythingLLM endpoint, fixed deployment version/digest, API secret, and embedding model. The API/Worker process owns this secret and the Runtime never receives it. Creating a Knowledge Base idempotently creates one provider workspace; deletion revokes query eligibility and asynchronously destroys the workspace. Rebuilds reuse the workspace while the platform records a monotonically identified `KnowledgeIndexGeneration`.

The adapter exposes structured `CreateWorkspace`, `DeleteWorkspace`, `UpsertRevision`, `RemoveRevision`, and `QueryGeneration` operations. It must translate provider errors into safe categories, never persist raw provider responses, and never use provider workspace IDs as product authorization keys. Contract tests use a fake adapter; real availability requires the exact deployment digest and embedding model to pass the production Conformance gate.

## Workflow execution

Workflow input and snapshots carry one or more Knowledge Base IDs. Category/document selection is intentionally not part of the Workflow contract; all unclassified and categorized documents in each selected base participate. At Run creation, the platform freezes the selected Knowledge Index Generation in the immutable Workflow Snapshot. A later ready revision affects only later Runs or an explicit rerun.

Before the primary Runtime model call, the trusted adapter queries each frozen generation with the fixed Workflow goal plus the current Run/Follow-up text. It returns at most eight deduplicated excerpts and a bounded 6,000-token context. A no-hit result continues execution with an explicit no-hit marker. Provider unavailability, revoked access, or an unavailable frozen generation fails the Run with a structured error; the platform never silently continues without required knowledge.

The Run history stores a bounded, redacted `KnowledgeCitation` for each returned excerpt (source identifiers, location, relevance, content hash, and short display text). Opening the source performs a fresh owner/public permission check. Deleting a document or changing a public base to private revokes source downloads and future queries without rewriting completed Run history.

## Lifecycle and API shape

- User-owned mutations enforce owner scope and return non-enumerating not-found errors across owners. Administrator-only mutations manage Administrator bases and AnythingLLM configuration.
- Deleting a base, Category, or document immediately hides it from new listings and retrieval, cancels outstanding jobs, and schedules idempotent object/index cleanup. A 30-day tombstone permits owner/Administrator restore; after expiry, bytes and provider data are permanently removed.
- Authenticated JSON APIs manage bases, Categories, documents, URL imports, revision status, retry, restore, and downloads. File ingestion uses a dedicated multipart endpoint; URL ingestion uses JSON. Mutating requests support idempotency keys and optimistic versions.
- Workflow API credentials can start or inspect their Workflow Runs but cannot manage Knowledge Bases. Manual, scheduled, follow-up, and API Runs all use the same frozen selection.

## Conformance boundary

The adapter, fake, ingestion state machine, permission checks, object lifecycle, and Workflow snapshot behavior are testable without external services. A production claim additionally requires the exact AnythingLLM image digest, embedding model, endpoint health, workspace isolation, query citation behavior, deletion cleanup, and Linux/network deployment evidence. Missing or skipped evidence is reported as unverified rather than passed.
