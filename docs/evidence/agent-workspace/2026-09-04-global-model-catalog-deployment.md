# Global Model Catalog deployment verification - 2026-09-04

## Scope

- Host: `47.237.108.63`
- Public origin: `https://47-237-108-63.sslip.io`
- Source release: `/opt/agent-platform/src.release-global-model-catalog-20260904T125300Z`
- Web release: `/opt/agent-platform/web/releases/global-model-catalog-20260904T125300Z`
- Pre-deployment backup: `/opt/agent-platform/backups/pre-global-model-catalog-20260904T125159Z`

The source release was copied from the current local working tree rather than an immutable Git commit. Existing concurrent work in that tree was preserved and included in the deployed snapshot.

## Backup and migration

The backup contains custom-format business and identity PostgreSQL dumps, the deployment configuration archive, and the previous source and Web release targets. Both dumps passed `pg_restore -l`; every backup file passed its recorded SHA-256 checksum before migration.

The API startup applied these append-only migrations successfully:

- `000012_expert_execution_configuration.sql`
- `000013_credits.sql`
- `000014_global_model_catalog.sql`

The migration ledger contains all three entries. `model_provider_connections.credential_owner_user_id` exists, the former `model_provider_connections.owner_user_id` and `provider_models.owner_user_id` columns do not exist, and all Personal Settings default model references resolve to a Provider Model. The migrated catalog contains two Model Provider Connections and eleven Provider Models.

## Deployment

- API image: `sha256:bece84b3b520255eca4db8b7a556bd537d56498039d0a4d9f4022712012131c9`
- Worker image: `sha256:05ffd90cc4dfdd1adf543d9dab9f82e0e6d2b0c6225e9640a088e82dcd7a7f39`
- Web entry: `assets/index-DZS7V-uh.js`

The Worker was stopped before the incompatible ownership-column migration. The new API applied migrations and became healthy before the new Worker started. API and Worker container metadata both resolve to the new source release.

## Verification

- Public Health returned HTTP `200` with `{"status":"ok"}`.
- Public Readiness returned HTTP `200` with `{"status":"ready"}`.
- Web and OIDC discovery returned HTTP `200`; HTTP redirected to HTTPS with `308`.
- API and Worker health checks report `healthy`.
- API and Worker logs since deployment contain no panic, fatal, or error-level entries.
- An authenticated Administrator and a temporary ordinary User received exactly the same two global Model Provider Connections.
- The ordinary User received HTTP `403` for connection create, update, delete, model refresh, and manual Provider Model creation.
- The temporary User was removed and Keycloak Direct Access Grants were restored to `false`.

## Evidence boundary

This verifies backup integrity, schema migration, service and Web deployment, public readiness, global Model Catalog visibility, and deployed Administrator-only mutation enforcement. It does not claim new Runtime image conformance, model-output correctness, Linux sandbox conformance, or Aliyun OSS integration evidence.
