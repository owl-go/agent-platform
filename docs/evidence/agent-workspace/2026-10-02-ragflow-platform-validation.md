# RAGFlow platform validation — 2026-10-02

## Deployed installation

The final guarded release is `ragflow-knowledge-20261002-4`, deployed from `main_temp` commit `669a54e86f05d65ba5fd1bab58a9289618be3bdc`. The development branch remains `codex/ragflow-knowledge-loop`; no direct deployment from the feature branch or merge into protected `main` was performed.

- Platform: `https://47-237-108-63.sslip.io`.
- Server source: `/opt/agent-platform/src.release-ragflow-knowledge-20261002-4`; Web: `/opt/agent-platform/web/releases/ragflow-knowledge-20261002-4`.
- Verified business/identity database and configuration backup: `/opt/agent-platform/backups/pre-ragflow-knowledge-20261002-4`. The original pre-provider configuration is separately retained under `/opt/agent-platform/backups/pre-ragflow-config-20261002`.
- API image: `sha256:efa2b07415162b730805898871b7ce743c7d9da3a1c319faad6e8332dbac65a9`; Worker: `sha256:ce02dc2bc133608833d8ae729c6f6d1e0c43053cd9da51c8765f27f6101d49a3`.
- Migration `000067_ragflow_knowledge_generations.sql` is installed. API and Worker share the protected RAGFlow configuration.

RAGFlow v0.24.0 and native Ollama `bge-m3` use the exact image/model digests in [local validation](2026-10-02-ragflow-local-validation.md). The durable Compose files are under `/Users/frank/.local/share/agent-platform-ragflow/deployment`, with credentials outside Git and named data volumes retained. Actual container bind mounts were inspected after recreation and point to this directory. The local UI remains on loopback port 19380; Knowledge SDK uses 19381; the shared entry uses 19382.

After the user confirmed switching the existing tunnel, `https://chili-darn-trolling.ngrok-free.dev` was retargeted to `http://127.0.0.1:19382`. Exact Knowledge SDK routes reach RAGFlow; existing model routes still reach the relay on port 3000. The relay container was not restarted. The existing ngrok LaunchAgent was retained; merged configuration passed `ngrok config check`. Traffic inspection was disabled and its request inventory remained empty after acceptance. The failed duplicate tunnel LaunchAgent was removed.

## Authenticated black-box checks

A temporary audience-scoped identity client and an ordinary platform User exercised the public HTTPS API. The existing web client and platform-wide model settings were not changed. A private TXT document recorded a thirteen-day policy and a unique test marker; the answer had to reproduce both, excluding plausible answers from model memory.

The following checks passed:

- Authenticated upload through the platform Object Store, Worker ingestion, local RAGFlow parsing/embedding and Ready publication.
- Chinese HTTP preview search with the expected immutable platform Document Revision citation; indexed unrelated queries returned no hits.
- Another principal, including an Administrator, received HTTP 404 for the private Knowledge Base.
- Workflow Run `f316afa8-e99d-49c5-bb77-ac5caa3c17b5` completed with the thirteen-day answer and exact marker. Its persisted knowledge evidence cites revision `62ab7b31-026c-4d51-9f4f-2cdc049afa41`. Worker restart recovered this Run after its first finalization failure.
- A fresh first Workspace Run on the final release, `bc333307-b283-45b6-915f-e0f9c7c6cd86`, also completed with the exact answer and retained revision citation after removing only the confirmed-deleted temporary Workspace.
- Smart Assistant publication validation, owner conversation and public share conversation completed with grounded answers; the owner turn records `source: knowledge`.
- Document deletion immediately removed retrieval results; restoration made the same retained revision searchable again.
- Stopping only the local Knowledge gateway caused platform search to return HTTP 502; the shared model relay health still returned 200. Restarting restored retrieval.
- Recreating RAGFlow and both gateways with the durable configuration and retained volumes preserved the indexed document, source mapping and revision citation, verified from the server through ngrok.

The test Knowledge Base was `a8cf1342-8574-4770-9d23-234f6b034047`, Document `c23eb685-28f7-40d2-b2e5-8957f00322eb`. These identifiers are evidence only and do not grant access.

## Regression fixes and executed gates

Real HTTP preview initially returned 404 because Kratos does not populate `http.Request.PathValue`. A regression through the actual Kratos router failed before the path-parser fix and passed afterward.

The first Workflow for the temporary User initially generated an answer but failed finalization because its persistent Owner directory was absent. Creating that directory also required the same UID/GID as the staged Workspace so the non-root API could manage it. Regression tests cover first commit, ownership, rollback, existing-parent preservation and rejection of a symlink parent. A cross-compiled test executable ran on the Linux server as root and spawned UID/GID 65532 for real Workspace list/clear operations; all three targeted tests passed. This is Unix permission evidence, not gVisor Conformance.

Executed successfully:

- Targeted Knowledge, strict configuration, HTTP routing and Runtime Executor tests.
- `make test` with a disposable PostgreSQL 17 DSN and `make build`.
- The guarded deploy script with all gates enabled: backend test/build, 491 frontend tests, frontend typecheck/production build, verified backup manifests, Runtime/CLI Builder image smoke tests, CLI bundle re-verification, database migration, service health/readiness, OIDC discovery and HTTPS redirect checks.
- The explicitly enabled real `TestRAGFlowLiveKnowledgeLoop`, including replacement, current versus frozen generations, private denial, deletion/restore, thirty-day expiry and source/provider cleanup; see the earlier local report for its scope.

## Acceptance cleanup

Twenty recorded checks passed, including removal of the temporary Assistant, both Workflows, Expert and Knowledge Base, public share revocation (HTTP 404), zeroing the temporary daily allocation, disabling the ordinary test User and deleting the temporary identity client. Protected acceptance tokens were removed; only nonsensitive check records remain under `/opt/agent-platform/evidence/ragflow-20261002/checks.json`. Historical Run and governance evidence remain subject to the existing lifecycle. Original source/provider records of deleted fixtures retain the documented thirty-day restore window rather than being forcibly purged.

The disposable local PostgreSQL test container was removed. RAGFlow, Ollama, the restricted/shared gateways and the existing ngrok service remain running.

## Verification limits

Real platform ingestion used TXT. PDF/OCR/spreadsheet formats and the full Linux + gVisor Sandbox/Production Conformance suites were not executed for this change. Contract tests cover timeout, redirects, scoped retrieval, source integrity, lease recovery and retry fencing. No WebSocket-specific black-box check was performed.

The Mac, Docker Desktop, Ollama and ngrok must remain available for retrieval. An interruption returns an explicit provider failure; restarting the same deployment with retained volumes restores mappings. Do not remove data volumes while retained source revisions or frozen Runs depend on them.

## Later API credential/account change

The user supplied a replacement API Key and explicitly requested online activation and connectivity testing. Authentication succeeded, but the new Key belonged to another RAGFlow account: it could not access the previous private datasets and did not have `bge-m3:aw-790764642607@Ollama` configured. A real retrieval attempt raised `LookupError(... not authorized)`.

The target account now has the same local Ollama embedding configuration and default embedding alias. Only that embedding configuration changed; other model defaults were preserved. The one retained provider revision was copied to a private dataset in the new account, checked against the platform source size/SHA-256, parsed and verified searchable. Its internal provider mapping was replaced transactionally, preserving the immutable platform revision and generation membership. The original Object Store source was retained; the obsolete provider document copy was removed after verification. No active platform Knowledge Base existed at the account switch.

Protected backup: `/opt/agent-platform/backups/ragflow-key-change-20261002T093357Z`. The canonical server env and local protected test configuration now contain the new Key. API and Worker were recreated from the already released `main_temp` images, both were verified to have the new credential and became healthy. Server-to-ngrok-to-local Chinese SDK retrieval passed.

An ordinary User then exercised the actual platform upload/ingestion/search path with the new credential. Six recorded checks passed: authenticated upload, Object Store/Worker/RAGFlow Ready, Chinese preview with the correct revision citation, Administrator denial of the private base, indexed no-hit, and temporary resource/account/client cleanup. The uploaded revision was `645fc0c6-d003-47b8-99f8-f3819e1e91e2`. Nonsensitive evidence is stored at `/opt/agent-platform/evidence/ragflow-key-20261002/checks.json`; authentication tokens were removed. Workflow and Assistant model-answer tests were not repeated for this credential-only change; their earlier acceptance is recorded above.
