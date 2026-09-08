# Generic Connector authorization deployment verification - 2026-09-08

## Release

- Public origin: `https://47-237-108-63.sslip.io`
- Source commit: `39a8a4a31d591d950fda114a5fd3e6367cd7cd77` (`codex/fix-feishu-cli-install`)
- Active release: `generic-connector-auth-39a8a4a-20260908`
- Source: `/opt/agent-platform/src.release-generic-connector-auth-39a8a4a-20260908`
- Web: `/opt/agent-platform/web/releases/generic-connector-auth-39a8a4a-20260908`
- Backup: `/opt/agent-platform/backups/pre-generic-connector-auth-39a8a4a-20260908`
- Previous source: `/opt/agent-platform/src.release-generic-connector-auth-1d90a0b-20260908`

The guarded `scripts/deploy-platform.sh` deployment completed with all deployment gates enabled. The first cutover attempt at commit `324eb8a` stopped when PostgreSQL rejected `authorization` as an unquoted migration alias. The transaction did not commit. The corrected migration was first executed against the production schema inside `BEGIN` and `ROLLBACK`, then committed as `1d90a0b` and deployed successfully. A real-browser check exposed omitted repeated protobuf fields in the new Manifest response; commit `39a8a4a` normalized those fields at the Web API boundary and was deployed as the final release above.

## Validation and backup

The final release passed the complete backend test suite, `go build ./...`, all 171 Web tests across 24 files, Vue/TypeScript type checking, the production Web build, and `git diff --check`. The deployment script built immutable API, Worker, Egress Controller, Web, and CLI Builder artifacts.

Before the final cutover, the script created custom-format business and identity PostgreSQL dumps, a configuration archive, and records of the previous source and Web targets. Both dumps passed `pg_restore -l`; all five backup files passed their recorded SHA-256 checks.

## Migration and service activation

The production migration ledger contains:

- `000031_connector_manifest.sql`
- `000032_connector_action_requirements.sql`
- `000033_connector_audit_records.sql`

The new `connector_action_requirements` and `connector_audit_records` tables and the one-active-authorization-per-Connector index exist. The Worker stopped before migration, the API became healthy after migration, and the Egress Controller and Worker were then recreated.

- API image: `sha256:5c0c4bf6e983f064a5ba606b755a7ec77c182dede4ac4d311cd45d2935307930`
- Worker image: `sha256:8da40ff6ae43ef8675739d55eab4fafad48b6d90335708291022742a88a309e7`
- Egress Controller image: `sha256:7453e7687b2bee28c511e3330a6731a5970e068ca51e1d37bd36fd61744c10d8`

All three containers are healthy, report the final release as their Compose working directory, and contain no panic, fatal, or error-level log entries since replacement.

## Connector build and review

Migration deliberately moved the legacy available Feishu CLI Definition to `draft` because it had no reviewed Resolved Connector Manifest. The existing Definition then completed the production lifecycle `draft -> building -> testing -> review -> available` through the authenticated Administrator API; no database state was bypassed.

- Definition: `飞书 CLI`, `@larksuite/cli@1.0.93`
- State/version: `available`, version `45`
- Manifest version: `1`
- Bundle SHA-256: `a1b99b398fe4134a5d3f38828941548304e184c9778d52b33cbdb6e5b1a6276a`
- Conformance: five passed rows for the exact current Bundle SHA-256 and configured Runtime RepoDigests
- Capabilities: `identity`, `chat.search`, and `messages.send`, including structured inputs, identity, risk, exact Permissions, Egress, timeout, and idempotency semantics

The existing `wuyuewei` enablement remains enabled and its Feishu User authorization remains active and Connector-private. Its legacy grant does not contain the new exact `im:chat:readonly` or `im:message:send_as_user` Permissions, so a protected operation will enter the new lazy Permission expansion flow instead of silently reusing broader legacy strings.

## Public and browser verification

- Public Web, Health, Readiness, and OIDC discovery returned HTTP `200`; HTTP redirected to HTTPS with `308`.
- An authenticated real Chrome session opened the production Connector catalog. Merely browsing and opening the available Connector did not start external account authorization.
- The detail drawer displayed Manifest v1, the exact Bundle SHA-256, all three localized capability summaries, command prefixes, Permissions, Egress hosts, timeouts, idempotency policies, and the reviewed Usage Guide.
- Before commit `39a8a4a`, opening the detail drawer reproduced `Cannot read properties of undefined (reading 'join')` for the valid empty `identity.scopes` collection. After deployment, the same interaction rendered normally and produced no new browser console errors.
- Keycloak Direct Access Grants were restored to `false`. The failed temporary acceptance fixture left zero `acceptance-*` users in Keycloak and the business database.

## Evidence boundary

The generic Authorization Adapter, Action Requirement, one-time platform URL, auto-poll/resume, approval/audit, and compact execution-summary paths passed their committed unit and integration suites and are running in production. The exact Feishu bundle completed production build and configured Runtime Conformance, and the existing User state was preserved.

The real provider consent redirect and same-execution resume still require an interactive login by the owning User. They were not claimed as complete in this verification, and no Feishu message was sent. The legacy full deployment acceptance script was also executed; it stopped at `settings-and-expert` because it still attempts an ordinary-User Model Provider mutation that the current global catalog correctly rejects with HTTP `403`. Its cleanup completed. This script result is recorded as failed and is not acceptance evidence for this release.
