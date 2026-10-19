# Docker Compose 部署

Docker Hub 按职责发布两个多架构镜像：

- `haozhi152/agent-platform`：产品控制服务，包含 Web 入口、API、Worker、Keycloak、PostgreSQL 17、开源 MinIO 与 Nginx，不包含模型 Runtime CLI。
- `haozhi152/agent-platform-runtime`：统一执行镜像，只包含 Claude Code、Codex、Hermes、OpenClaw、PI Agent 及其公共执行依赖。

Compose 先用一次性 `runtime-image` 服务拉取并缓存 Runtime 镜像，成功后再启动 `app`。任务执行时 Worker 从宿主 Docker 创建短生命周期 Runtime 容器；Runtime 不挂载 Docker Socket 或整卷 `/data`。默认是单用户可信本机的显式 `runc` 模式，不能替代生产隔离验收。

## 1. 前提

安装 Docker Engine / Docker Desktop 与 Compose v2。建议给 Docker 分配至少 4 GB 内存，并为两个镜像和持久数据预留足够磁盘空间。

## 2. 使用 Docker Compose

在源码目录运行：

```bash
docker compose -f deploy/all-in-one/compose.yaml pull
docker compose -f deploy/all-in-one/compose.yaml up -d
docker compose -f deploy/all-in-one/compose.yaml ps
docker compose -f deploy/all-in-one/compose.yaml logs -f app
```

等待 `app` 变为 `healthy` 或日志出现 `Agent Workspace ready`，然后访问 `http://localhost:8080`。首次初始化默认管理员为 **`admin / admin123`**。重启不会重置账号、密码、系统密钥或数据。

在同目录创建 `.env` 可覆盖常用设置：

```dotenv
PORT=8080
PUBLIC_URL=http://localhost:8080
ADMIN_USERNAME=admin
ADMIN_PASSWORD=replace-with-a-strong-password
IMAGE_TAG=latest
RUNTIME_IMAGE=haozhi152/agent-platform-runtime:latest
DATA_VOLUME=agent-platform-data
```

`ADMIN_USERNAME` 和 `ADMIN_PASSWORD` 只在空数据卷首次初始化时生效。已有数据不能通过环境变量改名或重置密码。公开部署必须使用精确 HTTPS Origin、强管理员密码和生产 Sandbox 配置；非环回 HTTP 或公开环境的默认弱密码会拒绝启动。

Compose 默认使用 `agent-platform-data` 卷保存 PostgreSQL、私有对象、Workspace、状态密钥和配置，与旧版单容器部署保持兼容；需要隔离多套安装时可用 `DATA_VOLUME` 指定其他卷名。不要运行 `docker compose down -v`，也不要只备份数据库而遗漏对象和 `state.json`。

Docker Socket 只挂载给受信任的控制服务，用于创建受限 Runtime 容器，权限等同于控制 Docker 主机。不要把 Socket 继续挂入 Runtime，不要改成 `--privileged`。

## 3. 镜像、架构与版本

两个仓库均发布相同格式的 Tag：

- `latest`：ARM64 + AMD64，多架构自动选择；
- `arm64` / `amd64`：对应架构别名；
- `YYYY-MM-DD-<commit>`：不可变发布对应的多架构 Tag；
- `YYYY-MM-DD-<commit>-arm64` / `-amd64`：单架构发布 Tag。

生产更新应固定两个仓库各自的 RepoDigest，而不是把产品镜像 Digest 当作 Runtime Digest，也不要只记录可移动的 `latest`。

## 4. 从源码构建

```bash
git clone https://github.com/owl-go/agent-platform.git
cd agent-platform
git checkout main_temp
make image-build
IMAGE_TAG=local \
RUNTIME_IMAGE=haozhi152/agent-platform-runtime:local \
  docker compose -f deploy/all-in-one/compose.yaml up -d
```

`make image-build` 分别生成：

```text
haozhi152/agent-platform:local
haozhi152/agent-platform-runtime:local
```

产品镜像不再继承 Runtime 镜像。修改任一侧只会重建对应职责的镜像层。

## 5. 端口、域名与更新

修改本机端口时同时设置 `PORT=18080` 和 `PUBLIC_URL=http://localhost:18080`。公开使用时由外部 HTTPS 反向代理终止 TLS，并将 `PUBLIC_URL` 设置为精确 HTTPS Origin。代理须保留 API/SSE 长连接。

更新前先停止任务并做一致性备份，然后执行：

```bash
docker compose -f deploy/all-in-one/compose.yaml pull
docker compose -f deploy/all-in-one/compose.yaml up -d
```

Compose 会分别拉取产品镜像与 Runtime 镜像；没有新 Digest 时复用本地缓存。数据卷保持不变。镜像回退不是 Schema 或数据回退。

## 6. 生产 `runsc`

本地 `local/runc` 下，Worker 会对分发的固定 CLI Connector 包进行独立、无网络和无账号凭据的启动检查；通过当前 Runtime Image ID 检查后，默认目录会自动恢复可安装状态，通常在启动后数十秒内完成。管理员已经明确停用的条目不会自动恢复。飞书、钉钉与企业微信安装后的账号授权仍需 User 完成，真实业务读写必须另验；企业微信 1.4.3 包含 ARM64/x64 二进制。切换 Runtime 镜像后重新验证，不继承旧 Image ID 的通过记录。隔离 npm Builder 和 Workflow Message Channel 不因此启用。

本地 Connector 使用可信个人 bridge，没有生产域名级网络隔离保证，不应在共享主机使用。生产验证表与本地验证表分开，详见 [本地 Connector 边界](../../docs/adr/0049-local-cli-connector-verification.md)。

本 Compose 默认是本地 `runc` 模式。生产仍要求 Linux + `runsc`、固定 Runtime RepoDigest、受管 Egress Network、固定 Resolver、宿主 Egress Controller 和对应 Digest 的 Conformance。

生产覆盖至少需要：

```dotenv
SANDBOX_PROFILE=production
RUNTIME_IMAGE=haozhi152/agent-platform-runtime@sha256:<runtime-repo-digest>
```

同时通过 Compose override 挂载既有 Controller Socket 到 `/run/agent-platform-egress/egress-controller.sock`，并把固定 Resolver 文件只读挂载到 `/data/config/sandbox-resolv.conf`。缺失任何前提都会拒绝启动。完整边界见 [Sandbox 规范](../../docs/technical/sandbox-runner.md)和[生产验收](../../docs/technical/production-conformance.md)。

## 7. 验证与发布

产品与 Runtime 的模型无关真实检查：

```bash
python3 scripts/smoke-unified-image.py \
  haozhi152/agent-platform:local \
  --runtime-image haozhi152/agent-platform-runtime:local
```

它验证产品镜像不含五个 Runtime 命令、公开入口与认证边界、独立 Runtime 镜像的非 root / 只读 / 无网络执行，以及保留数据重启。

维护者在干净且精确匹配 `origin/main_temp` 的提交上登录 Docker Hub，再运行：

```bash
make image-publish
```

发布脚本先推送两个仓库的 ARM64/AMD64 版本 Tag；四个构建全部成功后才同时更新两个仓库的多架构版本、`latest` 和架构别名。凭证保留在 Docker Credential Store，不进入源码、镜像层或日志。

## 第三方组件

产品镜像保留 PostgreSQL、Keycloak、Nginx 和固定版本开源 MinIO 的许可证与来源；Runtime 镜像保留五个固定 CLI 及 Node/Python 依赖。MinIO 使用 `RELEASE.2025-10-15T17-29-55Z` 源码编译，不是需要商业许可证的 AIStor。构建输入见本目录 Dockerfile、`deploy/runtimes/unified/Dockerfile` 与项目 Go 模块依赖。
