# Single-Worker Control-plane Deployment

This Compose stack runs the current API, Worker, Web, PostgreSQL, and MinIO services on one Linux Worker. Only the Web port is published; PostgreSQL and MinIO remain on an internal Docker network.

The base stack is suitable for loopback smoke tests. `compose.https.yaml` adds the production-facing edge: automatic TLS, same-origin Web/API/SSE routing, and a PostgreSQL-backed Keycloak OIDC issuer. This proves the control plane only; it is not Production Runtime Conformance evidence.

## Configuration

API 和 Worker 只读取 YAML 配置。默认 Compose 配置使用 `config/platform.minio.yaml`；切换阿里云 OSS 时，从 `config/platform.aliyun-oss.example.yaml` 创建部署专用文件，并通过 `PLATFORM_CONFIG_FILE` 指定它。`${NAME}` 占位符在进程启动时从环境展开，未知 YAML 字段、缺失环境变量和非法值会让服务 fail closed。

复制 `.env.example` 到仓库外，替换所有 Secret，并把可变基础设施 Tag 解析为 Repository Digest。配置文件权限设为 `0600`。Runtime 镜像单独管理，始终使用 Runtime Conformance 记录的 RepoDigest。

```bash
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml config
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

部署文件可以明确覆盖 YAML 路径：

```bash
PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

Verify the public entrypoint and proxied API health endpoint:

```bash
curl --fail http://127.0.0.1/
curl --fail http://127.0.0.1/api/healthz
curl --fail http://127.0.0.1/api/readyz
```

## Same-origin HTTPS and OIDC

Set `PUBLIC_HOST` to a DNS name whose A/AAAA record reaches the Worker, allow inbound TCP 80/443 and UDP 443, and set all OIDC URLs to that exact HTTPS origin. `Dockerfile.web` consumes the four `VITE_OIDC_*` values at build time; API and Worker consume `platform.https.yaml` at startup. They are configuration, not source-code constants.

The Keycloak realm import is deployment-owned because it contains bootstrap users. Store it outside the repository at `OIDC_REALM_FILE`. The realm must provide:

- realm `agent-platform` and public client `agent-platform-web` with Authorization Code + PKCE;
- exact Redirect URI `${VITE_OIDC_REDIRECT_URI}`, post-logout origin `${VITE_OIDC_POST_LOGOUT_REDIRECT_URI}/*`, and trusted Web Origin;
- `organization` claim (or the configured `OIDC_ORGANIZATION_CLAIM`) and the `agent-platform-api` audience;
- no committed user password, client secret, Token, private key, or Runtime Credential.

Start the HTTPS stack with the deployment YAML outside the repository:

```bash
chmod 600 /opt/agent-platform/config/platform.env \
  /opt/agent-platform/config/platform.https.yaml \
  /opt/agent-platform/config/keycloak-realm.json

PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.https.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml \
  -f deploy/platform/compose.https.yaml config --quiet

PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.https.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml \
  -f deploy/platform/compose.https.yaml up -d --build
```

Verify TLS, security headers, OIDC discovery, Health, Readiness, an authenticated API, and an SSE response without exposing a Token in shell history:

```bash
curl --fail --proto '=https' --tlsv1.2 "https://${PUBLIC_HOST}/"
curl --fail --proto '=https' --tlsv1.2 "https://${PUBLIC_HOST}/api/healthz"
curl --fail --proto '=https' --tlsv1.2 "https://${PUBLIC_HOST}/api/readyz"
curl --fail --proto '=https' --tlsv1.2 \
  "https://${PUBLIC_HOST}/identity/realms/agent-platform/.well-known/openid-configuration"
curl --fail --head "http://${PUBLIC_HOST}/" # must redirect to HTTPS
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml -f deploy/platform/compose.https.yaml ps
```

Use `docker compose ... logs --since 15m api worker web caddy identity` for diagnostics. Logs must be scanned for planted Secret values before being retained as evidence.

## Rollback

Keep the previous source bundle or immutable service image references until verification completes. To roll back application code while preserving PostgreSQL, MinIO, Keycloak, and Caddy volumes:

```bash
cd /opt/agent-platform/src.previous
PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.https.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml -f deploy/platform/compose.https.yaml up -d --build
```

Do not run `down -v`: the named volumes contain persistent product and identity data. Database migrations are append-only; a rollback must use a binary compatible with the migrated schema or restore a tested database backup.

## Readiness Boundary

The UI and `/readyz` prove control-plane availability. Missing Production Runtime Conformance, real model access, Git SSH Push, or provider-specific Object Storage evidence must remain visible as unmet prerequisites. Never insert synthetic Run success or Git delivery records to make deployment verification pass, and do not change the Phase 0 `NO-GO` decision without the independent evidence set.
