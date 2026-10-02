# Knowledge Base ingestion and retrieval boundary

This document defines the current technical seam for the Knowledge Base feature described by ADR-0034 and ADR-0040. The platform currently has no active Retrieval Provider; source management remains available, while ingestion and retrieval availability must not be claimed.

## Ownership and source persistence

- The platform owns Knowledge Bases, Categories, Documents, immutable Document Revisions, permissions, lifecycle state, Workflow bindings, generations, and retained citations.
- Source bytes remain in private Object Storage. The database stores logical object keys, lower-case SHA-256, size, MIME, source metadata, lifecycle state, and revision references; provider URLs and identifiers never enter product snapshots.
- User-owned bases are private. Administrator bases may be private or public; public bases are authenticated read/search/download resources but remain Administrator-only for mutation.
- Deletion immediately removes resources from new listings and retrieval eligibility while preserving the existing tombstone and restore behavior.

## Paused ingestion state

Uploads and captured public URL sources may still be validated and persisted. Their durable Ingestion Jobs are not claimed while no Retrieval Provider is configured, so they do not advance to `ready`. Existing source objects and product-owned records remain available for a future provider.

The Worker must not mark a revision Ready without verified indexing. Retry and regeneration records may remain queued, but a missing provider is never converted into a false success. Historical Ready generations remain database history; they are not proof that a currently queryable index exists.

## Provider-neutral seams

The ingestion seam accepts immutable source bytes and a revision identity, then commits Ready state only after a provider confirms indexing. The retrieval seam accepts a principal, Knowledge Base, generation, bounded query, result limit, and token limit. Candidate results are resolved back to an accessible latest Ready platform revision before they may be returned or cited.

No provider implementation is wired in the current API or Worker. A future provider must keep credentials inside the trusted API/Worker boundary, expose no provider identifier as an authorization key, and satisfy the existing state, permission, provenance, generation, cleanup, and error contracts before activation.

## API and execution behavior

Knowledge Base catalog, Category, source lifecycle, download, and restore APIs remain available. Search returns an explicit unavailable response when a historical Ready generation exists but no retrieval provider is active; a base without a Ready generation continues to report `index_ready: false`.

Workflow Runs and Smart Assistant conversations with Knowledge Selection fail closed when retrieval is required and no provider is active. They never silently answer as though selected knowledge had been consulted. Executions without Knowledge Selection are unaffected.

The retired AI Applications embedding-provider endpoint remains HTTP `410`; no browser configuration surface is restored. Historical embedding settings and pgvector rows are retained but are not an active product query path.

## Smart Assistant scope review

After platform safety and direct/semantic FAQ handling, a Smart Assistant's model may make a preliminary scope rejection without Knowledge context. When that Assistant has selected Knowledge Bases, the service queries the existing permission-checked retrieval seam with the original question before returning the fixed scope refusal. Missing provider configuration, unavailable generations and revoked access remain failures rather than no-hit or scope-refusal results.

With non-empty verified excerpts, one metered scope review receives the original question, current Assistant configuration, enabled FAQ identities and source-labelled excerpts. It distinguishes selected project/business documentation from this Assistant's hidden configuration or credentials, requires actual relevance rather than keyword overlap, and treats retrieved instructions as inert source material. If admitted, generation reuses the same excerpts and original question; it does not query again or accept a different question from the review. No-hit and an upheld scope rejection retain the fixed refusal. Explicit and normalized FAQ matches, semantic FAQ decisions and the initial platform safety rejection do not invoke this review path.

The additional model call uses its own Credit stage identity (`4`), distinct from initial classification (`1`), summary (`2`) and answer generation (`3`). Its usage is accumulated even when it refuses or returns an invalid classification; failed/cancelled calls use the common reservation release and error boundary. Recording-provider tests cover control flow, source injection, refusal, failure, cancellation and settlement, but do not establish live retrieval or a real model's semantic relevance judgment.

## Deployment

Production Compose contains no external retrieval service or credentials. The guarded deployment removes the former container and its data volume and strips obsolete YAML/env configuration after backups are created.

## Conformance boundary

Local tests cover provider-neutral ingestion/retrieval contracts, permission checks, source filtering, fail-closed execution, and the absence of retired configuration. They do not establish live Knowledge Base retrieval. Enabling another provider requires black-box evidence for upload-to-index-to-hit, no-hit, authorization, replacement, deletion, cleanup, provider failure, generation handling, and the exact deployed artifact.
