# Agent Workspace Single-Worker Deployment

This deployment runs API, Worker, PostgreSQL, MinIO, Caddy, and Keycloak on one Linux Worker. The Vue application is built on the release workstation and uploaded as a versioned `dist` directory; it does not run in a separate container. Caddy is the only public entrypoint, while PostgreSQL and MinIO remain on internal Docker networks.

## One-command release

For an already provisioned Worker, run the complete guarded deployment from the release workstation:

```bash
make deploy
```

`scripts/deploy-platform.sh` runs the backend and frontend gates, reads only the public Web/OIDC values from the remote env file, creates and verifies business-database, identity-database, and configuration backups, uploads an immutable source release, prebuilds API, Worker, and Egress Controller images, stops the old Worker, starts the new API to apply append-only migrations, verifies the latest migration ledger entry, starts the Egress Controller and new Worker, atomically deploys the Web release, and checks public Health, Readiness, OIDC, HTTPS redirect, container health, release identity, and error logs.

The defaults match the production layout:

```text
PLATFORM_DEPLOY_HOST=agent-platform
PLATFORM_DEPLOY_ROOT=/opt/agent-platform
```

Override `PLATFORM_RELEASE_ID` when a caller needs a predetermined immutable release name. `SKIP_DEPLOY_GATES=1` exists only for an explicitly approved emergency release; normal deployments must keep the gates enabled. The script deliberately does not automatically start an old binary after migrations. If the new API fails after the schema changes, it leaves recovery evidence and the pre-deployment backup in place and reports that the Worker may remain stopped. Restore a schema-compatible release or the verified database backup before resuming execution.

The one-shot `minio-init` service idempotently creates the configured private Bucket after MinIO becomes healthy. API and Worker wait for that initialization to succeed, so a missing Bucket fails during startup instead of after a completed Runtime execution.

The base stack contains the private control and storage services. `compose.https.yaml` adds automatic TLS, static Web hosting, same-origin API/SSE routing, and a PostgreSQL-backed Keycloak OIDC issuer. Runtime availability is reported separately and remains disabled until its image has passed the target Linux + gVisor checks.

## Configuration

API 和 Worker 只读取 YAML 配置。默认 Compose 配置使用 `config/platform.minio.yaml`；切换阿里云 OSS 时，从 `config/platform.aliyun-oss.example.yaml` 创建部署专用文件，并通过 `PLATFORM_CONFIG_FILE` 指定它。`${NAME}` 占位符在进程启动时从环境展开，未知 YAML 字段、缺失环境变量和非法值会让服务 fail closed。

复制 `.env.example` 到仓库外，替换所有 Secret，并把可变基础设施 Tag 解析为 Repository Digest。宿主 env 与访问交接文件保持 `root:root 0600`；API YAML 使用容器 UID `65532`、Keycloak realm import 使用容器 UID `1000`，二者保持 `0400`。Runtime 镜像单独管理，始终使用 Runtime Conformance 记录的 RepoDigest。

首次启动前创建 Workspace 绑定目录并交给 API/Worker 的非 root 用户。若目录保持 `root:root 0755`，服务虽然可以读取配置，但新增工作流的首次目录或文件写入会失败。

```bash
install -d -o 65532 -g 65532 -m 0700 "${WORKSPACE_ROOT}"
```

The execution overlay gives the Worker only `CHOWN`, `DAC_OVERRIDE`, and `FOWNER` in addition to its Docker socket. Staging switches directories to Runtime UID `65532` before the Worker finishes writing them; `FOWNER` is required to normalize the modes of files created by that Runtime UID before merging a successful Workspace. Runtime containers remain non-root and drop every capability.
`CREDENTIAL_TEMP_ROOT` is mounted at the identical absolute path inside the Worker because the host Docker daemon, not the Worker container, resolves Runtime bind-mount sources.

The same overlay starts a dedicated `egress-controller` in the host Network Namespace with only `NET_ADMIN` added. Worker receives its Unix Socket through a read-only named volume and never receives host-network capability. `AGENT_EGRESS_NETWORK`, `AGENT_EGRESS_SUBNET`, and `AGENT_DNS_SERVERS` must match the YAML `sandbox` values and the host `configure-public-egress.sh` configuration exactly; the controller rejects every CLI lease when they drift.

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

Verify the base API from inside its private container network:

```bash
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml exec -T api \
  wget -qO- http://127.0.0.1:8080/readyz
```

## Same-origin HTTPS and OIDC

Set `PUBLIC_HOST` to a DNS name whose A/AAAA record reaches the Worker, allow inbound TCP 80/443 and UDP 443, and set all OIDC URLs to that exact HTTPS origin. `scripts/deploy-web.sh` requires the four `VITE_OIDC_*` values in the release workstation environment and consumes them during the local production build. API and Worker consume `platform.https.yaml` at startup. These values are configuration, not source-code constants.

The Web release root defaults to `/opt/agent-platform/web` and has this layout:

```text
/opt/agent-platform/web/
├── current -> releases/<revision>
└── releases/
    ├── <previous-revision>/
    └── <revision>/
```

Build and upload a release from the workstation that has the repository checkout and production OIDC values. `WEB_DEPLOY_HOST` is an SSH destination understood by both `ssh` and `rsync`. The script builds locally, rejects incomplete or symlinked output, uploads into a new immutable release directory, normalizes public-file permissions, and atomically changes `current` only after verification succeeds.

```bash
export PUBLIC_HOST="agent-platform.example.test"
export VITE_OIDC_AUTHORITY="https://${PUBLIC_HOST}/identity/realms/agent-platform"
export VITE_OIDC_CLIENT_ID="agent-platform-web"
export VITE_OIDC_REDIRECT_URI="https://${PUBLIC_HOST}/auth/callback"
export VITE_OIDC_POST_LOGOUT_REDIRECT_URI="https://${PUBLIC_HOST}"

WEB_DEPLOY_HOST=agent-platform \
WEB_RELEASE_ROOT=/opt/agent-platform/web \
make web-deploy
```

`MODEL_RELAY_UPSTREAM` optionally points at a host-local HTTP model gateway. Caddy exposes it only through the authenticated `/model-relay/` TLS route, so Model Provider API Keys are not sent over public plaintext HTTP.

The Keycloak realm import is deployment-owned because it contains bootstrap users. Store it outside the repository at `OIDC_REALM_FILE`. The realm must provide:

- realm `agent-platform` and public client `agent-platform-web` with Authorization Code + PKCE;
- exact Redirect URI `${VITE_OIDC_REDIRECT_URI}`, post-logout origin `${VITE_OIDC_POST_LOGOUT_REDIRECT_URI}/*`, and trusted Web Origin;
- the `agent-platform-api` audience and stable Keycloak User subject;
- a 72-hour SSO idle and maximum lifespan (`259200` seconds), with short-lived Access Tokens renewed by the Web client;
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

`WEB_RELEASE_ROOT` must exist and contain a valid `current` release before Caddy starts. Subsequent Web releases only run `make web-deploy`; they do not rebuild or restart any Compose service.

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

Use `docker compose ... logs --since 15m api worker caddy identity` for diagnostics. Logs must be scanned for planted Secret values before being retained as evidence.

## Rollback

Keep the previous source bundle or immutable service image references until verification completes. To roll back API or Worker code while preserving PostgreSQL, MinIO, Keycloak, and Caddy volumes:

```bash
cd /opt/agent-platform/src.previous
PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.https.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml -f deploy/platform/compose.https.yaml up -d --build
```

Roll back only the Web UI by atomically selecting an existing static release; this does not restart Caddy or any application service:

```bash
WEB_DEPLOY_HOST=agent-platform \
WEB_RELEASE_ROOT=/opt/agent-platform/web \
scripts/deploy-web.sh activate <previous-revision>
```

Do not run `down -v`: the named volumes contain persistent product and identity data. Database migrations are append-only; a rollback must use a binary compatible with the migrated schema or restore a tested database backup.

## Readiness Boundary

The UI and `/readyz` prove only API, database, identity, and frontend availability. A Runtime is selectable only when the deployed RepoDigest has passed its real model, MCP, cancellation, Secret-redaction, Workspace, and gVisor checks. Never insert synthetic Run success or event records to make verification pass.
