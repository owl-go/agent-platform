# 部署运维参考

日常发布请使用[一键部署](README.md)。这里收录恢复、基础设施升级和可选能力配置，普通发布不需要逐项操作。

## 发布失败时

- 构建或上传失败：服务保持原版本。检查提示中的日志，修复后重新运行 `make deploy`。
- 健康检查失败且 Migration 账本未变：自动恢复原 API、Worker 和 Web。
- 新 Migration 已应用或账本无法读取：保留现场并停止 Worker，不自动运行旧二进制。选择兼容当前 schema 的修复版本，或由运维明确恢复数据库备份。
- 仍有任务运行：本次发布停止；任务完成后重试 `make deploy`。

备份在 `<安装目录>/backups/pre-<发布编号>`。`recovery.json` 记录原源码、Web、镜像和 Migration 账本；两个数据库备份已检查可读，配置和日志只保存在服务器受限目录。保留备份，避免 `docker compose down -v`。

## 基础设施与 Runtime 升级

日常入口检测到 Compose、Caddy、身份主题、Sandbox 或 Runtime 变更会停止发布。这类变更需要对应镜像和环境验收，不能当作普通应用更新。

需要完整升级时，在已集成并测试的 `main_temp` checkout 中使用 `scripts/deploy-platform-infrastructure.sh`，通过 `PLATFORM_DEPLOY_HOST`、`PLATFORM_DEPLOY_ROOT` 指定目标。这个入口会构建并推送 Runtime / CLI Builder、重新验证 CLI Connector、更新受限配置和重建基础设施；退役的检索数据卷保留。它不提供日常入口的自动应用回滚，不适用于空服务器供应。全新安装见[安装说明](installation.md)。

下面是各能力的独立配置参考。

## Configuration

API 和 Worker 只读取 YAML 配置。默认 Compose 配置使用 `config/platform.minio.yaml`；切换阿里云 OSS 时，从 `config/platform.aliyun-oss.example.yaml` 创建部署专用文件，并通过 `PLATFORM_CONFIG_FILE` 指定它。`${NAME}` 占位符在进程启动时从环境展开，未知 YAML 字段、缺失环境变量和非法值会让服务 fail closed。

复制 `.env.example` 到仓库外，替换所有 Secret，并把可变基础设施 Tag 解析为 Repository Digest。宿主 env 与访问交接文件保持 `root:root 0600`；API YAML 使用容器 UID `65532`、Keycloak realm import 使用容器 UID `1000`，二者保持 `0400`。Runtime 镜像单独管理，始终使用 Runtime Conformance 记录的 RepoDigest。

首次启动前创建 Workspace 绑定目录并交给 API/Worker 的非 root 用户。若目录保持 `root:root 0755`，服务虽然可以读取配置，但新增工作流的首次目录或文件写入会失败。

```bash
install -d -m 0700 "${WORKSPACE_ROOT}"
find "${WORKSPACE_ROOT}" -xdev -exec chown -h 65532:65532 {} +
chmod 0700 "${WORKSPACE_ROOT}"
```

The execution overlay gives the Worker only `CHOWN`, `DAC_OVERRIDE`, and `FOWNER` in addition to its Docker socket. Staging switches directories to Runtime UID `65532` before the Worker finishes writing them; `FOWNER` is required to normalize the modes of files created by that Runtime UID before merging a successful Workspace. Runtime containers remain non-root and drop every capability.
`CREDENTIAL_TEMP_ROOT` is mounted at the identical absolute path inside the Worker because the host Docker daemon, not the Worker container, resolves Runtime bind-mount sources.

The same overlay starts a dedicated `egress-controller` in the host Network Namespace with only `NET_ADMIN` added. Worker receives its Unix Socket through a read-only named volume and never receives host-network capability. `AGENT_EGRESS_NETWORK`, `AGENT_EGRESS_SUBNET`, and `AGENT_DNS_SERVERS` must match the YAML `sandbox` values and the host `configure-public-egress.sh` configuration exactly; the controller rejects every CLI lease when they drift.

`worker.runtime_idle_timeout` defaults to the deployed value `30m`. Session and Workflow Runtime containers are stopped after each execution, retain their immutable Docker definition for this idle window, and are then removed by the Worker reaper. Per-execution credential directories are removed immediately after stop and are never retained for the idle window.

```bash
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml config
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

部署文件可以明确覆盖 YAML 路径：

```bash
PLATFORM_CONFIG_FILE=/srv/agent-workspace/config/platform.yaml \
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

Verify the base API from inside its private container network:

```bash
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml exec -T api \
  wget -qO- http://127.0.0.1:8080/readyz
```

## Identity login theme

The `agent-workspace` Keycloak login theme inherits the pinned image's native templates and Chinese/English messages. It replaces the dark background with workspace tokens and a compact white card. Build before provisioning the identity container; source theme files alone do not include the generated token stylesheet:

```bash
python3 scripts/build-identity-theme.py /tmp/agent-workspace-theme-RELEASE
rsync -a /tmp/agent-workspace-theme-RELEASE/ deploy@example.com:/srv/agent-workspace/identity-themes/releases/RELEASE/
```

Set `KEYCLOAK_THEME_ROOT` in the private host env to the built release directory (or `/srv/agent-workspace/identity-themes/current`, a symlink to it). Files must be readable by container UID 1000, with directories 0755 and files 0644. The Compose bind is read-only. Changing a symlink requires recreating only the identity container so Docker resolves the new source; restart alone retains the previous bind. Keep the previous release for rollback. Public CSS/message files contain no credentials; verify `manifest.json` against the uploaded bytes before activation.

New realms import `loginTheme=agent-workspace`, `internationalizationEnabled=true`, `supportedLocales=[zh-Hans,en]`, and `defaultLocale=zh-Hans`. An existing realm does not reimport those settings. On the host, export the protected platform env and run the following helper from the integrated release; it authenticates using an HTTPS POST body and changes only those four fields:

```bash
set -a
. /srv/agent-workspace/config/platform.env
set +a
python3 scripts/configure-identity-theme.py backup /srv/agent-workspace/backups/RELEASE-appearance.json
python3 scripts/configure-identity-theme.py apply
# Restore appearance when rolling back the mount:
python3 scripts/configure-identity-theme.py restore /srv/agent-workspace/backups/RELEASE-appearance.json
```

Do not print the environment or shell trace these commands. Back up the private host env before updating the mount. For a currently deployed source bundle predating the theme bind, a small Compose override can add the read-only `/opt/keycloak/themes` bind; use the existing project/config files and `up -d --no-deps identity`, then verify OIDC discovery before applying appearance. Future source releases include the bind in `compose.https.yaml`. The helper does not import accounts, modify clients, change authentication policy, or revoke sessions. After activation, verify fresh product entry, Chinese defaults, the language selector, native error/password/reset flows, mobile layout, and public Health/Readiness. Rollback restores the previous bind/env, recreates identity, and restores the backed-up appearance fields.

## Same-origin HTTPS and OIDC

Set `PUBLIC_HOST` to a DNS name whose A/AAAA record reaches the Worker, allow inbound TCP 80/443 and UDP 443, and set all OIDC URLs to that exact HTTPS origin. `scripts/deploy-web.sh` requires the four `VITE_OIDC_*` values in the release workstation environment and consumes them during the local production build. API and Worker consume `platform.https.yaml` at startup. These values are configuration, not source-code constants.

The Web release root defaults to `/srv/agent-workspace/web` and has this layout:

```text
/srv/agent-workspace/web/
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

WEB_DEPLOY_HOST=deploy@example.com \
WEB_RELEASE_ROOT=/srv/agent-workspace/web \
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
chmod 600 /srv/agent-workspace/config/platform.env
chown 65532:65532 /srv/agent-workspace/config/platform.https.yaml
chmod 400 /srv/agent-workspace/config/platform.https.yaml
chown 1000:0 /srv/agent-workspace/config/keycloak-realm.json
chmod 400 /srv/agent-workspace/config/keycloak-realm.json

PLATFORM_CONFIG_FILE=/srv/agent-workspace/config/platform.https.yaml \
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml \
  -f deploy/platform/compose.https.yaml config --quiet

PLATFORM_CONFIG_FILE=/srv/agent-workspace/config/platform.https.yaml \
docker compose --env-file /srv/agent-workspace/config/platform.env \
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
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml -f deploy/platform/compose.https.yaml ps
```

Use `docker compose ... logs --since 15m api worker caddy identity` for diagnostics. Logs must be scanned for planted Secret values before being retained as evidence.

## Rollback

Keep the previous source bundle or immutable service image references until verification completes. To roll back API or Worker code while preserving PostgreSQL, MinIO, Keycloak, and Caddy volumes:

```bash
cd /srv/agent-workspace/src.previous
PLATFORM_CONFIG_FILE=/srv/agent-workspace/config/platform.https.yaml \
docker compose --env-file /srv/agent-workspace/config/platform.env \
  -f deploy/platform/compose.yaml -f deploy/platform/compose.https.yaml up -d --build
```

Roll back only the Web UI by atomically selecting an existing static release; this does not restart Caddy or any application service:

```bash
WEB_DEPLOY_HOST=deploy@example.com \
WEB_RELEASE_ROOT=/srv/agent-workspace/web \
scripts/deploy-web.sh activate <previous-revision>
```

Do not run `down -v`: the named volumes contain persistent product and identity data. Database migrations are append-only; a rollback must use a binary compatible with the migrated schema or restore a tested database backup.

## Readiness Boundary

The UI and `/readyz` prove only API, database, identity, and frontend availability. A Runtime is selectable only when the deployed RepoDigest has passed its real model, MCP, cancellation, Secret-redaction, Workspace, and gVisor checks. Never insert synthetic Run success or event records to make verification pass.

## Optional Scan Registration

WeChat Official Account and Feishu scan sign-in/register are configured by a product Administrator under User Management → Registration. They default to closed and keep Keycloak as the product credential authority. Deploy the optional broker secrets and run the one-time admin-only identity-profile setup before enabling methods; [scan registration](../../docs/technical/scan-registration.md) documents exact provider permissions, safe-mode callbacks, configuration, and outstanding real-account acceptance. Never reuse Workflow Message Channel or Connector credentials for product registration.
