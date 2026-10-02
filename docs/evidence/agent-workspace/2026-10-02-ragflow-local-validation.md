# RAGFlow local validation — 2026-10-02

## Installation and scope

The development branch is `codex/ragflow-knowledge-loop`. No server release or production database migration was performed for this change. RAGFlow, its trusted platform adapter and local embeddings were exercised on the Mac with Docker Desktop; this is local integration evidence, not authenticated production acceptance.

- RAGFlow v0.24.0: `infiniflow/ragflow@sha256:508cc283f5d61d1e565820a8e0ae8d98f9a8ac4b3b8cf7aefc5f0093e4bb4c50`, linux/amd64 under Apple Silicon emulation.
- Native Ollama 0.35.0, `bge-m3` digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, registered as `bge-m3:aw-790764642607@Ollama`; a Chinese embedding returned 1,024 dimensions.
- RAGFlow uses Elasticsearch, MySQL, Valkey and its supported OpenDAL/MySQL internal storage. Original platform source bytes remain behind the existing Object Store interface.
- Local UI: `http://127.0.0.1:19380`. Restricted Knowledge SDK: `http://127.0.0.1:19381`. Optional shared model/Knowledge entry: `http://127.0.0.1:19382`.
- Credentials and the live test JSON are stored with restricted permissions outside the repository, under `/Users/frank/.local/share/agent-platform-ragflow`. The change set was scanned against those actual credential values with no matches.

## Executed checks

The following completed successfully:

- `make -C backend wire` regenerated API assembly with the configured searcher.
- `go -C backend test ./internal/knowledgebase/... ./internal/platformconfig/...`.
- PostgreSQL integration tests matching `TestKnowledge(Source|Generation|Ingestion)` with a disposable PostgreSQL 17 service, including immutable manifests, access checks, expired leases, retries, delete/restore claim fencing and shared-source cleanup.
- `make test` with the disposable PostgreSQL DSN, and `make build`.
- `make web-typecheck` and `make web-build`; the existing large-bundle warning remains. No frontend behavior was changed.
- `git diff --check`.

`TestRAGFlowLiveKnowledgeLoop` was explicitly enabled using the protected RAGFlow test JSON and disposable PostgreSQL DSN. The final invocation passed in 7.76 seconds. It used real RAGFlow parsing, Elasticsearch retrieval and local Ollama embeddings, with a platform repository and Memory Object Store. It verified Chinese upload/parse/readiness/hit, replacement, current versus frozen generation content (fifteen versus seven days), private source denial, indexed no-hit, immediate deletion exclusion, restoration, rejection after thirty days, provider deletion and original-source cleanup. Test documents were removed from the provider afterward; empty dataset metadata can remain.

The real API returned code 108 for an absent exact dataset-name lookup and code 102 for an absent exact document-name lookup. Parsing DONE preceded searchable chunk visibility. The adapter now waits for an actually visible, available chunk before publishing Ready; contract tests reproduce that delay and resume without uploading/parsing twice.

Restricted gateway checks returned 404 for `/`, `/v1/user/login` and `/v1/user/register`. Its shared-entry Nginx configuration passed `nginx -t`; the model `/health` returned 200 directly and through the proxy, model `/api/v1/models` retained the identical unauthenticated 401 response, and an authenticated Knowledge dataset request returned HTTP 200/code 0 through port 19382. Existing public model routing was not changed. Streaming/WebSocket forwarding is configured but was not black-box tested.

## Remaining acceptance

The existing free ngrok endpoint is already online for the local model relay on port 3000. Starting an independent Knowledge tunnel failed with `ERR_NGROK_334`; that unsuccessful launch agent was stopped. The optional shared entry is prepared and locally validated, but switching the existing tunnel requires the user's pending choice because it affects an existing model service. A separate ngrok account/endpoint is the alternative.

After resolving the tunnel, integrate through `main_temp`, run the guarded release with backup/migration/health checks, and execute authenticated platform HTTP upload/search, a grounded Workflow Run with retained citations, and owner/public Smart Assistant conversations. Those server paths, public tunnel retrieval, real platform Object Store round trip, provider outage/restart acceptance, PDF/OCR/spreadsheet formats and Linux + gVisor conformance remain unverified. HTTP failure/timeout/redirect protection and source byte integrity are covered by local contract tests. The ordinary test command skips the optional live RAGFlow test unless its protected configuration is supplied; such skips are not live evidence.
