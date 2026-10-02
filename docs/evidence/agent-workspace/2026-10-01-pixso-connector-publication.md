# Pixso connector verification — 2026-10-01

## Package and protocol evidence

- Source `pixso`, version `1.0.0`, MCP Streamable HTTP endpoint `https://pixso.net/mcp`, OAuth `mcp:connect`, Egress `pixso.net` only.
- Current `connectorpackage.Parse` validated the final ZIP via `go -C backend run ./cmd/connector-package-validate ../outputs/pixso/pixso-1.0.0.zip`: normalized SHA-256 `c6c388adad99a375af6dd327fbacaadb216c4e588312395fc9adffe22b3c52c4`, one Skill, no CLI bundle.
- Official authorization-server and protected-resource metadata returned 200 and declared S256, authorization code, refresh token, dynamic public-client registration and Bearer header transport. Unauthenticated MCP initialize returned 401 with the protected-resource metadata challenge.
- Pixso accepted dynamic public-client registration using the actual platform callback `https://47-237-108-63.sslip.io/api/v1/connectors/pixso/oauth/callback`; matching redirect URI and `token_endpoint_auth_method=none` were checked without persisting client identifiers or credentials.
- Before publication, Administrator catalog and the current Administrator's User Installation catalog contained zero Pixso entries.
- Brand PNG is the original 32×32 Pixso official website favicon converted from ICO without scaling; SHA-256 `150c8008f92829a03004e50080baa89c7aeb92fbba53cb09de953ac8b40b2816`. Package SVG embeds the same pixels; platform presentation supplies a supported PNG data URL.

## Executed checks

- `gofmt` on modified Go files; `git diff --check`.
- Target package tests for Pixso OAuth, workspace service, package parser/presentation, GORM repositories and Runtime MCP credential configuration.
- `python3 -B -m unittest discover -s scripts/connectors/pixso -p 'test_*.py'`: five publication checks passed.
- Disposable PostgreSQL 16: `TestPixsoMCPSnapshotUsesAuthorizationAADAndExcludesRefreshMaterial` passed. It checks encrypted authorization AAD, owner isolation, expiry and refresh exclusion.
- `WORKSPACE_TEST_POSTGRES_DSN=<disposable PostgreSQL DSN> make test` and `make build` passed, including database-backed integration tests.
- `pnpm --dir frontend test`: 428 tests across 46 files passed; Pixso browser launch and upgrade behavior included.
- `make web-typecheck` and `make web-build` passed.

## Publication and deployment

- Feature commit `7fda071` was pushed on `codex/pixso-connector`, then merged and pushed to `main_temp` as `b2ec8b5` before deployment. No `main` push was performed.
- Release `pixso-connector-20261001T091800Z` deployed successfully via `scripts/deploy-platform.sh` from that integration checkout. Completed local gates were reused with `SKIP_DEPLOY_GATES=1`; tests and builds listed above were actually executed before release. The script created verified database/config/source/Web backups, built services and Web, reverified installed CLI bundles against the candidate Runtime, and completed deployed health checks.
- API image `sha256:1b2c9791352d6046f3f183025623baef62e758cf60b07f71fe622025fdf99fc2`; Worker image `sha256:ed9a4186033b240b4460374a1fb6bb9c6638de72231f732e879c70a029256422`.
- Pixso publication is `available`, publication version 1, active Revision `61e48202-e15e-4c09-b728-fd6e83c7dee9`, exact normalized package digest above.
- `publish.py` checked the deployed callback returns 400 for missing state with `Cache-Control: no-store` and `Referrer-Policy: no-referrer`, and Pixso accepted DCR using that actual callback. User catalog and Administrator catalog each had exactly one formal Pixso Publication with its brand PNG. Publication was also rerun to verify it reuses the exact revision.
- Playwright browser check under the deployment Administrator's User identity showed one Pixso entry in the design category, its real brand image, version 1.0.0 and the explicit package-validation label. A temporary Installation was created; before authorization its dialog exposed an actionable “连接” button. Clicking it launched the official Pixso browser login, and the platform displayed waiting-for-authorization plus “打开授权页面”; no manual Token form was used. Screenshots `output/playwright/pixso-details.png` and `pixso-installed.png` remain local and ignored by Git.
- The temporary Installation was uninstalled after the browser-entry check; no Pixso account grant or design mutation was created. Disposable PostgreSQL and temporary browser-login material were removed. MCP packaging created no temporary CLI Definition or bundle build resource.

## Remaining evidence

Real Pixso account authorization, authenticated tools/list, business tools/call
and real-account refresh were not run: browser verification stopped at Pixso
login. Package validation, DCR and a working browser entry do not establish those
results. No Pixso Linux + gVisor model Runtime, complete Production Conformance,
remote object-store Conformance or real design write was run. Existing CLI bundle
reverification during deployment is not Pixso MCP Runtime Conformance. Users must
install the published Connector and connect their own Pixso accounts; publication
does not authorize them or silently upgrade their existing Installations.
