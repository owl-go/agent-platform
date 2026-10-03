# Public Assistant header removal — 2026-10-03

## Behavior and checks

The User requested removal of the Assistant name and new-conversation area from the shared chat because it occupied too much of the floating window. The public page now begins with its message thread, omitting the title, new-conversation button and header spacing. The unused reset handler, icon import and public header style were removed. Both direct public links and iframe embeds use this page. The floating widget keeps its close and reopen controls; the authenticated conversation keeps its existing header and new-conversation action.

Existing public-page tests were updated to check header absence for saved-size and embedded layouts and to retain partial output plus visitor conversation continuation after stopping. No new test file or runtime route was introduced.

Executed on both feature and integration checkouts:

- Focused page/thread suite: 16 tests in 3 files passed on the feature checkout.
- Complete frontend suite passed; integration result: 527 tests in 51 files.
- `make web-typecheck`, `make web-build` and `git diff --check` passed.

The production build retained its existing advisory warning for chunks larger than 500 kB. Backend, database/storage integration and Linux/gVisor Runtime gates were not rerun because only public frontend presentation changed.

## Release and browser acceptance

- Feature code: `461a7d6` on `codex/assistant-prompt-variables`, pushed.
- Integrated code: `425aeaa747ad42080b7afb40ace145ac2f9aecd9` on `main_temp`, pushed before release.
- Web release: `assistant-headerless-20261003-1`.
- Web path: `/opt/agent-platform/web/releases/assistant-headerless-20261003-1`.
- Protected backup: `/opt/agent-platform/backups/pre-assistant-headerless-20261003-1`. Business and identity dumps passed `pg_restore -l`; configuration, prior pointers and backup manifest were verified.

`scripts/deploy-web.sh` built with the production public OIDC values, uploaded and validated the immutable Web release, then atomically activated it. All 86 production static files matched local build bytes. Entry hashes:

- `index.html`: `7cdc318cc4cfb9fb59ea1a7db2362ca5ceeb0fcdb8af941e73a1a4d482e84353`.
- `assets/assistant-embed.js`: `5565d2d5912b7b1f4fb7e4d22881cc953c882664e36d554b4075b687d5220d73`.

A temporary localhost fixture used the actual `installAssistantWidget` implementation and current share to load the production page in a 400×600 floating window. The iframe had zero conversation-header elements, an enabled composer, the configured welcome and FAQ chips, and one widget close control. Closing reduced it to the launcher; reopening restored the same iframe and chat. A screenshot was inspected. No turn was sent, no model was invoked and no owner Credits were consumed. The temporary fixture and tab were removed; the User's share configuration and original conversation were not changed.

Public Health and Readiness returned `ok` and `ready`; OIDC discovery matched the configured issuer. Canonical configuration hashes were unchanged. API, Worker and Egress Controller remained healthy; Caddy remained running.

## Scope and rollback

API source remains `/opt/agent-platform/src.release-assistant-courtesy-20261003-1`; API image remains `sha256:bb9e7f7a36d72ddfa18080e1a9c43d72a5c8d206d2f0df288bb181364c274327`. Worker remains `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`. No migration, backend, retrieval configuration, Runtime image or persistent volume changed. Existing share links and snippets remain valid and load the updated layout on refresh.

Rollback: configure the deployment host and run `scripts/deploy-web.sh activate assistant-scroll-20261003-1`. Keep API, configuration and volumes unchanged.
