# Phase 4 HTTPS Control-plane Deployment — 2026-08-24

## Deployment

- Revision: `1c261de` (includes `19733c5`, `7e42981`, and `1c261de`).
- Worker: `47.237.108.63`, Ubuntu Linux, Docker 29.7.2, Docker Compose 5.4.0.
- Public origin: `https://47-237-108-63.sslip.io`.
- Source: `/opt/agent-platform/src.release-1c261de`, selected by `/opt/agent-platform/src`.
- Configuration: `/opt/agent-platform/config`; secrets and bootstrap access remain outside the repository.
- Persistent services: PostgreSQL and MinIO on internal Docker networks; PostgreSQL-backed Keycloak on its own internal network.

Caddy obtained a publicly trusted Let's Encrypt certificate for `47-237-108-63.sslip.io`. The certificate was valid from 2026-08-24 through 2026-11-22 at verification time. TCP 80 redirects to HTTPS; TCP/UDP 443 serve HTTP/2 and HTTP/3-capable TLS. PostgreSQL, MinIO, the identity database, and Keycloak have no published host port.

## Verification Result

- `/`, `/api/healthz`, `/api/readyz`, and OIDC discovery return successfully over the same HTTPS origin.
- HTTPS responses include HSTS, `nosniff`, frame denial, no-referrer, and restrictive Permissions Policy headers.
- OIDC discovery reports the exact configured issuer and S256 PKCE support. Browser Authorization Code + PKCE login, provider logout, and protected-state removal passed.
- Unauthenticated ordinary API and Run Event SSE requests return 401 through the same proxy. A hostile Origin preflight receives no permissive CORS header.
- API, Worker, Web, Caddy, Keycloak, both PostgreSQL services, and MinIO are running; API, Worker, Web, PostgreSQL, identity PostgreSQL, and MinIO report healthy.
- A remote log scan found none of the final four generated user passwords or invalidated one-time browser credentials.

## Role Evidence

- Platform Administrator logged in through deployed OIDC and registered the server's existing pinned Codex RepoDigest through the real API. It remains `Experimental`; no Conformance evidence was invented.
- Agent Builder logged in and created `deployment-control-agent` through the real API. No Draft was falsely validated or released without dependencies.
- Agent User logged in and received the real empty Coding Task state plus an unmet launch prerequisite. Coding Task creation remained disabled, so no synthetic Run was created.
- Run Operator logged in and received real empty Run and Audit search results.

The deployment database contained one experimental Runtime, one Agent, zero Runs, and two Audit Events after verification. These counts match the two deliberate control-plane writes.

## Data Migration Decision

The previous `agent_platform` database contains historical Phase 0/load-test organizations and 91 legacy experimental Runtime rows without auditable Organization ownership. Migration `000012` correctly failed closed rather than guessing an owner. The deployment therefore created a separate `agent_platform_control` database and ran all migrations from an empty schema. The old database remains intact.

Before the attempt, a custom-format backup was written to `/opt/agent-platform/build/backups/pre-https-20260824.dump` with mode `0600`. The previous source tree remains at `/opt/agent-platform/src.previous-20260824`.

## Reproduction

```bash
ssh agent-platform
cd /opt/agent-platform/src
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml \
  -f deploy/platform/compose.https.yaml ps

curl --fail --proto '=https' --tlsv1.2 \
  https://47-237-108-63.sslip.io/api/healthz
curl --fail --proto '=https' --tlsv1.2 \
  https://47-237-108-63.sslip.io/api/readyz
curl --fail --proto '=https' --tlsv1.2 \
  https://47-237-108-63.sslip.io/identity/realms/agent-platform/.well-known/openid-configuration
```

Deployment user passwords are random and stored only in `/opt/agent-platform/config/deployment-access.txt` with mode `0600`. Retrieve them through the existing SSH boundary when a human needs to verify a role; do not copy them into tickets, logs, or repository files.

## Rollback

Point `/opt/agent-platform/src` back to `/opt/agent-platform/src.previous-20260824`, select the original deployment config/database, and run the documented Compose command. Do not remove named volumes. The old application database and the pre-deployment dump provide separate recovery paths.

## Readiness Boundary

This is a deployed control-plane closure, not Coding Agent Production Conformance. Real model access, configured Git SSH credentials and Push, four Runtime credentials, Aliyun OSS verification, and a complete Linux + gVisor execution evidence set were not supplied for this deployment. The UI reports those prerequisites as unavailable, no Run was fabricated, and Phase 0 remains `NO-GO`.
