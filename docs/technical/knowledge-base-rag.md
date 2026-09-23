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

Administrator configuration supplies one controlled AnythingLLM endpoint, fixed deployment version/digest, API secret, and embedding model under the strict `anythingllm` YAML section. The API/Worker process owns this secret and the Runtime never receives it. The Worker uses AnythingLLM's `/v1/workspace/new`, `/v1/document/upload`, and `/v1/workspace/{slug}/vector-search` APIs; the provider workspace slug is derived from the platform Knowledge Base ID and is never exposed to clients. Creating a Knowledge Base idempotently creates one provider workspace; deletion revokes query eligibility and asynchronously destroys the workspace. Rebuilds reuse the workspace while the platform records a monotonically identified `KnowledgeIndexGeneration`.

When `anythingllm.endpoint` is omitted, source acceptance remains available but ingestion and retrieval stay unavailable until an administrator configures the provider. A configured Worker claims durable `KnowledgeIngestionJob` rows, reads the private source object, uploads it to the Knowledge Base workspace, and records a ready revision plus a new index generation. Runtime containers receive only the bounded retrieval text; they never receive the AnythingLLM API key or call its API.

The current adapter exposes `EnsureWorkspace`, `DeleteWorkspace`, `UpsertRevision`, `RemoveRevision`, and `Query`. The API and Worker reuse the trusted adapter boundary; a browser never calls AnythingLLM directly. Provider errors must not expose its credentials or raw responses, and provider workspace IDs are never product authorization keys. `RemoveRevision` currently has no provider-side delete-by-source implementation; product retrieval therefore revalidates every candidate against current database permissions and revision state. Contract tests use a fake adapter; real availability requires the exact deployment digest and embedding model to pass the production Conformance gate.

## Interactive Knowledge Base search

The Knowledge Base detail page offers a read-only test query so a User can check indexed content without creating a Workflow Run or Smart Assistant conversation. This query targets the latest **ready** Knowledge Index Generation at request time; unlike a Run, it does not freeze a generation in a Workflow Snapshot. The endpoint is `GET /api/v1/knowledge-bases/{knowledge_base_id}/search?q={query}` with the normal authenticated User token, not a Workflow API credential. A private Base is visible only to its owner; a public Administrator Base is readable by any authenticated User. Cross-owner or deleted Bases return `404`, and blank queries or queries longer than 500 Unicode code points return `422`.

After authorization, a Base with no ready generation returns HTTP `200` with `{"index_ready":false,"items":[]}`. Otherwise the API asks AnythingLLM for up to 30 bounded candidates and returns at most ten nonempty excerpts in retrieval order. Each excerpt is truncated to 1,200 Unicode code points. The provider's revision title or source location must contain a Document Revision UUID; the API resolves that UUID against the requested Base and requires an active Base, Category (when present), and document, plus the document's latest ready revision. Unknown, deleted, superseded, failed, or inaccessible candidates are skipped. Document and Category display names come from the platform database, not untrusted provider metadata. The response never contains a provider URL, Object Key, signed download URL, or API secret.

The current AnythingLLM client passes a generation number to `Query` but its vector-search call is scoped only by workspace; it cannot select a historical provider index by generation. The interactive page intentionally searches the current provider workspace and filters results to currently eligible revisions. The stronger frozen-generation reproducibility required for Workflow Runs remains a separate conformance gap; a ready-generation database row alone does not prove that provider-side snapshot isolation exists.

Successful responses use the following shape; an indexed no-hit result has `index_ready: true` and an empty `items` array:

```json
{
  "index_ready": true,
  "items": [
    {
      "document_id": "<uuid>",
      "revision_id": "<uuid>",
      "document_name": "guide.txt",
      "category_name": "Manuals",
      "text": "Relevant source excerpt...",
      "relevance": 0.91
    }
  ]
}
```

`category_name` is omitted for unclassified documents. Missing provider configuration returns `503`; a provider query failure returns `502`; a database lookup failure returns `500`. The page distinguishes an unready index, an indexed no-hit result, and a retrieval error. This endpoint does not invoke a Provider Model, generate an answer, or archive the query; the cited excerpt is a search preview, not a full document download or a retained Run citation.

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

The adapter, fake, ingestion state machine, permission checks, object lifecycle, interactive source filtering, and Workflow snapshot behavior are testable without external services. Local tests cover the ten-item bound, stale/deleted-source rejection, owner/public database access, and the page's result states. They do **not** prove that a real uploaded file produces a usable vector hit with revision provenance. Production acceptance must upload a supported file, wait for a ready revision and generation, search for a distinctive phrase, confirm a sourced excerpt, then verify no-hit, unauthorized access, deletion/replacement filtering, provider failure, and cleanup against the exact AnythingLLM image digest and embedding model. Endpoint health, workspace isolation, and Linux/network deployment evidence are also required. Missing or skipped evidence is reported as unverified rather than passed.
