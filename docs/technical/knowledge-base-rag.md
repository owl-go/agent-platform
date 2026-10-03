# Knowledge Base ingestion and retrieval boundary

ADR-0042 selects RAGFlow v0.24.0 as the optional Retrieval Provider. Knowledge Base management remains available when `retrieval` is omitted. In that case ingestion is paused and required retrieval fails closed. See the [local deployment instructions](../../deploy/ragflow-local/README.md) and [2026-10-02 local evidence](../evidence/agent-workspace/2026-10-02-ragflow-local-validation.md) and [platform acceptance](../evidence/agent-workspace/2026-10-02-ragflow-platform-validation.md) for actual verification; configuration and stored Ready flags alone do not establish live availability.

## Ownership and source persistence

The platform owns private, Department and Platform Knowledge Bases, Categories, Documents, immutable Document Revisions, current access, lifecycle state, Workflow bindings, generations and retained citations. Original bytes stay in private Object Storage under logical keys with size, SHA-256 and MIME metadata. RAGFlow identifiers live only in `knowledge_provider_revisions`, namespaced by deployment identity; they never authorize access or enter product snapshots. Each base/deployment uses one private dataset and every revision uses an independent document with its UUID filename and original format extension.

## Ingestion and recovery

Upload/URL capture validates and persists source bytes before queuing an Ingestion Job. A configured Worker migrates legacy Assistant text sources to the same Object Storage/revision queue, then uploads each source, explicitly starts RAGFlow parsing, and polls the document until DONE with a positive chunk count and an actually visible, available indexed chunk. Empty extraction, parsing failure, missing documents, HTTP/API failure or timeout cannot become Ready.

A ten-minute lease and at most eight-minute execution context allow interrupted processing to be reclaimed. RAGFlow upload mappings and exact revision filenames allow retry to resume previously uploaded/parsing documents. Three automatic attempts use bounded exponential backoff; terminal failure is visible and can be retried from saved source. Finishing checks the running attempt and active Base/Category/document, so deleted or cancelled jobs cannot publish Ready. A previous Ready revision survives replacement failure. Generation allocation, immutable membership, source readiness and job completion commit together.

## Generations and retrieval

`knowledge_generation_revisions` records the latest Ready revision of every active document when a generation is committed. Previous provider documents stay indexed to support frozen Workflow generations. RAGFlow retrieval always supplies a single mapped dataset and that generation's explicit document IDs; absent or partial mappings are an error, never an unfiltered dataset query. Existing pre-RAGFlow Ready generations are invalidated and active original sources requeued by migration 000067.

The common retrieval Engine checks current private/Department/Platform access before calling RAGFlow and again before returning. Candidates must match the provider mapping, frozen manifest, Ready revision, active Base, active Category and active document. A frozen Run can use a retained older revision; latest-generation preview does not return superseded revisions. Results are deduplicated, bounded and contain only platform source identity, safe text and relevance. Provider diagnostics and HTTP transport details are not persisted because they may contain credentials, URLs or source content. Redirects are refused and JSON responses are size bounded.

Knowledge detail search returns `index_ready: false` without a Ready generation, an empty `items` array for an indexed no-hit, and an explicit error on outage. Workflow Runs receive at most eight excerpts within a bounded context; no-hit is explicit, required retrieval failure prevents Runtime execution. Smart Assistant owner, validation and public conversation paths share the same trusted searcher. No browser embedding configuration is introduced; the retired endpoint stays HTTP 410.

## Lifecycle

Soft deletion immediately excludes sources from all retrieval. Restore is allowed for thirty days and requeues ingestion cancelled by document deletion, preserving the attempt counter so a stale Worker cannot finish the restored claim. Cleanup then removes RAGFlow documents/mappings and deletes original Object Keys only when every reference has expired and no provider mapping remains. Regenerated revisions may share one original key; cleanup must not delete a retained reference. Metadata and historical citations do not grant download access. Old documents are retained while an active generation may still need them.

## Smart Assistant scope review

After platform safety and direct/semantic FAQ handling, a Smart Assistant's model may make a preliminary scope rejection without Knowledge context. When that Assistant has selected Knowledge Bases, the service queries the existing permission-checked retrieval seam with the original question before returning the fixed scope refusal. Missing provider configuration, unavailable generations and revoked access remain failures rather than no-hit or scope-refusal results.

With non-empty verified excerpts, one metered scope review receives the original question, current Assistant configuration, enabled FAQ identities and source-labelled excerpts. It distinguishes selected project/business documentation from this Assistant's hidden configuration or credentials, requires actual relevance rather than keyword overlap, and treats retrieved instructions as inert source material. If admitted, generation reuses the same excerpts and original question; it does not query again or accept a different question from the review. No-hit and an upheld scope rejection retain the fixed refusal. Explicit and normalized FAQ matches, semantic FAQ decisions and the initial platform safety rejection do not invoke this review path.

The additional model call uses its own Credit stage identity (`4`), distinct from initial classification (`1`), summary (`2`) and answer generation (`3`). Its usage is accumulated even when it refuses or returns an invalid classification; failed/cancelled calls use the common reservation release and error boundary. Recording-provider tests cover control flow, source injection, refusal, failure, cancellation and settlement, but do not establish live retrieval or a real model's semantic relevance judgment.


## Trusted configuration

API and Worker consume the same strict YAML block:

```yaml
retrieval:
  provider: ragflow
  ragflow:
    endpoint: ${RAGFLOW_ENDPOINT}
    api_key: ${RAGFLOW_API_KEY}
    deployment_id: ${RAGFLOW_DEPLOYMENT_ID}
    embedding_model: ${RAGFLOW_EMBEDDING_MODEL}
    request_timeout: 30s
    parse_timeout: 5m
```

The endpoint is an HTTPS origin (or loopback HTTP for local tests) without user info, query, fragment or path. Partial/unknown configuration is rejected. `deployment_id` identifies one durable RAGFlow installation; renewing its ngrok origin does not change it. The credential belongs only to trusted API/Worker processes. Production Compose passes the four variables to those services and does not install a RAG engine on the server. Model calls for answering continue to use the platform's configured Provider Model; RAGFlow handles extraction and local embedding/retrieval.

## Conformance boundary

Local contracts cover safe HTTP behavior, upload/parse readiness, retry reuse, scoped retrieval, generation manifests, leases, authorization and retention cleanup. Live tests must additionally exercise the exact RAGFlow digest and local embedding model, and an authenticated platform upload/search plus Workflow and Smart Assistant path. Linux + gVisor Runtime conformance remains separate from the RAGFlow API conformance.
