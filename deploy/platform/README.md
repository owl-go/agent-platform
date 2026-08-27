# Agent Workspace Single-Worker Deployment

This Compose stack runs the current API, Worker, Web, PostgreSQL, and MinIO services on one Linux Worker. Only the Web port is published; PostgreSQL and MinIO remain on an internal Docker network.

The one-shot `minio-init` service idempotently creates the configured private Bucket after MinIO becomes healthy. API and Worker wait for that initialization to succeed, so a missing Bucket fails during startup instead of after a completed Runtime execution.

The base stack is suitable for loopback smoke tests. `compose.https.yaml` adds automatic TLS, same-origin Web/API/SSE routing, and a PostgreSQL-backed Keycloak OIDC issuer. Runtime availability is reported separately and remains disabled until its image has passed the target Linux + gVisor checks.

## Configuration

API 和 Worker 只读取 YAML 配置。默认 Compose 配置使用 `config/platform.minio.yaml`；切换阿里云 OSS 时，从 `config/platform.aliyun-oss.example.yaml` 创建部署专用文件，并通过 `PLATFORM_CONFIG_FILE` 指定它。`${NAME}` 占位符在进程启动时从环境展开，未知 YAML 字段、缺失环境变量和非法值会让服务 fail closed。

复制 `.env.example` 到仓库外，替换所有 Secret，并把可变基础设施 Tag 解析为 Repository Digest。宿主 env 与访问交接文件保持 `root:root 0600`；API YAML 使用容器 UID `65532`、Keycloak realm import 使用容器 UID `1000`，二者保持 `0400`。Runtime 镜像单独管理，始终使用 Runtime Conformance 记录的 RepoDigest。

首次启动前创建 Workspace 绑定目录并交给 API/Worker 的非 root 用户。若目录保持 `root:root 0755`，服务虽然可以读取配置，但新增工作流的首次目录或文件写入会失败。

```bash
install -d -o 65532 -g 65532 -m 0700 "${WORKSPACE_ROOT}"
```

The execution overlay gives the Worker only `CHOWN` and `DAC_OVERRIDE` in addition to its Docker socket. These are required because staging switches directories to Runtime UID `65532` before the Worker finishes writing them. Runtime containers remain non-root and drop every capability.
`CREDENTIAL_TEMP_ROOT` is mounted at the identical absolute path inside the Worker because the host Docker daemon, not the Worker container, resolves Runtime bind-mount sources.

`worker.runtime_idle_timeout` defaults to the deployed value `30m`. Session and Workflow Runtime containers are stopped after each execution, retain their immutable Docker definition for this idle window, and are then removed by the Worker reaper. Per-execution credential directories are removed immediately after stop and are never retained for the idle window.

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

`MODEL_RELAY_UPSTREAM` optionally points at a host-local HTTP model gateway. Caddy exposes it only through the authenticated `/model-relay/` TLS route, so Model Profile Secrets are not sent over public plaintext HTTP.

The Keycloak realm import is deployment-owned because it contains bootstrap users. Store it outside the repository at `OIDC_REALM_FILE`. The realm must provide:

- realm `agent-platform` and public client `agent-platform-web` with Authorization Code + PKCE;
- exact Redirect URI `${VITE_OIDC_REDIRECT_URI}`, post-logout origin `${VITE_OIDC_POST_LOGOUT_REDIRECT_URI}/*`, and trusted Web Origin;
- the `agent-platform-api` audience and stable Keycloak User subject;
- no committed user password, client secret, Token, private key, or Runtime Credential.

Start the HTTPS stack with the deployment YAML outside the repository:

```bash
chmod 600 /opt/agent-platform/config/platform.env
chown 65532:65532 /opt/agent-platform/config/platform.https.yaml
chmod 400 /opt/agent-platform/config/platform.https.yaml
chown 1000:0 /opt/agent-platform/config/keycloak-realm.json
chmod 400 /opt/agent-platform/config/keycloak-realm.json

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

The UI and `/readyz` prove only API, database, identity, and frontend availability. A Runtime is selectable only when the deployed RepoDigest has passed its real model, MCP, cancellation, Secret-redaction, Workspace, and gVisor checks. Never insert synthetic Run success or event records to make verification pass.
