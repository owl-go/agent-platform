# Image Generation deployment verification - 2026-09-11

## Release

- Public origin: `https://47-237-108-63.sslip.io`
- Source commit: `d257c289d76e1cbd00aa720663528e35ab794ac7`
- Release: `platform-20260911T023602Z`
- Source: `/opt/agent-platform/src.release-platform-20260911T023602Z`
- Web: `/opt/agent-platform/web/releases/platform-20260911T023602Z`
- Backup: `/opt/agent-platform/backups/pre-platform-20260911T023602Z`

The official `scripts/deploy-platform.sh` ran from the clean
`codex/main-production-release` worktree with deployment gates enabled. The
release includes the production migration-history filename correction for the
AI Creation image schema.

## Validation and backup

The release passed `make test`, `make build`, the frontend test suite (202
tests across 27 files), `make web-typecheck`, the production Web build, and
`git diff --check`. Before migration, the script created custom-format business
and identity PostgreSQL dumps, a deployment configuration archive, and records
of the previous source and Web targets. Both dumps passed `pg_restore -l`; all
five backup files passed their SHA-256 checksum checks.

## Migration and service activation

The production migration ledger contains the image-generation migrations
`000035_ai_creation_image_generation.sql`, `000036_image_model_credentials.sql`,
and `000037_fixed_image_credit_rate.sql`. Source and Web symlinks resolve to the
release paths above. API, Worker, and Egress Controller were recreated from the
release and report healthy.

- API image: `sha256:542c417e10b97ab94211c8de98961b9f117bd0400a5de5563eda27977a5d5ec6`
- Worker image: `sha256:0424ed355b6f9462ceff72b092c9e735237b9ffdc430f84ce874272297ea4ea6`
- Egress Controller image: `sha256:6f8ccd9b77ee9eb138cd8850d3c2c336d0ad4f1993771210052786e67251f5a6`
- Caddy image: `sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d`
- Web entry: `/assets/index-DUpBW9Fr.js`
- Web entry SHA-256: `b997c8292e5e9bd3293c9a1b8d30c9916564282846d131ca925b785da6ebb297`

## Public verification

- `/api/healthz` returned HTTP `200` with `{"status":"ok"}`.
- `/api/readyz` returned HTTP `200` with `{"status":"ready"}`.
- OIDC discovery returned HTTP `200` through the public TLS origin.
- HTTP requests redirect to HTTPS with status `308`.
- API, Worker, Egress Controller, PostgreSQL, identity database, and MinIO
  health checks report healthy; Caddy and Keycloak are running.
- Service logs since replacement contain no panic, fatal, or error-level entries
  according to the deployment script's log check.

## Evidence boundary

This verifies the committed release, backup integrity, migration activation,
service health, and public Web/API availability. It does not establish live
third-party Image Provider credentials, authenticated image-generation output,
new Runtime image conformance, Linux sandbox conformance, or Aliyun OSS
integration evidence; those remain explicit environment-dependent gates.
