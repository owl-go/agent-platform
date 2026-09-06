# Conversation resource selection deployment verification - 2026-09-06

## Release

- Public origin: `https://47-237-108-63.sslip.io`
- Source commit: `58328495c26b5e405b3a4aebce51f14626d2b6b9`
- Release: `conversation-resources-5832849-20260906`
- Source: `/opt/agent-platform/src.release-conversation-resources-5832849-20260906`
- Web: `/opt/agent-platform/web/releases/conversation-resources-5832849-20260906`
- Backup: `/opt/agent-platform/backups/pre-conversation-resources-5832849-20260906`
- Previous source: `/opt/agent-platform/src.release-platform-20260906T050137Z`

The official `scripts/deploy-platform.sh` ran successfully from a separate detached worktree at the source commit. Local browser artifacts and unrelated working-directory files were not included. Deployment gates were enabled.

## Validation and backup

The release worktree passed `pnpm --dir frontend install --frozen-lockfile`, `make test`, `make build`, `pnpm --dir frontend test` (122 tests across 19 files), `make web-typecheck`, production Web builds, and `git diff --check`. This deployment run did not configure remote storage integration or a local PostgreSQL integration DSN; passing the default suite does not establish those environment-dependent checks.

Before migration, the script created custom-format business and identity PostgreSQL dumps, a deployment configuration archive, and records of the previous source and Web targets. Both dumps passed `pg_restore -l`; all five backup files passed their SHA-256 checksum checks. Backup permissions restrict access to the deployment account.

## Migration and service activation

The migration ledger advanced from `000026_cli_recommended_skills.sql` to `000027_conversation_selections.sql`. A separate schema query confirmed the `conversation_selections` table and its six columns, plus nullable `selection_id` columns on `sessions` and `runs`.

The Worker stopped before the API migration. The API became healthy and the migration ledger check passed before the Egress Controller and Worker were recreated. Source and Web symlinks resolve to the new release, and all three service containers report the new release as their Compose working directory.

- API image: `sha256:f4ed80b9a67746f9bc737f7212b5d1e523b33e8e53ebf94ae52b0ac14b5d8138`
- Worker image: `sha256:f9bdeb8191eec856c925fdadc02c7b7dc07b50976247bf3115f9f22a84b6db53`
- Egress Controller image: `sha256:22443575b4ee47830d85bea3ee8d0e925a8ea2cd04e5bb2e60bec5b7d63dcfc3`
- Web entry: `/assets/index-jdwjS386.js`
- Web entry SHA-256: `440f276f05864b715f95e3b18f484f1ae78e771fc3f1fd2501d55d3e7d0f4e4e`

## Public verification

- API, Worker, and Egress Controller health checks report `healthy`.
- Service logs since replacement contain no panic, fatal, or error-level entries according to the deployment script's log check.
- Public Web, Health, Readiness, and OIDC discovery returned HTTP `200`; HTTP redirected to HTTPS with `308`.
- Health returned `{"status":"ok"}` and Readiness returned `{"status":"ready"}`.
- An independent request from the deployment workstation confirmed the public JavaScript entry is byte-for-byte identical to the production build.
- Unauthenticated requests to the new conversation selection, conversation files, and Skill document endpoints returned HTTP `401`.

## Evidence boundary

This verifies the committed release, backup integrity, schema activation, service health, public Web assets, and authentication enforcement on the new routes. It does not establish authenticated production resource-selection flows, actual model execution, or new Runtime image, Linux sandbox, or remote object-storage conformance. No identity configuration was changed for these checks.
