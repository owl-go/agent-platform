# Assistant reply visibility and scrolling — 2026-10-03

## Diagnosis and behavior

The shared Assistant Conversation thread had no scrolling ownership. Authenticated conversations left the thread at its previous scroll position while responses grew; public conversations separately assigned scrollTop after events, without handling viewport/composer resizing or preserving a reader's position above the bottom.

A temporary browser fixture rendered the real authenticated page with deterministic streamed answers. Before the fix, the last paragraph ended at y=1373.375 while the thread and composer boundary was y=579, with scrollTop=0 and maximum scroll=859. The answer existed but was below the visible message area. After the fix, the same fixture reached scrollTop=859 and the paragraph ended at y=514.375, above the composer. No live Provider, Knowledge content or visitor conversation was used for this fixture.

Scrolling now belongs to the shared `AssistantConversationThread`. New questions reveal the latest turn. Streamed and final answers follow the bottom while the reader remains there. Scrolling up pauses following; returning to the bottom or sending another question resumes it. A ResizeObserver preserves the latest position when the thread size changes, including iframe resizing and growing input fields; image-load reflow also reveals the latest content when following. The observer disconnects on unmount. Public page-specific component `$el` scrolling was removed.

## Executed checks

- New focused component tests failed before the fix and passed after it. They exercise rendered question/delta/final changes, history reading, new-question following, resizing and observer cleanup.
- Focused component/authenticated/public page tests: 16 tests in 3 files passed on the feature and integration checkouts.
- Feature frontend suite: 446 tests in 51 files passed.
- Integrated frontend suite: 527 tests in 51 files passed.
- Both checkouts passed `make web-typecheck`, `make web-build` and `git diff --check`.
- Browser verification used real page components and deterministic fake streams. Both authenticated and public surfaces displayed the final answer above the composer. At 375×360, the authenticated final paragraph remained visible after shrinking the viewport and growing the question field to six lines; the settled scroll gap was zero. The public final paragraph ended at y=148.375, above the composer at y=213, with zero scroll gap and no horizontal overflow. A screenshot was inspected. Temporary fixture files and its browser tab were removed; viewport override was reset.

## Release and acceptance boundary

- Feature code: `21b786a` on `codex/assistant-prompt-variables`, pushed.
- Integrated code: `6c0613592c57fcb86d00db4840d83ea0162d9603` on `main_temp`, pushed before deployment.
- Web release: `assistant-scroll-20261003-1`, built and activated through `scripts/deploy-web.sh` with production OIDC settings.
- Web path: `/opt/agent-platform/web/releases/assistant-scroll-20261003-1`.
- Verified protected backup: `/opt/agent-platform/backups/pre-assistant-scroll-20261003-1`. Business and identity dumps passed `pg_restore -l`; configuration, prior pointers and SHA-256 manifest were verified.

Public Health and Readiness returned HTTP 200 with statuses `ok` and `ready`; OIDC discovery matched the configured issuer. All 86 production static files returned HTTP 200 and matched local build bytes. Remote entrypoint hashes matched:

- `index.html`: `815b2f8f1986f32c08356bdd30cdb8aa893dd3192cd3d7f9b44a2dd27d4ce254`.
- `assets/assistant-embed.js`: `599746e7fa27b2b150969c8a9a38bc1c061e97d1364e7a5f5ef6dee918773b45`.

API, Worker and Egress Controller remained healthy; Caddy remained running. Canonical environment and YAML bytes were unchanged.

Only Web changes. API source remains `/opt/agent-platform/src.release-assistant-embed-types-20261003-1`; API image remains `sha256:73f33c016e1c633851feba773f8cf7f207aac9b66809586c194d36a96646b90e`, and Worker remains `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`. No backend, schema, Runtime or storage implementation changed; their environment-specific gates were not rerun. No User-owned share configuration or private conversation was changed for verification. Live Provider acceptance is not claimed.

Rollback: `scripts/deploy-web.sh activate assistant-share-save-20261003-1` with the deployment host configured. Keep configuration and persistent volumes.
