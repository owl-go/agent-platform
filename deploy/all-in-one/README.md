# 单镜像 Docker 安装与源码部署

一个常驻容器内含 Web、API、Worker、Keycloak、PostgreSQL 17 和开源 MinIO，五个固定版本 Runtime CLI 也在同一个镜像里。任务执行时仍创建临时隔离容器，复用这一个镜像；这不是把不可信任务直接放进数据库/认证进程。默认是单用户可信本机的显式 runc 模式，生产隔离不能用本地验收代替。

## 1. 安装 Docker

需要 Docker Engine / Docker Desktop、Compose v2；从源码构建还需要 Git 与 Buildx/Bake。Docker 安装见 [Docker 官方安装入口](https://docs.docker.com/get-started/get-docker/)。建议给 Docker 分配至少 4GB 内存，并为包含五个 CLI、认证与数据库的镜像及持久数据预留足够磁盘空间；这不是高可用集群部署。

## 2. 直接拉取并启动

```bash
docker pull haozhi152/agent-platform:latest
docker run -d --name agent-platform \
  --restart unless-stopped --stop-timeout 60 \
  --security-opt no-new-privileges --pids-limit 2048 --memory 4g \
  -p 127.0.0.1:8080:8080 \
  -e PUBLIC_URL=http://localhost:8080 \
  -v agent-platform-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  haozhi152/agent-platform:latest
docker logs -f agent-platform
```

首次启动自动初始化数据，等待容器 `healthy` 或日志出现 `Agent Workspace ready` 后访问 `http://localhost:8080`。默认产品管理员用户名 **`admin`**，密码 **`admin123`**。这是首次初始化值，不是每次启动的密码重置逻辑。首次使用后务必在身份账号界面修改密码，或首次启动就用 `-e ADMIN_PASSWORD='你设置的强密码'` 覆盖。`ADMIN_USERNAME` 也可在空数据卷初始化时覆盖；已有数据不能通过这些变量改名或重置密码。

Keycloak 在镜像构建时完成优化，运行时使用正式 `start --optimized` 与单实例本地缓存，不使用开发启动模式。内部 HTTP 只监听 loopback，公开访问的 TLS 仍由外部代理负责。首次数据库迁移或低速磁盘可能需要数分钟，认证启动就绪上限为 10 分钟；子进程失败会立即退出，而不是等待超时。

不发布 PostgreSQL、MinIO、Keycloak 或 Worker 的内部端口。Docker socket 相当于对 Docker 主机的高权限控制，仅挂给受信任的控制容器；不挂给 Runtime，且不使用 `--privileged`。不要将默认弱密码或本地 runc 模式直接暴露到公网。

首次登录后在“设置 → 模型设置”添加供应商、选择模型并保存企业默认，再创建会话。镜像不包含你的供应商 Key、既有本地账号、会话或对象数据。

也可使用本目录唯一一个服务的 Compose：

```bash
docker compose -f deploy/all-in-one/compose.yaml up -d
```

### 架构与版本

- `haozhi152/agent-platform:latest`：包含 ARM64 和 AMD64 的 manifest，Docker 按宿主自动选择。
- `haozhi152/agent-platform:arm64`：Apple Silicon / ARM64 Linux。
- `haozhi152/agent-platform:amd64`：x86-64 Linux。
- 每次发布还保留 `<日期>-<提交前12位>-arm64`、`…-amd64` 和对应多架构版本标签。正式更新应记录实际 Digest，而不只记可移动 Tag。

### 改端口或域名

更换本机端口时同步设置 `-p 127.0.0.1:18080:8080 -e PUBLIC_URL=http://localhost:18080`。无需重新构建前端。公开使用时必须由外部 HTTPS 反向代理提供 TLS，`PUBLIC_URL` 设置为精确 HTTPS Origin；同时覆盖初始管理员密码。非环回 HTTP Origin 或公开环境的默认弱密码会拒绝启动。反向代理须保留 API/SSE 长连接；证书和公网发布仍由操作者配置，不自动修改云服务器。

公开 Origin 还必须显式设置 `SANDBOX_PROFILE=production` 并满足第 5 节的 Linux + runsc 前提；本地 runc 只允许环回 Origin，不因设置了强密码就变成生产模式。

## 3. 从源码构建并部署同一个镜像

```bash
git clone https://github.com/owl-go/agent-platform.git
cd agent-platform
git checkout main_temp
# 默认构建本机架构（Apple Silicon 为 ARM64，x86-64 为 AMD64）
make image-build
# 或 AMD64
docker buildx bake -f deploy/all-in-one/docker-bake.hcl \
  --set '*.platform=linux/amd64' --load app
```

得到 `haozhi152/agent-platform:local` 后，用上面的 `docker run` 命令，将镜像标签改成 `:local`。Go、Node、pnpm、PostgreSQL、Java 和 MinIO 都在构建阶段取得，主机不需要分别安装。源码和 Docker 拉取安装使用相同启动与数据布局，不需要逐个启动组件。

源码开发还可使用根目录的 `make local-up`，但这是调试方式：API/Web/MinIO 本机运行、基础设施容器运行，与本页单容器部署不同。新源码开发实例也自动创建 `admin / admin123`；已有实例保留旧账号和随机密码，运行 `make local-credentials` 查询，不重置已有数据。

## 4. 数据、更新和备份

`/data` 是必须持久化的数据卷，含 PostgreSQL、私有 MinIO 对象、Workspace、状态密钥与私有配置。丢失加密密钥会使既有凭证无法解密；不允许只备份数据库而漏掉状态文件和对象。数据卷不进入镜像，也不要提交到 Git。

更新前先停止任务并做一致性备份，再停止旧容器、拉取版本、用相同数据卷和 `PUBLIC_URL` 创建新容器。不要运行 `docker compose down -v` 或删除数据卷。已有 Keycloak Realm 只同步公开客户端回调地址，不覆盖 User 密码。结构迁移仍由 API 的不可变 Migration 启动路径执行；不能把镜像回退视为 Schema 或数据回退。

查看状态：`docker inspect agent-platform --format '{{.State.Health.Status}}'`。容器内关键进程退出会使整套服务退出，使用上述 restart 策略重启；SIGTERM 会先停止 Worker，再依次关闭 API、代理、认证、对象存储与数据库。

## 5. 生产 runsc

默认单容器是 ADR-0046 的本地模式，CLI Connector、CLI Builder 和消息渠道关闭。生产仍要求 Linux + runsc、Registry RepoDigest、受管 Egress Network、固定 Resolver 和宿主 Egress Controller，不会因缺少 runsc 自动降级。

高级整合部署可显式设置 `SANDBOX_PROFILE=production`、`RUNTIME_IMAGE=haozhi152/agent-platform@sha256:<精确Digest>`、`EGRESS_NETWORK`、`EGRESS_SUBNET`，挂载既有 Controller Socket 到 `/run/agent-platform-egress/egress-controller.sock`，并把固定 Resolver 文件只读挂到 `/data/config/sandbox-resolv.conf`。缺失任一前提即拒绝启动；主机策略按[Sandbox 规范](../../docs/technical/sandbox-runner.md)准备，并在指定 Digest 上执行适用 Conformance。原[分离式生产安装](../platform/installation.md)继续保留。没有 runsc/生产 Conformance 证据时，不将整合镜像标记为生产验收通过。

## 6. 维护者发布

生产 Runtime 默认不可用；完成指定 Digest 的 Conformance 后，才可通过 `VERIFIED_RUNTIME_ENGINES=codex,claude` 显式启用已验证引擎。这个操作声明不替代验收证据，不能把本地 runc 测试当成生产验证。

先提交并推送功能分支，合入 `main_temp`，在其准确提交上运行门禁和实际镜像验收，然后 `docker login -u haozhi152`，再运行：

```bash
make image-publish
```

发布脚本拒绝脏工作区和不等于 `origin/main_temp` 的 HEAD。先推送两个架构的版本化标签，两者都成功后才切换 `latest` 多架构 manifest 与架构别名；登录凭证留在 Docker Credential Store，不写到源码、镜像层或日志。

模型无关的真实容器测试：`python3 scripts/smoke-unified-image.py haozhi152/agent-platform:local`，用独立临时卷验证启动、公开配置、认证边界、同镜像隔离执行和保留数据重启。`--keep` 可保留唯一命名的测试实例供浏览器验证；不加时只清理本次创建的测试容器和卷，不触碰既有安装。

整合镜像的 Runtime 冒烟检查用 `UNIFIED_CONTROL_IMAGE=true RUNTIME_IMAGE_REF=haozhi152/agent-platform:local make runtime-image-smoke`，显式选择任务 Entrypoint 与非 root 用户，不启动控制服务。跨架构模拟运行不能替代对应架构硬件上的生产验收。

## 第三方组件

本镜像整合现有固定版本 CLI 与其开源组件，许可证随各上游发行包保留。MinIO 使用开源 `RELEASE.2025-10-15T17-29-55Z` 源码编译，不是需要商业许可证的 AIStor；其对应源代码及构建输入见 [MinIO 源码](https://github.com/minio/minio/tree/RELEASE.2025-10-15T17-29-55Z)、本目录 Dockerfile 和 Go 模块依赖。其他基础组件来源为 [PostgreSQL](https://github.com/postgres/postgres)、[Keycloak](https://github.com/keycloak/keycloak)、[Nginx](https://nginx.org/en/) 及各 Runtime 镜像列出的固定 npm/PyPI 包；分发者应保留相应许可证与源码信息。
