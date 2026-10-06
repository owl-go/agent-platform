# Share settings saved before embed code generation — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Diagnosis and behavior

The User's current embed URL returned HTTP 200 with `Content-Security-Policy: ... frame-ancestors https://www.baidu.com` and `X-Frame-Options: DENY`. It did not permit the local preview origin `http://localhost:4177`. The response headers were inspected through a temporary local diagnostic form without recording the Share Token, URL path, response body or visitor content.

The Share Configuration editor also had a save-flow defect: generating a new Share Token used the persisted Assistant version without saving edited allowed origins, dimensions or floating-window settings. A regression test reproduced this before the fix: token generation made zero configuration-save calls.

The action is now labelled “保存并生成新的分享 Token” / “Save and generate new share Token”. It validates and saves the draft first, including any pending chat icon, then generates a Token against the returned saved version. First-time enablement reuses the Token issued by the save. Save failure prevents rotation; rotation failure after a successful save preserves the saved revision and clears the displayed Token/code. Ordinary Save retains its existing behavior.

The local preview uses the same iframe sandbox for both policy cases. A temporary Chromium fixture with localhost in `frame-ancestors` loaded visibly, even with `X-Frame-Options: DENY`; a fixture permitting only Baidu displayed the connection-refused surface. This supports the allowed-origin diagnosis and is not live Provider acceptance. Temporary diagnostic tabs and the port 4188 server were closed. The User's local preview on port 4177 remains available and its pasted code was preserved.

## Verification and deployment

- Feature code: `ec03613` on `codex/assistant-prompt-variables`, pushed.
- Integrated code: `ffc6b7b05bcdc3673aa96c7bfc18a2ca602f4d7f` on `main_temp`, pushed before deployment. The locale merge retained existing Connector strings and added local-preview translations.
- Web release: `assistant-share-save-20261003-1`, activated through `scripts/deploy-web.sh` after a production OIDC build.
- Web path: `/srv/agent-workspace/web/releases/assistant-share-save-20261003-1`.
- Protected verified backup: `/srv/agent-workspace/backups/pre-assistant-share-save-20261003-1`. Both database dumps passed `pg_restore -l`; configuration, prior release pointers and the SHA-256 manifest were verified.

Focused tests passed 24 tests in 3 files on both feature and integration checkouts. Full frontend suites passed 443 tests in 50 files on the feature and 524 tests in 50 files on `main_temp`. Both checkouts passed `make web-typecheck`, `make web-build` and `git diff --check`; integration dependencies were installed with the frozen lockfile. Tests cover saving edited origins before rotation, using the saved version, refusing generation after failed save, avoiding duplicate first-enable rotation, and retaining the saved revision without displaying stale code when rotation fails.

Public Health and Readiness returned HTTP 200 with statuses `ok` and `ready`; OIDC discovery matched the configured issuer. All 86 production static files returned HTTP 200 and matched local build bytes. Remote entrypoint hashes matched the build:

- `index.html`: `77771bc5457c1a374fd00a8a4af0ff56ee7eb0a2ef84f00c33f73305b4c01f23`.
- `assets/assistant-embed.js`: `9f5ee9d3541655cd05cfc2775af93dfe97a107ebfa28815f0e9774e14ea6f6b6`.

## Scope, acceptance and rollback

This was a Web-only release. The API source pointer remains `/srv/agent-workspace/src.release-assistant-embed-types-20261003-1`; API image remains `sha256:73f33c016e1c633851feba773f8cf7f207aac9b66809586c194d36a96646b90e` and Worker remains `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`. API, Worker and Egress Controller remained healthy; Caddy remained running. Canonical environment and YAML bytes were unchanged. No backend, migration, Runtime or storage implementation changed; their environment-specific gates were not rerun.

The local paste-and-run preview is a development entrypoint and is excluded from the production build. Its implementation is recorded in feature commit `59fd011` and is included in the integration source.

No User-owned production Share Configuration or whitelist was modified for verification. The User must refresh the management page, include `http://localhost:4177` in allowed origins, save/generate using the corrected action, and paste the resulting code into the local preview. The previously inspected Token still had the Baidu-only policy at diagnosis time; successful live cross-site chat after that User save has not been verified. Existing allowed-origin, rate, safety and owner Credits checks remain enforced.

Web rollback: `scripts/deploy-web.sh activate assistant-embed-types-20261003-1` with the deployment host configured. Retain configuration, source pointers and persistent volumes.
