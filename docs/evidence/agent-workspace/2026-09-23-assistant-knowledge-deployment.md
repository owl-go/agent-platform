# Assistant conversations and Knowledge Base search deployment — 2026-09-23

## Release

- Integration branch: `main_temp`, source commit `4c2eeea8c7781561a1f496317a42bc80022e2fec`.
- Included branches: `codex/dedicated-assistant-chat` (`f64c136`) and `codex/knowledge-search` (`83ddcfa`, including implementation commit `990af45`).
- Release ID: `platform-20260923T114405Z-4c2eeea`.
- Source: `/opt/agent-platform/src.release-platform-20260923T114405Z-4c2eeea`.
- Web: `/opt/agent-platform/web/releases/platform-20260923T114405Z-4c2eeea`.
- Verified pre-cutover backup: `/opt/agent-platform/backups/pre-platform-20260923T114405Z-4c2eeea`.
- Public origin: `https://47-237-108-63.sslip.io`.

The clean integration worktree merged both branches without conflict. The official `scripts/deploy-platform.sh` ran with deployment gates enabled, not `SKIP_DEPLOY_GATES=1`. The pre-cutover backup includes custom-format business and identity PostgreSQL dumps, configuration, and previous source/Web targets; `pg_restore -l` and the backup SHA-256 checks passed before cutover.

## Checks completed

- Before deployment, `make test` passed with a disposable PostgreSQL DSN, including the Assistant Conversation concurrency/audit and Knowledge Base source-authorization integration tests. The disposable PostgreSQL container was stopped afterward.
- `make build`, `make web-typecheck`, `make web-build`, and the full frontend suite passed. The frontend suite had 262 passing tests across 40 files.
- The deployment script reran its Go test/build, frontend test/typecheck, and production Web build gates successfully, then confirmed migration `000053_assistant_conversations.sql` was applied.
- The new API, Worker, Egress Controller, Caddy, and AnythingLLM containers were activated. The script verified their health/status, public `/api/healthz` and `/api/readyz` responses, OIDC discovery, HTTP-to-HTTPS redirect, and no panic/fatal/error-level entries in the checked service logs.
- An independent public check returned `{"status":"ok"}` and `{"status":"ready"}`. The public `index.html` SHA-256 matched the local production build: `2619250aa13caa72cdf503a387cb7b1543570208295d9acf86b1390bc36e9dec`.
- Without a token, the new Knowledge Base search and Assistant Conversation routes each returned HTTP `401`. The two corresponding deployed service source files matched the integration checkout byte-for-byte by SHA-256.

## Evidence boundary

This proves the merged release, backup integrity, migration activation, serving of the new Web build, route protection, and deployment health. It does **not** prove an authenticated end-to-end Assistant model response or a real uploaded-document-to-AnythingLLM-search round trip. The latter remains explicitly unverified in `docs/testing/2026-09-23-knowledge-base-search.md`; frozen historical Knowledge Index Generation selection is also not supported by the current AnythingLLM client. Do not present these as production-accepted behaviors until exercised with a configured account and provider.
