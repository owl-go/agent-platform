# 首次安装脚本验证：2026-10-06

新增 `make install` / `scripts/install-platform.sh`，读取集成后的 `origin/main_temp` 快照。交互只要求 root SSH 地址、平台域名和管理员邮箱，可用 `--root` 指定目录，`--check` 只读检查。

自动步骤包括 Ubuntu 22.04/24.04 amd64 环境检查、Docker Compose 与 gVisor 安装、独立随机凭据、32 字节数据加密 Key、注册 RSA 签名 Key、受限配置、精确 OIDC 回调、初始 Administrator、身份 service-account 角色、静态 Web / 身份主题、私有存储、持久 Egress、服务启动、HTTPS / Readiness / Web 字节 / Migration 校验。安装成功后保存日常部署地址。

安装状态先记录版本，再允许上传和供应。中断后重试使用原版本和原配置；不轮换加密 Key、管理员密码、Bootstrap Subject 或注册签名 Key。完成安装和没有本脚本状态的已有安装会拒绝首次供应，并提示使用 `make deploy`。禁止接管其他 Docker 容器和已有平台持久卷。源码和 Web 上传校验路径与哈希，发布指针为相对路径，身份主题可由 Keycloak 的非 root 用户读取。

新安装所有 Runtime `available` / `native_resume` 与 CLI Builder 关闭，Runtime 镜像 Digest 留空。Compose 的非空 `RUNTIME_IMAGE` 占位只用于禁用状态，不作为可执行镜像或 Conformance 证据。模型、供应商注册应用、可选渠道与检索凭据仍在产品/运维入口配置。

已执行：

- `make deploy-test`：22 个部署测试、13 个安装测试通过，三个 Shell 入口语法通过。
- `go -C backend test ./internal/platformconfig`：使用真实安装生成器输出，API / Worker 严格 YAML 校验通过；检查所有执行能力保持关闭。
- `make test`、`make build`：通过。未设置 `WORKSPACE_TEST_POSTGRES_DSN`，数据库集成 Skip 不计为通过。
- 对生成的临时 env 执行三份真实 Compose 的 `config --quiet`：通过，不输出展开后的配置和密钥。
- 现有线上 `/opt/agent-platform` 执行安装 `--check`：预期拒绝，返回“安装目录已有文件…请使用 make deploy”；未安装依赖、未改配置、未重启服务。
- 文档相对链接、`git diff --check` 与 `cmp -s AGENTS.md CLAUDE.md`：通过。

安装说明从 183 行压缩到 26 行；旧说明放入手工安装参考，普通安装不再逐项编辑三个配置文件。

环境限制：没有可用于本次验收的全新 Ubuntu 主机。包供应、真实初始 Keycloak 导入/管理 API、TLS 和新数据库首次 Migration 的整体闭环尚未在新服务器执行；启动顺序与角色配置使用隔离 fake，严格配置与 Compose 校验使用实际解析器。此变更不新增 Linux + runsc、Runtime / Connector 或模型 Production Conformance 证据，也不改变现有服务器的 Runtime 开放状态。

安装源与接口依据：[Docker 官方 Ubuntu APT 安装](https://docs.docker.com/engine/install/ubuntu/)、[gVisor 官方 APT 安装](https://gvisor.dev/docs/user_guide/install/)、[Keycloak Admin REST API](https://www.keycloak.org/docs-api/latest/rest-api/index.html)。系统包来自签名 APT 仓库；安装使用镜像模板中的固定业务依赖 RepoDigest，记录实际 Docker / runsc 版本可从受限安装日志核查。
