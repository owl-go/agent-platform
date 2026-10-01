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

## Publication and remaining evidence

Publication is pending deployment of the tested `main_temp` integration. This
record will be updated with actual revision, catalog, deployment and browser
results after execution. Package validation and public endpoint checks do not
establish real Pixso account authorization, authenticated tools/list or
business tools/call. No Linux + gVisor model Runtime, complete Production
Conformance, remote object-store Conformance or real design write was run.
