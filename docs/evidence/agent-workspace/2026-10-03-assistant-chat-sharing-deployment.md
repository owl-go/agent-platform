# Assistant chat and sharing deployment — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Release

- Feature branch: `codex/assistant-prompt-variables`, through `af9f813`.
- Integrated and pushed `main_temp`: `0a004334c80db45075595d38eb7ba5d0e7790db1`.
- Release: `assistant-chat-20261003-1`.
- Source: `/srv/agent-workspace/src.release-assistant-chat-20261003-1`.
- Web: `/srv/agent-workspace/web/releases/assistant-chat-20261003-1`.
- Public origin: `https://workspace.example.com`.
- Verified backup: `/srv/agent-workspace/backups/pre-assistant-chat-20261003-1`.
- Previous source: `src.release-ragflow-knowledge-20261002-4`; previous Web: `knowledge-controls-20261002-1`.
- API image: `sha256:325ab54efa37077b53c8fdc24d5c37fdabcd2a987f21dee017f7d670459fe8c9`.
- Worker image: `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`.

The release includes single-pass prompt variables, direct FAQ and identity handling, excerpt-backed scope review, reactive incremental chat updates, a shared message-thread component for public embeds, a public SSE route, and Share Configuration editor fixes. The integration conflict preserved both current RAGFlow configuration documentation and the new Smart Assistant scope-review documentation.

## Checks and activation

- Targeted Go service/server tests, `make test`, `make build`, frozen frontend dependency installation, frontend tests (511 tests across 48 files), `make web-typecheck`, `make web-build`, and `git diff --check` passed on the integration checkout.
- Business and identity database dumps passed `pg_restore -l`; configuration and previous source/Web/image pointers were backed up with verified SHA-256 manifests on the host. No backup credentials were downloaded.
- Tracked source was uploaded from the integrated Git commit. API/Worker images were prebuilt using the existing service Dockerfile and release-specific image tags in a host-only Compose override. Worker was stopped before replacing API; both new services became healthy before activating the source pointer.
- Migration `000067_ragflow_knowledge_generations.sql` remains installed. This release adds no migrations. Canonical deployment environment/YAML SHA-256 checks confirmed unchanged bytes after activation.
- `scripts/deploy-web.sh` performed the production OIDC build, immutable upload, file validation and atomic current-pointer switch. No test/build gates were skipped.
- Public Health and Readiness returned HTTP 200 with `ok` and `ready`; OIDC discovery returned the configured issuer. HTTP entry redirects to HTTPS with 308. API/Worker startup error counts were zero; Egress Controller remained healthy and Caddy remained running.
- Public `index.html`, every entrypoint-referenced static asset and `assets/assistant-embed.js` returned HTTP 200 and matched local production bytes. Remote current files matched as well.
- Entrypoint SHA-256: `2996c0bf05584632f27b94a1443091db542d576d5d7698fafbd9aa9aed4ed2e3`.
- Embed entry SHA-256: `20deac988e7c24710e8d289b3870b266b4b73344bf1586a71ec4e87c4e1b7e22`.

## Boundaries and rollback

This scoped release replaced API, Worker and Web. Runtime/CLI Builder images, Egress Controller, Caddy, identity configuration, RAGFlow configuration and persistent volumes were retained. The full platform deployment script was not run because it rebuilds unrelated Runtime/CLI Builder images; equivalent applicable service/Web gates, backups and activation checks were executed explicitly. Runtime smoke and Linux/gVisor Production Conformance were not rerun.

Before deployment, isolated local browser fixtures verified 640×600 direct rendering, incremental Markdown before completion, a scrolling 320×600 iframe and stationary composer without horizontal overflow. Public streaming, cancellation, failed partial output, visitor continuation, token-revision/visitor isolation and quota refusals passed service/component regressions. These are fixture checks, not real model or cross-site browser acceptance. The older share URL transcribed from the supplied screenshot returned 404; no successful production conversation or live cross-site embed is claimed. Private owner settings, knowledge data and credentials were not inspected or modified for acceptance.

The previous immutable Web can be selected with `scripts/deploy-web.sh activate knowledge-controls-20261002-1`. Service rollback can use the previous source and image pointers in the protected backup; configuration and schema were unchanged by this release. Persistent volumes must be retained.

## Follow-up: basic greetings

Feature `401b062`, integrated and pushed as `main_temp` `5c7dd5b6d3f8ab8eab18bb27410b6a9a593481ab`, handles complete basic greetings before model scope classification, after platform safety and direct FAQ matching. It returns the current public welcome or a name-only greeting fallback. Greetings with an additional task still enter the existing full-request path.

The regression first reproduced `你好呀` returning the fixed scope refusal in authenticated and public execution, with one classification call, one Knowledge search and a Credit admission. After the fix both tests pass with no model, search or Credit admission. Tests also cover normalized greetings, extended requests, configured greeting FAQ precedence, public welcome-only output, the public free-text gate and the persisted SSE terminal answer. Targeted service/server tests, `make test`, `make build` and `git diff --check` passed on both feature and integrated branches.

API-only release `assistant-greeting-20261003-1` uses source `/srv/agent-workspace/src.release-assistant-greeting-20261003-1` and image `sha256:ffa188c88cf6440b1bc7ccd8aa533e78c7cdb5a5633600993fbb468d793d6e21`. Verified business dump/configuration and previous source/Web/API pointers are retained in `/srv/agent-workspace/backups/pre-assistant-greeting-20261003-1`. The API became healthy with zero startup errors; public Health/Readiness returned 200 with `ok`/`ready`, and environment/YAML SHA-256 checks remained unchanged. Worker and Web remain on `assistant-chat-20261003-1`; neither was replaced, and no migration or Runtime change was made. The older supplied public token returned 404 on the attempted greeting acceptance, so a completed production greeting on the User's current link is not claimed. No private settings, transcripts or Knowledge content were inspected. No frontend or Runtime gates were rerun for this API-only change.
