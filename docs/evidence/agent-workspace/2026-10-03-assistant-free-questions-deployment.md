# Shared Assistant free questions — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Behavior and release

The Share Configuration editor no longer exposes “每日自由提问上限” or “允许自由提问”. Every enabled, currently validated share permits free-text input without a per-Assistant daily call cap. Previously saved `free_text_enabled=false` and finite or exhausted daily limits are ignored immediately on reads and public requests, without saving the Assistant, changing its version, or rotating its Token. New saves normalize the two legacy JSON fields to `true` and `0` for client compatibility. Publication validation no longer requires a positive daily cap. Both public SSE and the legacy answer route remove the free-text and daily-counter gates; the obsolete quota adapter is removed. Existing platform rate controls, safety, owner Credits, allowed origins and data-processing acknowledgement still apply.

- Feature commit: `f16a9a5` on `codex/assistant-prompt-variables` (pushed).
- Integrated release commit: `4c153a27282ce86ff3427b698da762ed20106c07` on `main_temp` (pushed before deployment).
- API/Web release: `assistant-free-questions-20261003-1`.
- Source: `/srv/agent-workspace/src.release-assistant-free-questions-20261003-1`.
- Web: `/srv/agent-workspace/web/releases/assistant-free-questions-20261003-1`.
- API image: `sha256:0dca4d7ba959be29dc483f54ca8aaa1c494531abceb0ad97c98cd20505f7c56e`.
- Verified backup: `/srv/agent-workspace/backups/pre-assistant-free-questions-20261003-1`.
- Previous API/source: `assistant-greeting-20261003-1`; previous Web: `assistant-chat-20261003-1`.

## Executed checks

On the feature and integrated release checkouts, targeted AI Application domain/application/data and workspace service tests, `make test`, `make build`, frontend tests, `make web-typecheck`, `make web-build`, and `git diff --check` passed. The integrated frontend suite passed 512 tests in 48 files after a frozen dependency install. Focused share-editor/public-page tests passed 15 tests.

Regressions cover old disabled flags plus an exhausted two-call cap accepting three consecutive greeting and business turns, visitor continuation, zero daily-counter calls, enabled public metadata, the legacy FAQ answer route, normalized read/create/update configuration, cap-free publication checks, editable free-text input even with legacy metadata, and saving sharing with no daily cap. Existing rate refusals, safety, FAQ identity matching, stale validation, visitor isolation, streaming and cancellation remain covered. Fake models were used; this is not live Provider acceptance.

The business and identity PostgreSQL dumps passed `pg_restore -l`. Protected configuration, previous source/Web/API/Worker pointers and backup SHA-256 manifests were verified on the host. Only tracked integrated source was uploaded. The API was prebuilt with the existing service Dockerfile, activated via the production Compose stack and a release-specific host-only image override, then verified healthy before switching the source pointer. `scripts/deploy-web.sh` built with production OIDC values, validated the immutable upload and atomically activated Web. No applicable local gate was skipped.

Public Health and Readiness returned HTTP 200 with JSON statuses `ok` and `ready`; OIDC discovery matched the configured issuer. All 85 production static files returned successfully and matched the local production bytes. Remote entrypoint hashes matched too:

- `index.html`: `7f3431af2142aabb5db82dd598bb2e2c010f764e1519632d7c600ae600afbb68`.
- `assets/assistant-embed.js`: `8d5d3154bef2d046c0aca76880565d0f992bfd7c045b6e8ced29e6f9de85a782`.

API startup error count was zero. Worker and Egress Controller remained healthy and Caddy remained running. Canonical environment/YAML SHA-256 checks confirmed unchanged bytes. The latest installed migration remains `000067_ragflow_knowledge_generations.sql`; this change adds no migration.

## Boundaries and rollback

This scoped release changed only API and Web. Worker remains on image `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`; Runtime/CLI Builder images, Caddy, identity, Egress, RAGFlow configuration and persistent volumes were retained. The full platform script was not invoked because it would rebuild unrelated execution images and perform unrelated retired-volume cleanup; its applicable service/Web gates, verified backups, immutable release and activation checks were executed explicitly.

Six environment-dependent GORM/PostgreSQL integration tests skipped because `WORKSPACE_TEST_POSTGRES_DSN` was not set; these skips are not PostgreSQL acceptance. Runtime image smoke, Linux/gVisor Production Conformance and real cross-site browser/model acceptance were not run. No current valid user-supplied Share Token was available for a live conversation, so successful model answers on the User's current share are not claimed. No private Assistant configuration, visitor transcripts, Knowledge content or credentials were inspected or modified for acceptance. Historically revoked shares are not restored.

Web rollback uses `scripts/deploy-web.sh activate assistant-chat-20261003-1`. API rollback uses the previous immutable source and API image recorded in the protected backup. Schema/configuration are unchanged; retain persistent volumes.
