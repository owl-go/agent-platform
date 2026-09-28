# Knowledge Base and Workflow RAG Specification

> Current implementation status: ADR-0040 retires the previously selected external Retrieval Provider without choosing a replacement. The provider-specific stories and decisions below remain design history, not currently available product behavior; source ownership, persistence, and lifecycle requirements remain active.

## Problem Statement

Agent Workspace currently has attachments, Workflow Workspaces, and Artifacts, but no durable knowledge collection that a User can maintain and reuse across Workflow Runs. Users cannot organize source material, upload or capture supported documents, wait for asynchronous parsing and embedding, or ask a Workflow to retrieve grounded context from selected knowledge.

The existing `Workspace`, `Artifact`, and `File Reference` concepts cannot be repurposed safely: a Workspace is mutable Workflow execution state, an Artifact is a generated deliverable, and a File Reference is scoped to one conversation turn. The product therefore needs a first-class `Knowledge Base` model with explicit ownership, lifecycle, indexing, permission, and Run reproducibility semantics.

## Solution

Add a top-level authenticated Knowledge Bases area. A User can create private Knowledge Bases. The Administrator can create private or public Knowledge Bases; a public Administrator Knowledge Base is readable, searchable, and downloadable by every authenticated User, while only the Administrator can mutate it.

Each Knowledge Base contains optional non-nested first-level `Knowledge Categories` and `Knowledge Documents`. A document may be unclassified, but belongs to at most one Category. Files and one-time public HTTP(S) URL snapshots are accepted immediately after safe validation and persisted in private Object Storage. Parsing, OCR, chunking, and embedding/indexing happen asynchronously through durable `Ingestion Jobs`.

A Workflow can bind multiple complete Knowledge Bases. When a Run is created, its Workflow Snapshot freezes the selected `Knowledge Index Generation`. Before the Runtime model call, a trusted platform adapter queries the frozen generation with the Workflow goal and current Run input, then injects a bounded `Retrieval Context`. The Run history keeps redacted `Knowledge Citations`; source viewing always performs a fresh permission check.

## User Stories

1. As a User, I want to create a private Knowledge Base, so that I can maintain reusable material without exposing it to other Users.
2. As a User, I want to rename or describe my Knowledge Base, so that I can recognize its purpose in the catalog.
3. As an Administrator, I want to create an Administrator-owned Knowledge Base, so that I can publish platform-maintained reference material.
4. As an Administrator, I want to choose private or public visibility for an Administrator Knowledge Base, so that internal material and shared material have different access boundaries.
5. As an authenticated User, I want to browse a public Administrator Knowledge Base, so that I can understand what reference material is available to me.
6. As an authenticated User, I want to search and download a public Knowledge Base document, so that I can inspect the source behind a Workflow answer.
7. As a non-owner User, I want mutations to a private Knowledge Base to appear as not found, so that private resource existence is not enumerable.
8. As a User, I want to create a first-level Knowledge Category, so that I can organize documents without introducing an arbitrary folder tree.
9. As a User, I want to upload a document into a Category, so that retrieval can preserve the source's organizational context.
10. As a User, I want to upload a document without a Category, so that small or cross-cutting sources do not require artificial classification.
11. As a User, I want one document to belong to no more than one Category, so that ownership and deletion remain unambiguous.
12. As a User, I want to upload the same source into two different Categories, so that each Category can own an independent document lifecycle.
13. As a User, I want to replace a source in one Category without overwriting its history, so that previous revisions remain auditable.
14. As a User, I want to upload PDF, DOC/DOCX, XLS/XLSX, Markdown, TXT, PNG, JPEG, and WebP sources, so that common business and media formats can become searchable knowledge.
15. As a User, I want to submit one public HTTP(S) URL, so that a web page snapshot can become a stable source without requiring a crawler account.
16. As a User, I want unsafe URLs, private-network destinations, unsafe redirects, unsupported formats, and MIME/magic mismatches rejected, so that ingestion cannot become an SSRF or content-smuggling path.
17. As a User, I want a valid upload to succeed as soon as its bytes are safely persisted, so that slow parsing and embedding do not block the upload request.
18. As a User, I want to see `accepted`, `processing`, `ready`, `failed`, or `blocked` ingestion state, so that I know whether a source can participate in retrieval.
19. As a User, I want a failed Ingestion Job to retain its original source and error summary, so that I can retry without uploading the same bytes again.
20. As a User, I want a successful URL refresh to become a new Document Revision, so that new page content does not rewrite the old source.
21. As a User, I want a failed URL refresh to leave the previous ready revision searchable, so that a transient fetch or provider failure does not remove usable knowledge.
22. As a User, I want to delete a Category or document, so that it immediately leaves new listings and retrieval while cleanup happens safely in the background.
23. As a User, I want to restore a deleted private Knowledge Base, Category, or document during its tombstone period, so that an accidental deletion is recoverable.
24. As an Administrator, I want to restore or permanently remove an Administrator Knowledge Base after its retention period, so that lifecycle cleanup is auditable and complete.
25. As a User, I want configurable document-count, byte, and daily ingestion quotas, so that my Knowledge Base cannot grow without bounds.
26. As an Administrator, I want one controlled Retrieval Provider configuration, deployment identity, and protected credentials when retrieval is enabled, so that RAG infrastructure is centrally governed.
27. As a User, I want the platform to hide provider index IDs and secrets, so that provider details do not become a second authorization surface.
28. As a User, I want an unavailable Retrieval Provider to fail an affected retrieval explicitly, so that the Workflow never silently answers without required knowledge.
29. As a User, I want an empty retrieval result to continue with an explicit no-hit marker, so that no relevant source is distinguished from a provider outage.
30. As a User, I want to bind multiple Knowledge Bases to one Workflow, so that a single execution can use several approved sources.
31. As a User, I want Workflow binding to select whole Knowledge Bases rather than fragile document lists, so that Category management does not constantly rewrite Workflow configuration.
32. As a User, I want a Run to freeze its Knowledge Selection and Knowledge Index Generation when created, so that later edits do not change the intended scope of an existing Run.
33. As a User, I want the retrieval query to combine the fixed Workflow goal with the current Run or Follow-up input, so that both task intent and the immediate question ground the answer.
34. As a User, I want retrieval to be bounded and deduplicated, so that large Knowledge Bases do not overwhelm the Runtime context.
35. As a User, I want each result to identify its Knowledge Base, Category, document revision, source location, relevance, and safe display text, so that I can verify the answer.
36. As a User, I want no-hit citations to be distinguishable from ordinary response text, so that I do not mistake an unsupported answer for a sourced answer.
37. As a User, I want Run history to retain bounded, redacted Knowledge Citation summaries, so that completed executions remain auditable without copying an entire Knowledge Base into history.
38. As a User, I want clicking a citation to re-check current permission, so that a later deletion or privacy change revokes source access immediately.
39. As an Administrator, I want changing a public Knowledge Base to private to affect queued and future retrievals, so that visibility changes take effect without rewriting completed Run history.
40. As a User, I want public Knowledge Base content to remain read-only to me, so that shared reference material cannot be altered by consumers.
41. As a User, I want manual, scheduled, follow-up, and Workflow API Runs to use identical Knowledge Selection semantics, so that trigger type does not create an authorization bypass.
42. As a Workflow API credential holder, I want to start and inspect Runs without managing Knowledge Bases, so that an execution credential cannot mutate product resources.
43. As a User, I want the Knowledge Bases page to show category, unclassified documents, ingestion status, failures, retry, restore, and deletion actions, so that lifecycle operations are understandable without opening an administrator console.
44. As a User, I want Workflow settings to expose only multi-select Knowledge Base binding, so that document management stays in the dedicated Knowledge Bases area.
45. As a User, I want the feature to work on desktop and mobile layouts with keyboard-accessible status and confirmation controls, so that knowledge management remains usable across supported clients.
46. As an authenticated reader, I want to enter a query inside a Knowledge Base and see up to ten relevant excerpts with their document and optional Category as sources, so that I can verify what an uploaded document actually made searchable.
47. As an authenticated reader, I want an unready index, an indexed no-hit result, and a retrieval failure to be distinguishable, so that I do not mistake a pending or unavailable provider for an empty document.
48. As a source owner, I want the test query to hide deleted, superseded, failed, and currently inaccessible Document Revisions, so that a stale provider index cannot show them as valid citations.

## Implementation Decisions

- **Domain model**: Add `Knowledge Base`, `Knowledge Category`, `Knowledge Document`, immutable `Document Revision`, `Ingestion Job`, `Knowledge Selection`, `Knowledge Index Generation`, `Retrieval Context`, and `Knowledge Citation` to the Workspace domain vocabulary. Do not reuse `Workspace`, `Artifact`, `Attachment`, or `File Reference`.
- **Ownership**: User-owned Knowledge Bases are private. Administrator-owned Knowledge Bases carry private/public visibility. Public Administrator bases are authenticated read/search/download resources; all mutations remain Administrator-only.
- **Hierarchy**: Categories are optional, first-level, and non-nested. A document has zero or one Category. The same normalized source in another Category is an independent document.
- **Source identity and revisions**: Upload SHA-256 or normalized URL identifies a logical document within a Category. Replacement or refresh creates a new immutable revision. A failed new revision never displaces the previous ready revision.
- **Source allowlist**: Accept PDF, DOC/DOCX, XLS/XLSX, Markdown, TXT, PNG, JPEG, WebP, and one-time public HTTP(S) snapshots. Extract text for searchable formats, preserve page/sheet/paragraph/URL provenance, use OCR for supported images, and retain images for download when OCR is unavailable.
- **Safety limits**: Keep the existing 100 MiB per-source limit. Start with configurable defaults of 10,000 documents, 10 GiB source bytes per Knowledge Base, and 1,000 ingestion revisions per day. Failed retries count toward the daily task quota.
- **Object storage**: Persist original bytes in the existing private Object Storage abstraction with size and lower-case SHA-256 validation. Store logical object keys only; never store provider URLs or signed parameters in domain snapshots.
- **Ingestion seam**: Add a durable Ingestion Job queue separate from Runtime execution. Jobs use idempotency, leases, bounded retries, exponential backoff, and terminal `failed`/`blocked` reasons. Upload acceptance is independent from asynchronous extraction, OCR, chunking, and indexing.
- **Retrieval Provider boundary**: Use a trusted adapter owned by API/Worker processes. Any future provider configuration supplies a fixed deployment identity and protected credentials. Provider IDs are mappings, not authorization keys. Runtime containers never receive retrieval credentials or call the provider directly.
- **Index generations**: Rebuilds reuse the per-Knowledge-Base provider workspace while the platform records a coherent Knowledge Index Generation. A Run freezes the generation at creation; later ready revisions affect later Runs or explicit reruns.
- **Workflow contract**: Extend Workflow input and snapshot with a set of Knowledge Base IDs. Binding is whole-Knowledge-Base only; Category and document selection remain management/citation boundaries. Validate every selected base against owner/public access at mutation and execution boundaries.
- **Retrieval contract**: Query with fixed Workflow goal plus current Run/Follow-up input. Return at most eight deduplicated excerpts within a 6,000-token context. No-hit continues with an explicit marker. Provider outage, revoked access, or missing frozen generation fails the Run with a structured error.
- **Interactive test query**: The Knowledge Base detail page checks that Base's latest ready generation and searches its current provider workspace, independently of a Workflow Run. Accept a nonempty query of at most 500 Unicode characters, retrieve bounded candidates, and show no more than ten excerpts in provider order with current document/Category names. Revalidate each candidate's owner/public access, active Base/Category/document, and latest ready Document Revision before display. An unready index, no hit, and provider error remain separate outcomes; provider URLs and secrets never enter the response. This current-workspace preview does not satisfy the separate frozen-generation requirement for Runs.
- **Execution evidence**: Persist bounded, redacted Knowledge Citation summaries in Run history, including source identifiers, location, relevance, content hash, and safe display text. Source opening performs a current permission check and does not rely on historical access.
- **Deletion**: Use soft deletion/tombstones. Deletion immediately hides a base/category/document from listings and retrieval, cancels outstanding Ingestion Jobs, and schedules idempotent Object Storage/provider cleanup. Retain a 30-day restore window, then permanently remove source bytes and provider data.
- **API**: Add authenticated JSON endpoints for Knowledge Base/Category/document lifecycle, URL import, status, retry, restore, and downloads. Use a dedicated multipart endpoint for file ingestion. Mutating actions use idempotency keys and optimistic versions. Workflow API credentials cannot manage Knowledge Bases.
- **Test-query API**: Add authenticated read-only `GET /api/v1/knowledge-bases/{knowledge_base_id}/search?q=...`, returning `index_ready` plus up to ten `items` containing excerpt text, relevance, Document/Revision IDs, document name, and optional Category name. The endpoint does not create a conversation, call a model, or persist a query history.
- **Frontend**: Add a top-level Knowledge Bases route with overview, category/unclassified document management, upload/URL import, state filtering, retry, restore, deletion confirmation, public/private badges, and responsive empty/error states. Add only multi-select Knowledge Base binding to Workflow settings.
- **Configuration and deployment**: A future provider must use strict configuration for its endpoint, secret reference, fixed deployment identity, model, quotas, and request limits. Missing or unhealthy provider configuration blocks indexing/retrieval but does not delete accepted source bytes.
- **Conformance**: Adapter fakes and contract tests establish behavior without external services. Production availability requires the exact provider artifact, model, endpoint health, index isolation, query citations, deletion cleanup, and Linux/network evidence. Skipped evidence remains unverified.

## Testing Decisions

- Test the highest seam first: authenticated Workspace API/Application Service behavior using fake Object Storage and a fake Retrieval Provider. Assert externally visible state, authorization, errors, and persisted outcomes rather than private helper calls.
- Add table-driven domain tests for ownership, visibility, Category cardinality, normalized source identity, revision transitions, quota limits, URL safety, and soft-delete/restore semantics.
- Add GORM/PostgreSQL integration coverage for the append-only migration, owner/public read rules, optimistic versions, idempotency, tombstones, Workflow binding, frozen generation snapshots, and cleanup leases. Use the repository's existing integration-test database patterns.
- Add Object Storage contract tests for source size/SHA-256 validation, private logical keys, download authorization, deletion, and idempotent cleanup. Extend the existing Memory/MinIO/Aliyun provider conformance suite rather than inventing a second storage abstraction.
- Add Ingestion Worker tests for accepted-to-ready success, parser/OCR failure, provider outage, retry, lease expiry, duplicate job replay, failed refresh preserving an older ready revision, cancellation after Category/document deletion, and daily quota accounting.
- Add provider adapter contract tests for index creation/deletion, revision upsert/remove, generation query, safe provider error mapping, no raw response persistence, and secret absence. A real deployment test is a separate Conformance gate and cannot be replaced by a fake.
- Add Workflow application/worker tests for multiple-base binding, generation freeze at Run creation, goal-plus-input query construction, bounded/deduplicated context, no-hit continuation, provider-failure fail-closed behavior, permission revocation, citation persistence, and API/scheduled/follow-up parity.
- Add HTTP tests for multipart upload, URL import, status polling, retry, restore, download authorization, idempotency replay, optimistic conflicts, non-enumerating private access, and Workflow settings updates.
- Add browser tests for the Knowledge Bases catalog/detail/upload/category flows, ingestion states and retry, public/private display, responsive layout, keyboard navigation, deletion/restore confirmations, and Workflow multi-select binding. Follow the existing Vue page-test style.
- Test the interactive query's ten-result bound, citation source validation, owner/public isolation, stale/deleted revision filtering, and distinct unready/no-hit UI states. Verify a real upload-to-index-to-search round trip separately against the configured provider deployment; fake adapters and local database tests do not establish that production behavior.
- Run Go formatting and target package tests first, then `make test`, `make build`, `make web-typecheck`, and `make web-build`. Run runtime/storage/production Conformance only when the required provider and Linux environment exists; report unavailable gates as missing evidence.

## Out of Scope

- Knowledge Base binding for Sessions or Conversation Selection in the first version.
- Anonymous public access, arbitrary sharing grants, team membership, or per-User mutation rights on public Administrator Knowledge Bases.
- Nested Categories, multi-category document membership, tags as a second hierarchy, or Workflow-level Category/document filters.
- Recursive site crawling, authenticated websites, scheduled URL refresh, browser sessions, or a general web search engine.
- Unsupported file formats, macro execution, arbitrary archives, private-network URL fetching, or claiming every MIME type is searchable.
- Runtime-direct Retrieval Provider access, provider credentials in Runtime containers, or treating provider index IDs as product ownership.
- Replacing the platform's Workspace, Artifact, Attachment, or File Reference semantics with Knowledge Document objects.
- Session memory, automatic long-term model memory, or copying full source documents into Run history.
- Exact production provider artifact/model claims before the required deployment Conformance evidence exists.
- Commit, Push, Review Branch, PR/MR, or source-control workflows for Knowledge Base documents.

## Further Notes

- The accepted architectural decisions are recorded in ADR-0034 and the glossary additions are in `CONTEXT.md`.
- The product and technical requirements are amended in the existing requirements document and the Knowledge Base/RAG technical design.
- The implementation should preserve the current Domain → Application → Data boundaries: GORM records, HTTP DTOs, and YAML configuration must not enter the domain model.
- A future provider milestone should be a vertical slice: private User Knowledge Base, one Category plus unclassified documents, accepted upload, durable Ingestion Job state, fake provider adapter, one Workflow binding, frozen generation, and citation-bearing retrieval contract. Public Administrator bases and the real provider Conformance gate can then be enabled without changing the core ownership model.
