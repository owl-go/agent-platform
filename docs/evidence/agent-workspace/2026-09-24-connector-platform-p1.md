# Connector Platform P1 verification — 2026-09-24

## Verified implementation

- Administrator staging and publication require a parsed immutable package plus a passed `cli_connector_conformance` row for the exact bundle SHA-256 and Runtime RepoDigest.
- User installation, upgrade, setup, multiple external account authorizations, account selection, refresh-token rotation, disconnect, and recovery use the Connector Installation boundary. Credential associated data binds the User, Installation, and external account; disconnect and uninstall clear access and refresh credential ciphertext.
- Runtime snapshots freeze the Revision, package/bundle identity, Installation, and selected Authorization. The Worker revalidates the current Publication, Installation, Authorization, policy, Runtime Digest, and approval before starting the process.
- Migration `000056_feishu_connector_installation_projection.sql` installs an idempotent publication trigger. A real non-legacy Feishu publication projects retained applications, active account and refresh tokens, and future Expert bindings even when publication occurs after database migration. Historical snapshots and legacy tables remain unchanged.
- The Web catalog presents each published Connector once and includes install, setup, account selection, authorization refresh/disconnect, upgrade, disable, and uninstall actions.

## Checks completed

- `GOTOOLCHAIN=go1.26.8 make generate` completed, and a second generation produced an identical diff.
- `WORKSPACE_TEST_POSTGRES_DSN=<disposable PostgreSQL 16 DSN> GOTOOLCHAIN=go1.26.8 make test` passed. This exercised the complete migration chain, publication Conformance lookup, multiple account selection, authorization CAS refresh, Feishu setup flows, and publication-triggered legacy projection. The disposable database container was removed afterward.
- `GOTOOLCHAIN=go1.26.8 make build` passed.
- `make web-typecheck` and `make web-build` passed.
- `pnpm --dir frontend test` passed: 271 tests across 40 files.
- `git diff --check` passed.

## External evidence boundary

`make production-conformance-preflight` was run and failed closed because this host is macOS, `runsc` and the controlled Egress network are unavailable, and the required Runtime image, provider credential, object-store, repository, and evidence-root variables are not configured. Production Conformance was therefore not run.

The deployed Linux host does have the `runsc` Docker runtime and `agent-public-egress` network. A second preflight there still failed closed because the Conformance repository/work/evidence roots, resolver file, sandbox test URLs, five immutable Runtime images/models/credential directories, Aliyun OSS credentials, and MinIO API credentials are not configured. No model or provider call was started.

No official Feishu package was staged or published during this verification, so there is no package SHA-256, bundle SHA-256, Runtime RepoDigest, provider registration, OAuth account, or Linux sandbox execution result to record. The service rejects publication without exact locally recorded Conformance evidence. The P1 implementation can be deployed, but the official catalog entry must remain unavailable until a Linux + `runsc` Conformance run supplies those exact Digests and an Administrator stages and publishes that resulting archive.

## Deployment

- Integration commit: `6d1f01c` on `main_temp`.
- Release: `platform-20260924T094307Z`.
- Source: `/opt/agent-platform/src.release-platform-20260924T094307Z`.
- Web: `/opt/agent-platform/web/releases/platform-20260924T094307Z`.
- Pre-cutover backup: `/opt/agent-platform/backups/pre-platform-20260924T094307Z`.
- Public origin: `https://47-237-108-63.sslip.io`.

The deployment script ran its test, build, frontend test/typecheck/build, backup-integrity, image-build, migration, cutover, and service-health gates without bypasses. Independent checks returned `{"status":"ok"}` and `{"status":"ready"}`. The public and local `index.html` SHA-256 values both equal `ff722d1aaa22d20a268f695bd8e58a34e715b94e7e90bd81bcf63953ef25c568`. API, Worker, Egress Controller, Caddy, the then-configured external retrieval service, PostgreSQL, identity, and MinIO containers were running; health-enabled services reported healthy. Migrations `000054_connector_package_publications.sql`, `000055_connector_feishu_authorization.sql`, and `000056_feishu_connector_installation_projection.sql` were present in `schema_migrations`.

The production database contained zero active `feishu` Publications after deployment. This is intentional: application deployment does not substitute for the missing exact Linux + `runsc` package Conformance evidence described above.
