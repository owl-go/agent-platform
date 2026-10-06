# Fullscreen and floating Assistant embedding — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Behavior and release

Share Configuration now selects fullscreen iframe or a floating window. The floating window starts expanded or showing only its chat icon. Owners may select PNG, JPEG, WebP or GIF files up to 2 MiB for an independent chat icon, preview locally, save configuration and image together, or restore the default icon. Cancelling does not upload or modify saved settings. Saving enabled sharing keeps the editor open for generating and copying the embed code. The public image proxy validates current publication, Share Token, origin and owner; generated code does not expose private Object Keys. Existing shares default to fullscreen embedding without Token rotation.

The generated floating snippet uses an isolated Shadow DOM on the host site and embeds the existing public chat surface. It bounds width and height to the viewport. Collapsing and reopening retains the same iframe, visitor conversation and stream. Fullscreen fills its host container and retains configured minimum height. The chat icon remains independent of the Assistant avatar. Ordinary Assistant edits preserve the new share fields. No new theme, locale, avatar-visibility or streaming switches were added.

- Feature commit: `8a1645c` on `codex/assistant-prompt-variables` (pushed).
- Integrated release commit: `ae364b6dcac25606dccd3ebef12831075f8b9f06` on `main_temp` (pushed before deployment).
- API/Web release: `assistant-embed-types-20261003-1`.
- Source: `/srv/agent-workspace/src.release-assistant-embed-types-20261003-1`.
- Web: `/srv/agent-workspace/web/releases/assistant-embed-types-20261003-1`.
- API image: `sha256:73f33c016e1c633851feba773f8cf7f207aac9b66809586c194d36a96646b90e`.
- Verified backup: `/srv/agent-workspace/backups/pre-assistant-embed-types-20261003-1`.
- Previous API/source/Web: `assistant-free-questions-20261003-1`.

## Executed checks

Feature and integrated checkouts passed targeted AI Application domain/application/data, workspace service and server tests, `make test`, `make build`, `make web-typecheck`, `make web-build`, and `git diff --check`. The feature frontend suite passed 437 tests in 49 files; focused final embed and share-editor tests passed 18 tests after the final save-flow adjustment. The integrated frontend suite passed 518 tests in 49 files after a frozen dependency install.

Regressions exercise legacy fullscreen defaults, JSON persistence of embed settings and independent icon, allowed and rejected image namespaces, exact bounded multipart routes, actual generated snippet execution, both initial widget states, same-iframe reopening, duplicate prevention, script-termination escaping, fallback icon, deferred upload and cancelled edit. Service tests verify owned and public image bytes, avatar preservation, version-bound publication after save, wrong-owner/stale-version/invalid-format/oversize rejection, compensation after failed writes, and preserving a committed object's pointer when publication recording fails. The latter leaves public access closed by stale validation.

A temporary production-minified Vite fixture ran in Chromium through Playwright CLI with the real share editor and public chat components plus deterministic API/stream fixtures. Generated snippets executed successfully. Desktop floating geometry was 400×600; at a 375×667 viewport, the panel was 351×571 and iframe 349×539 with no horizontal overflow. Both initially collapsed and expanded presentations were inspected. Fullscreen filled a 375×700 containing area. A partial response was observed before completion; closing and reopening retained the conversation and final output. In the mobile share editor, selecting an image made no upload; Save submitted the image and configuration, with the Save button inside the viewport. Screenshots were inspected. Temporary fixture source was removed and is not part of the release. This is browser evidence using fake services, not live Provider or cross-site deployment acceptance.

Business and identity database dumps passed `pg_restore -l`; protected configuration, source/Web/API/Worker pointers and SHA-256 backup manifests were verified on the host. Only tracked integrated source was archived and uploaded. The API was prebuilt and activated through the production Compose stack with a host-only release override. Its health passed before switching the source pointer. `scripts/deploy-web.sh` built production OIDC assets and atomically activated the verified immutable Web release.

Public Health and Readiness returned HTTP 200 with JSON statuses `ok` and `ready`. OIDC discovery matched the configured issuer. All 86 production static files returned successfully and matched local build bytes; remote entrypoint hashes matched:

- `index.html`: `aa5dd3c013f01873943e7063a7b9b7edc0f2cafb8229eaf052f53e5f05449735`.
- `assets/assistant-embed.js`: `aeca96b2861854f4a33efab3fc39c3cbac77390b147f75721338db1fc5ab6fbc`.

API startup ERROR count was zero. API, Worker and Egress Controller remained healthy; Caddy remained running. Canonical environment and YAML bytes were unchanged. Migration ledger latest remains `000067_ragflow_knowledge_generations.sql`; no migration was introduced.

## Boundaries and rollback

Only API and Web were updated. Worker remains on image `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`; Runtime and CLI Builder images, Caddy, identity, Egress, RAGFlow configuration and persistent volumes were retained. The full platform script was not invoked because it rebuilds unrelated execution images and performs unrelated retired-volume cleanup; its applicable service/Web gates, verified backups and immutable activation checks were executed explicitly.

Six environment-dependent PostgreSQL integration tests skipped without `WORKSPACE_TEST_POSTGRES_DSN`; these are not PostgreSQL acceptance. Real object-storage integration, Runtime image smoke, Linux/gVisor Production Conformance and a live cross-site chat with the User's current valid Share Token were not run. No private Assistant configuration, visitor transcript, Knowledge content or credential was inspected or modified for acceptance. Existing rate, safety, owner Credits and allowed-origin controls remain enforced.

Web rollback uses `scripts/deploy-web.sh activate assistant-free-questions-20261003-1`. API rollback uses the previous immutable source and image in the protected backup. Schema and canonical configuration are unchanged; retain persistent volumes.
