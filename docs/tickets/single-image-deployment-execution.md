# 单镜像部署执行记录

日期：2026-10-18。会话分支：`codex/local-deployment`。基线先合并 `origin/main_temp`，Merge Commit 为 `c6f857e`；未操作欠费云服务器，也未把旧安装业务数据迁入新整合实例或覆盖其数据。

## 验收范围与状态

| AC | 规则 / 门禁 | 当前证据 |
|---|---|---|
| AC-IMAGE-1：Web、API、Worker、认证、数据库、对象存储共用一个镜像与常驻容器 | ARC-003、COD-003 / G-DEPLOY、G-SEC | 最终 ARM64 / AMD64 镜像均通过真实首次启动、公开健康路由、OIDC、认证拒绝边界和保留数据重启；AMD64 为模拟运行。 |
| AC-IMAGE-2：首次创建 admin / admin123，保留数据不重置密码 | COD-003、TST-001 / G-DEPLOY、G-E2E | 开发验收实例默认管理员浏览器登录通过；最终两架构重启及本机最终版升级后 state.json SHA-256 不变。密码变更后的恢复演练未做。 |
| AC-IMAGE-3：同镜像隔离 Runtime，本地 runc、生产 runsc | ARC-002、COD-003/004 / G-RUNTIME | 两架构固定 CLI、PI SSE 防护与非 root / 只读 Runtime 冒烟通过；最终镜像实测任务健康探针为 NONE，控制容器健康检查保留。生产默认关闭未声明验证的引擎，Linux + runsc Conformance 未运行。 |
| AC-IMAGE-4：ARM64 / AMD64 Docker Hub 标签 | TST-005 / G-DEPLOY | 两架构版本标签、多架构版本、latest、arm64、amd64 已推送；匿名远端清单、平台 Digest 和两平台拉取已核对。 |
| AC-IMAGE-5：Docker 与源码部署文档 | REQ-005 / G-DOC | README、整合安装手册、ADR-0047、维护者发布脚本已添加；发布仅接受干净且精确匹配 origin/main_temp 的提交。 |

## 已执行门禁

- `make test`、`make build`、`go -C backend vet ./...`：通过。未配置 `WORKSPACE_TEST_POSTGRES_DSN`，相关数据库集成 Skip 不记为通过。
- `make web-typecheck`、`make web-build`：通过；Vite 原有大 Chunk 提示仍存在。
- 前端完整 Vitest：59 个文件、696 个测试通过；OIDC 定向 9 个测试通过。
- platformconfig、tokenverifier、runtimeexecutor 目标包及其 `-race`：通过。
- `make deploy-test`：整合配置与启动边界 9 项、发布保护和双模式 Shell 兼容 5 项、部署脚本 23 项、安装脚本 13 项、本地脚本 4 项通过，包含启动重试、受控命令错误脱敏和第二架构失败时不切换 latest。
- ARM64 Runtime 镜像冒烟：`UNIFIED_CONTROL_IMAGE=true RUNTIME_IMAGE_REF=haozhi152/agent-platform:dev-arm64 make runtime-image-smoke` 通过。
- Shell 语法、Compose 配置、Bake 配置、`git diff --check`、`cmp -s AGENTS.md CLAUDE.md` 已通过；最终变更后需补跑。

## 首次失败与恢复

1. PostgreSQL initdb 首次失败，确认缺少 libpq 动态库；补入对应上游库并增加 initdb / psql 版本检查。
2. 补库后确认 PostgreSQL 配置样板的符号链接目标缺失；补入上游公共样板文件。
3. 数据库启动探测未重试临时非零退出；调整受控命令错误为可重试类型，并测试捕获输出不进入错误日志。同样同步处理不存在的本地 Runtime Network 的创建分支。
4. 双架构并行构建使主机可用空间降至约 250MB，Docker 元数据写入报 I/O error。用户已授权只清理未使用构建缓存；已执行的两次清理分别回收约 1.169GB、3.019GB，不删除镜像、容器或卷。后续全量未使用缓存清理也被 Docker 元数据 I/O 错误阻断。

5. 用户确认重启后，Docker Desktop 正常恢复，`docker buildx prune --all --force` 回收 13.46GB 构建缓存，主机恢复约 13GiB 可用；镜像、容器和数据卷未删除。原本运行的三个本地服务已重新启动。
6. 用户明确授权迁到 Seagate Bas 外置盘，禁止格式化或影响已有文件。该盘为 ExFAT、迁移前约 765GiB 可用。仅创建新目录 `/Volumes/Seagate Bas/AgentPlatform-Docker`，通过 Docker Desktop 的 Resources → Advanced → Disk image location → Apply & restart → Yes, move it 启动官方迁移流程，目标为其下的 `DockerDesktop` 目录；迁移前已正常停止三个本地服务。迁移前清单保留 16 个容器、296 个数据卷、17 行镜像标识用于完成后比较。ExFAT 为最大约 228GiB 的 Docker.raw 预分配完整空间，目前复制仍在进行，不宣称迁移成功。

7. 13:46（Asia/Shanghai）完成迁移，Docker Desktop 显示 Engine running；虚拟机进程已打开外置盘的 Docker.raw，旧位置文件已由官方迁移流程移走。前后清单逐项比较一致：16 个容器、296 个卷、17 行镜像标识，无缺失、无新增；内置盘约 51GiB、外置盘约 537GiB 可用。没有格式化外置盘，未改动其中已有文件。外置盘 ExFAT 占用完整约 228GiB 的虚拟磁盘空间，必须保持连接，安全弹出前先停止 Docker。随后三个旧服务均恢复运行，API 与 Worker `/readyz` 返回 ready。
8. 整合启动预检发现上游 Java 的 conf、security 与 tzdb 为运行目录外的链接；补齐 `/etc/java`、`/etc/pki`、系统 Crypto Policy 与 Java 时区数据库，保留其许可证。构建检查增加完整 Keycloak CLI 启动，并提前执行 PostgreSQL 版本的 Keycloak build，运行改为正式 `start --optimized`、单实例 local cache；不在镜像层写入数据库/认证 Secret。Keycloak 首次就绪等待有 600 秒上限，子进程退出仍立即失败。
9. 外置盘上双架构 Go/CLI 并发构建出现长时间无进度、Docker 查询超时。只读诊断记录过 `docker ps` / `buildx du` 约 0.2 秒响应，压力下超过 10 秒，停止本任务的并发构建后恢复到 33ms；原有三个服务和数据未删除。改为宿主 Go 1.25.14、CGO=0、Linux ARM64/AMD64 串行交叉编译，两个架构的 API/Worker/MinIO 均由 file 确认静态 ELF，临时产物放忽略目录；组装使用 Buildx 的 named context 覆盖 build stage。ARM64 复用本地固定版本 Runtime 层，使用当前已验证镜像中相同 PI 版本的修补文件，已有 Runtime 的五引擎、SSE、权限和 Builder 冒烟再次通过；不会把临时编译目录纳入源码提交或普通构建上下文。
10. 系统 Bash 3.2 揭示新增空数组在 nounset 下的兼容问题；Runtime smoke 改用函数区分普通/整合模式，保留普通 Runtime 与 Builder 的默认用户检查。新增双模式脚本回归与发布保护共 5 个测试通过。整合初始化边界共 6 项通过，新增“已有数据但 state.json 丢失时拒绝生成新密钥”，避免数据恢复不完整时静默轮换。

14:18：ARM64 最新整合镜像已构建，Java 完整启动检查和 Keycloak 优化构建成功；真实首次启动与保留数据重启仍在验收，不能仅以构建成功认定可用。Go 1.25.14 下完整 test/build/vet、Web typecheck/build、deploy-test 与格式检查再次通过；数据库环境 Skip 和生产 runsc 缺失的证据边界不变。

接下来完成原有服务健康检查，重建和执行首次启动、浏览器登录、同镜像隔离任务及重启验收，最后完成 AMD64 与 main_temp 集成发布。保留当前功能分支及改动，不重写历史。没有生产 Sandbox / Production Conformance、远端发布或数据恢复演练完成的声明。

## 整合启动回归

- MinIO 健康接口先于 IAM 初始化完成，12 次真实启动中 4 次 bucket 初始化遇到 `503 XMinioServerNotInitialized`。增加有界重试，只接受这一明确的初始化错误；签名错误仍立即失败，不输出签名响应正文。修复后相同 12 次探测全部成功，并添加 503 重试 / 403 拒绝回归测试。
- Keycloak 优化构建必须固定 `/identity` 相对路径，否则正式启动拒绝不同的 build-time 选项；Worker 的 Egress Controller Socket 即使本地模式也须使用规范保留 basename，已统一为 `egress-controller.sock`。
- Nginx 对 `/api/healthz`、`/api/readyz` 增加与现有 Vite/Caddy 一致的精确重写。原先 API 本身就绪但前端探测被鉴权拒绝，会错误显示离线；已加入真实镜像 Smoke 检查。
- 浏览器已用默认 `admin / admin123` 登录开发验收实例，显示 Administrator、API 在线和四个产品可选 Runtime 可用。Mac 锁屏暂时阻断后续点击验收，不将受阻操作记为通过；不通过修改前端绕过锁屏。
- 镜像保留 MinIO 对应的 AGPL License 文件和固定版本源码地址；未把任何测试卷、供应商 Key 或宿主凭证放进镜像。
- 14:34 ARM64 `smoke-unified-image.py ...:dev-arm64-fixed --keep` 通过：独立测试卷首次 healthy、首页与公开配置、健康/就绪路由、OIDC issuer、匿名 API 401、同镜像 Runtime UID 65532 + 只读/无网络/无特权 + 数据卷映射，以及容器重启后再次 healthy、state.json 摘要不变。测试实例 `agent-platform-smoke-d85dfd444a`，未导入旧安装数据。
- 14:49 ARM64 版本 `2026-10-18-f9ad05ce00b4-arm64` 推送成功，远端拉取和匿名 Registry 清单查询均确认 Digest `sha256:1c66f1a89eed4f8ed58ef72e18a7c9495caf225b13323d9f4e4f7b46b720e8f7`；该版为后续边界补齐前的中间版本，不提前切换 latest。
- AMD64 五引擎、PI SSE、非 root / 只读 / 文件 Broker 和 Builder 冒烟通过，属于 ARM Mac 上模拟运行，不能替代原生 AMD64 或 Linux + runsc 验收。版本标签上传遇到一次本地代理 broken pipe，构建工具自动重试；不输出带签名上传状态参数的错误 URL，不把上传中计为成功。
- 真实临时 Runtime 的健康状态暴露控制探针继承问题；在无控制进程的非 root Runtime 中执行控制探针确实退出 1。先新增失败回归，再由 Worker 专用 Docker shim 为 create/run 添加 `--no-healthcheck`；主服务健康检查不变。新增真实 Smoke 检查 Docker Config.Healthcheck.Test 必须为 NONE，同时执行 UID 65532 探测，后续以补齐边界后的发布版本为准。

## 最终发布与交接

发布代码提交为 `1eaca52efd849bd75f63b80c12536fa6f6bfd98b`，已推送功能分支并以 Fast-forward 集成到 `main_temp`。构建与发布时工作区干净，HEAD 精确匹配 `origin/main_temp`；未修改保护分支 `main`。后续执行记录更新只记录证据，不改变镜像代码 Revision。

- ARM64 最终版本 `2026-10-18-1eaca52efd84-arm64`：独立卷 `agent-platform-smoke-e0fa10d9d8-data`，完整 Smoke 通过。
- AMD64 最终版本 `2026-10-18-1eaca52efd84-amd64`：独立卷 `agent-platform-smoke-e0c0a92e09-data`，完整 Smoke 通过。首次运行的镜像展开和模拟执行较慢，没有把 ARM Mac 结果算作 AMD64 原生生产验证。
- 两个 Smoke 均验证首页、动态公开配置、公开健康/就绪路由、OIDC issuer、匿名业务 API 401、同镜像 runc UID 65532、只读/无网络/无特权、数据卷映射、任务健康探针 NONE、保留数据重启，以及 state.json 摘要不变。
- 两个架构版本均上传成功后才更新 `2026-10-18-1eaca52efd84`、`latest`、`arm64`、`amd64`。匿名 Registry 请求三个别名均为 200；latest 恰好包含 linux/arm64 与 linux/amd64，架构别名分别只有对应平台。显式按两个平台拉取 latest 均成功。

| 类型 | Registry Digest |
|---|---|
| latest / 版本化多架构 Index | `sha256:0bb4cbb6669272ada79ebb706e9bf62174e6d1211cb20bd8d42a8e025518872e` |
| ARM64 平台 Manifest | `sha256:d6968fb3d2deb19e0b8a4cee58f4f753a1b056f838bfa1115cfbbfc181667557` |
| AMD64 平台 Manifest | `sha256:d8264f383b699edadab576b240c1b4e94c428a97825201bb895caf3afecc708f` |

架构别名本身是单平台 Index，分别指向上表平台 Manifest；其 Index Digest 为 ARM64 `sha256:9d8a7cc62c8c5a88dcb4794158e204bb0f615641e07dd0025f51ec59cc7122bd`、AMD64 `sha256:f749792fbc180d6c7ace208aa7cae872253c701784d3bc2dcbadb02c2214f3a9`，不与平台 Manifest Digest 混用。

本机 `agent-platform` 在 `http://localhost:8080` 运行最终 ARM64 版本，状态 healthy，`/api/readyz` 返回 ready；首次默认账号 admin / admin123。升级沿用 `agent-platform-data`，升级前后状态文件摘要一致，上一版容器保留为停止状态的 `agent-platform-backup-f9ad05c`。当前只是新整合安装，旧源码开发安装的业务数据没有自动迁入，原服务和数据仍保留；镜像不携带供应商 Key。唯一命名的镜像测试容器已停止，测试卷保留用于证据复查。

Docker 虚拟磁盘继续位于 `/Volumes/Seagate Bas/AgentPlatform-Docker/DockerDesktop/Docker.raw`。最终仍有外置盘约 537GiB、内置盘约 44GiB 可用；未格式化或操作外置盘原有文件。使用 Docker 时必须连接此盘，安全弹出前先停止 Docker。

本轮没有原生 AMD64 硬件验收、Linux + runsc Sandbox / Production Conformance、独立 PostgreSQL 集成环境或密码变更后的备份恢复演练。生产保持 runsc 与 Registry RepoDigest 的拒绝降级边界，不把本地 runc 或模拟测试描述成生产验证。

## 2026-11-03：game 服务器基础安装

用户授权通过 `ssh game` 部署，保留原网站，不部署 RAGFlow，但保留后续真实 AI 对话的执行路径。用户选择自行配置模型，不迁移本地供应商 Key、会话、连接器账号或系统密钥。本次为全新安装，不是旧本地实例的数据迁移。

- 环境：Ubuntu 26.04 / 原生 amd64，2 vCPU、约 2 GiB RAM、2 GiB Swap。原网站及其 PostgreSQL / Redis 继续运行；这是资源受限的基础安装，不代表已验证生产容量或并发能力。
- 应用源码来自 `origin/main_temp` 的 `0741b9157b5db0c765af4badb4f3f346a31f2691`，以干净 `git archive` 构建，没有上传工作区未提交文件。产品版本标签 `2026-11-03-0741b9157b5d-amd64` 已推送；本次未改 `latest` 或其他架构别名。
- 产品固定为 `haozhi152/agent-platform@sha256:355982e17c2ed24e381541e10d226db4209926b1bfb0ee0bbec4cd225a4589ff`，独立 Runtime 固定为 `haozhi152/agent-platform-runtime@sha256:a3f1a38d8af2b34418b29c4b95aaa0f384b878d4f840448cace4866a4052a63c`。
- 安装目录 `/srv/agent-platform`，Compose 常驻容器 `agent-platform`，独立卷 `agent-platform-production-data`；仅发布 `127.0.0.1:18080`，由宿主 Nginx 提供入口。Web/API/Worker/Keycloak/PostgreSQL/MinIO 共用产品容器；未部署 RAGFlow。
- 为新安装生成独立系统凭据和强管理员密码。用户名 `admin`，访问说明保存在仅 root 可读的 `/srv/agent-platform/config/admin-access.txt`；没有使用公开弱密码，也未读取或输出该文件内容。
- Docker 新容器默认进入 `agentplatform.slice`，平台总量 `MemoryHigh=1100M`、`MemoryMax=1350M`、`MemorySwapMax=1536M`、`CPUQuota=150%`，不把已有原站进程纳入此 Slice。应用另有容器资源限制。最终检查约使用 527 MiB Swap，说明当前容量余量有限，不承诺稳定并发。
- 安装 `runsc release-20261102.0`，启用 `--host-uds=open`，保持 `production` / `runsc` / Registry RepoDigest；没有 `runc` 回退。Egress 策略与宿主 Controller 为 systemd 服务，Docker 重启后恢复；临时镜像下载代理与 SSH 转发已停止使用。

### HTTPS 与访问

- 当前规范入口为 `https://134.175.90.85/sessions`，OIDC issuer、授权地址、Token 地址及回调使用同一 HTTPS IP Origin。
- IP 证书由 Let's Encrypt 签发，SAN 包含该 IP；Certbot 5.4.0 / ACME 5.4.0 使用隔离 venv，systemd timer、Nginx deploy hook 已配置。`certbot renew --cert-name agent-platform-ip --dry-run --no-random-sleep-on-renew` 实际通过。
- 用户提供的 `agent.shunyuit.cloud_nginx.zip` 已校验域名、有效期及证书/私钥匹配，证书有效至 2027-02-01（客户端日期口径）。私钥安装到 `/etc/nginx/ssl/agent.shunyuit.cloud/privkey.pem`，root:root / 0600；原始 ZIP 归档在服务器的受限配置目录，私钥未进入仓库、镜像或日志。
- 用户说明子域名备案尚未完成。域名证书已安装，但外网 HTTPS SNI 连接仍重置，不将 Nginx 本机配置检查记为域名公网可用。域名 HTTPS 配置先重定向到 IP 规范入口，备案完成后再显式切换唯一 OIDC Origin，不能仅做代理别名而破坏浏览器状态与回调。

### 实测结果与缺失证据

| AC / 门禁 | 结果 |
|---|---|
| AC-GAME-BASE / G-DEPLOY | `make deploy-test` 全部通过；原生 amd64 镜像真实启动，容器 healthy / RestartCount 0，外部页面 HTTP 200、`/api/readyz` ready，内部组合健康探针通过；原站 HTTPS 200。 |
| AC-GAME-AUTH / G-SEC | 外部匿名 `/api/v1/sessions` 返回 401；公开 OIDC Discovery 返回精确 IP issuer 与端点；管理员初始化成功。浏览器自动化创建验收页超时，未声明完成浏览器管理员登录。 |
| AC-GAME-RUNTIME / G-RUNTIME | 在实际 Linux + runsc 中验证五个固定 CLI 版本，UID 65532、只读 Rootfs、移除 Capability、no-new-privileges、资源限制及不挂 Docker Socket；受管网络访问公开 HTTPS 返回 200。只是基础检查，不是完整 Production Conformance。 |
| AC-GAME-EGRESS / G-SEC | 初测云元数据与网桥宿主被阻断，但本机公网 IP 因云 NAT 回环仍可访问。通过既有 `AGENT_EXTRA_DENY_CIDRS=134.175.90.85/32` 增加本机公网地址拒绝后，同一组测试全部阻断；规则随 systemd 持久恢复。 |

首次构建因 Docker 内 Go 依赖直连超时失败，改用现有受信任工作站代理后相同固定版本构建通过；服务器不能直连 Docker Hub，镜像准备经临时 SSH 转发，完成后移走临时 Docker Proxy drop-in 并重启 Docker。宿主 Controller 初次因 `/run/xtables.lock` 不存在而拒绝启动，增加 tmpfiles 的受限锁文件后实际启动成功。保留这些首次失败，不以重试代替原因。

真实模型供应商尚未由用户配置；模型调用、Session 流式/取消/恢复、Workflow、CLI Connector bundle × RepoDigest 联合验证和完整 Production Conformance 未执行。按现有生产验收契约，未取得真实证据的 Runtime 仍未标记 verified/available，不能把固定 CLI 版本检查写成真实 AI 对话通过。下一步由用户在管理员界面填写供应商，随后对准确 Digest 验证真实调用，再启用通过验收的引擎。没有开启 Native Resume、CLI Builder 或消息渠道。未执行本安装的备份恢复演练或长期压力测试。

### 同日补充：默认全部启用（已上线，以下保留过程）

用户随后明确要求当前及后续部署的全部引擎与内置连接器默认启用，见 ADR-0050。线上五个引擎已通过持久配置启用，管理员实际登录后的 `/api/v1/runtime-engines` 返回全部 available；原 `runsc`、RepoDigest、Egress、资源、凭据及审批边界未降级。用户已自行添加一个模型连接与 27 个模型，配置未被替换。现有平台默认 Codex + gpt-6.1-sol 保持不变。

使用真实配置、当前生产 runsc RepoDigest 和相同 Runtime Executor 进行了无工具受控回复测试：Codex 19.3 秒通过、PI 6.0 秒通过；Hermes 150 秒超时、OpenClaw 正常结束但未返回预期标记，Claude Code 因此供应商不声明 Anthropic Messages 协议而明确拒绝。用户要求先打开引擎选择，故不再把上述组合验证状态等同于全局可选状态；不声称五个组合全部调用通过。

代码新增 `ENABLED_RUNTIME_ENGINES=all` 默认及名单/none 选项，保持旧名单变量兼容与可选 Capability 默认关闭。Migration 000090 新增架构隔离的生产默认包验证记录，无需伪造 legacy Definition；Worker 对受管理包自动校验私有对象 SHA 并在 runsc 中无凭据/无网络启动，通过后仅恢复初始待启用 Publication。API 使用本安装的实际生产目标，不重写包声明、Revision、历史快照或用户授权。明确 Administrator 停用仍保留。

当前门禁：目标 Go 包、`make test`、`make build`、`make deploy-test` 通过；整合配置回归扩为 14 项；独立 PostgreSQL 17 的本地/生产证据隔离、旧 Migration 重跑、失败不记通过、失败冷却及明确停用保护真实执行通过。尚待 main_temp 集成、新镜像构建/部署及线上 20 条目录验证；不把代码测试记为最终上线完成。

#### 自动验证失败的后续诊断

`e6e1149` 已从 main_temp 构建、推送版本化 amd64 标签并部署。真实管理员 API 返回五引擎全部可用；实际 Session `b6e77628-fd69-4e57-b2b9-377db4bc616e` 完成受控 Codex 回复。连接器首轮自动检查全部失败，未伪造通过或直接改写可用状态。相同配置的独立 Conformance 探针通过十个 CLI，目录实际恢复为 19 条（含飞书、钉钉、企业微信）。

根因一：整合 Supervisor 的 `umask 0077` 被 Worker 继承，解压代码未恢复包的目录和文件位，导致非 root runsc 读取 launcher 时 permission denied；独立 Docker exec 的不同 umask 解释了其成功。新增独立子进程回归 `TestExtractBundleReadableBySandboxUnderPrivateUmask` 在修复前准确报 0700，修复后验证目录可遍历、只读文件及执行位。不改变 Supervisor 的凭证权限；解压只恢复 allowlisted 模式并统一保护 bundle，凭证目录不受影响。

根因二：GitHub 的 32 进程上限下，Node spawnSync 原版 gh 返回 ENOMEM；原版 gh 直接启动正常，缩小 Node heap 或增加内存不能修复，仅将进程上限调到 64 后成功。新 0.1.1 修订保留原版程序、权限白名单和授权模式，将同一组新限额嵌入 bundle metadata 生成独立 SHA-256；历史 32 进程修订不会复用新证据。正式 Worker 按同一上限重新验证。b4f42c3 为未部署的中间构建；不能计作 GitHub 已开放或真实账号业务通过。

另补齐 managed CLI 修订升级后的自动启用意图：原已开放或等待首次验证的条目升级后仍等待新修订验证，明确 Administrator 停用仍不恢复。隔离 PostgreSQL 回归在修复前复现“原已开放条目升级后 pending=false”，其余两个状态正常；修复只更新同一 Publication 事务中的待启用标记。首次测试曾在测试 PostgreSQL 尚未就绪时报 EOF，未作为缺陷复现；实际红/绿证据以数据库 ready 后的运行结果为准。

#### 最终上线与双架构验证

发布源码为 `916fcbad4144ad6aeed387877a898ff943f576c1`，功能分支自动提交/推送并 fast-forward 集成到 `main_temp`。产品镜像由精确 `origin/main_temp` 的干净 `git archive` 构建，未包含原工作区两项历史未提交改动或任何部署凭据。`b4f42c3` 的中间版本未部署；最终 GitHub 0.1.1 将 64 进程、512 MiB、1000 CPU millis 写入 bundle metadata，原版两架构 ELF 与 launcher 逐字节保持一致。

两个仓库均已推送 `2026-11-03-916fcbad4144-arm64` 与 `2026-11-03-916fcbad4144-amd64`。全部四个版本引用在 Registry 核验成功、两个架构安装/默认启用检查通过后，才更新各自版本化多架构标签、`latest`、`arm64`、`amd64`；远端清单核验 latest 恰有两种 Linux 架构，各架构别名恰有对应单平台。Runtime 源码与已验证的 `477b258fb0aa` 相同，本次复用其不可变平台镜像，没有将重打标签描述为重新构建。

| 2026-11-03 镜像引用 | Registry Digest |
|---|---|
| 产品 latest / 版本化多架构 Index | `sha256:5147d95afff741eaf9786118317024c1b0562b7b58aad900295543c20deceef5` |
| 产品 ARM64 平台 Manifest | `sha256:776420ccabc56baf696ee97ef9026d86846472cb5c5aa662b26329185c25605a` |
| 产品 AMD64 平台 Manifest | `sha256:112f42fda4dc8066674c49bb98919770a7e41181b7b113b6d867036ebc35aef0` |
| Runtime latest / 版本化多架构 Index | `sha256:e87886e26da0f098c874f4ca1349b6a4f92ed484b5a7d87e7a6fa69c458ba3f8` |
| Runtime ARM64 平台 Manifest（复用） | `sha256:79f313b38a13ea8ea67f1dcba5cc192306b0a54e7090c03ea6cd4fac1aa12606` |
| Runtime AMD64 平台 Manifest（复用） | `sha256:a3f1a38d8af2b34418b29c4b95aaa0f384b878d4f840448cace4866a4052a63c` |

- 当前线上控制镜像固定为 `haozhi152/agent-platform@sha256:112f42fda4dc8066674c49bb98919770a7e41181b7b113b6d867036ebc35aef0`。独立 Runtime 继续使用原生 amd64 的 `a3f1a38d…` RepoDigest、production/runsc，未因默认启用而更改隔离或 Egress。
- 切换前确认活动执行为 0，并关闭入口后停止应用，生成 `/srv/agent-platform/backups/before-connector-fix-916fcba/data.tar.gz` 冷备份，权限 0600 / 目录 0700。SHA-256 校验与 tar 可读性检查通过；这是备份完整性检查，不是数据恢复演练。模型配置、27 条模型目录、平台默认及账号凭据保留。
- 后台 Worker 自动验证 GitHub 新 bundle 后恢复其 Publication。当前 11 个 CLI 的活动 bundle × 实际生产 RepoDigest × amd64 均有真实通过记录；旧 32 进程 GitHub bundle 没有被写成通过。新 bundle SHA-256 为 `62c413457f6af358fd1db18165fd1112de34c525c1261d49b475750bd93690cf`，最终规范化 Package SHA-256 为 `d8553581788355a82001cdae41b6b30c0b3826073b6bf2c4204955093d40de06`。
- 管理员实际登录后的 User catalog 与 Administrator publication-health 均为 20 条、全部 available/conformance_available；五引擎全部 available。部署后的同一受控 Codex Session 再次 completed 且标记匹配。应用 healthy / RestartCount 0，公网 readyz 成功，原网站 HTTPS 200，维护标记已清除。
- 实际 Linux/runsc 的 `TestDockerConnectorWaitsForPolicyBeforeCommand` allowed/rejected 两项通过，使用当前固定 Runtime、受管网络和真实 Unix Egress Controller；验证策略前不运行命令、不在等待容器配置中注入凭据、拒绝不执行、结束清理。这不等于完整 Production Conformance。

| AC / 门禁 | 实际结果 |
|---|---|
| AC-DEFAULT-ENGINES / G-GO、G-DEPLOY | 整合配置 14 项回归、相关 Go 包、`make test`、`make build`、`make deploy-test` 通过；配置默认全部引擎，显式 none/名单仍有效，可选 Capability 仍关闭。 |
| AC-DEFAULT-CONNECTORS / G-GO、G-DB | 私有 umask 子进程回归先红后绿；隔离 PostgreSQL 17 的生产/本地证据隔离、失败冷却及升级后自动恢复/明确停用保护真实通过。完整 Go 门禁未设置 DSN 的其他数据库 Skip 不算通过。 |
| AC-DEFAULT-IMAGES / G-DEPLOY | ARM64 与 AMD64 的 `smoke-unified-image.py --keep` 均完成首次安装、公开路由/匿名拒绝、非 root 只读隔离任务、保留数据重启和 state 摘要稳定。各自真实 HTTP 登录后的 API 均为 5 引擎、20 条 User/Administrator 目录、零临时 legacy Definition，全部 CLI 由新实例自动验证。AMD64 本机检查为 ARM Mac 模拟；线上另有原生 amd64/runsc 验证。 |
| G-DOC | `git diff --check`、AGENTS/CLAUDE 镜像一致性及本次路径检查通过；当前记录为发布后的补充，不改变镜像源码 Revision。 |

HTTP 登录验收首次遇到 Python CookieJar 不发送 localhost 的 Secure Cookie，返回 400；仅在标签隔离的测试实例固定 localhost Origin 模拟[浏览器的 localhost 例外](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)，生产 Cookie 与应用鉴权没有调整。该项是 HTTP/API 验收，不声称完成真实浏览器界面验收。两个新建 Smoke 容器及其带测试标签的独立卷均已按精确名称清理，未删除原有服务、镜像或业务数据。

真实外部连接器账号授权与业务 API、所有引擎/模型组合、完整 Sandbox/Production Conformance、备份恢复及容量/长期压力检查仍未完成。上述“默认启用”与目录中的启动证据不表示这些项目已通过。

#### 设置页漏项补齐（已发布）

最终核对发现 `SettingsPage.refresh` 仍按品牌过滤 OpenClaw，并在保存前删除它的默认模型、替换已有 OpenClaw 引擎选择。此前“五引擎”证据是后台 API 状态，不能等同于设置页五个都可选。产品需求的 Runtime Settings 契约与本次“全部启用”均要求显示五个现有引擎；移除该过滤和静默改写，沿用现有控件与可用性/协议校验。

组件回归先复现 OpenClaw option 缺失，修复后目标 13 项通过；前端完整 696 项、`make web-typecheck`、`make web-build` 通过。后端与 Runtime 镜像源码没有新增变化，既有 runsc、CLI 及模型验收证据不改写。浏览器操作使用独立标签验收实例，绑定本次构建的前端产物，不使用线上账号或数据库；界面和最终镜像更新结果将在完成后补充。

Playwright 已通过真实 OIDC 登录独立实例，在 1440×900 与 375×812 验证个人/企业默认设置中的五个引擎，取消继承后的 OpenClaw 及企业默认 OpenClaw 均可选择。重新加载后的继承模式仍按契约禁用个人覆盖控件，不将此状态写成引擎不可用。前端市场实际显示 20 条品牌条目，首组恢复为含钉钉、飞书、企业微信的“沟通协作”，不再从项目管理开始；未操作外部账号授权。

同时复现旧全局 input 规则覆盖隐藏性格 radio 的 1px 宽度，导致手机页面 scrollWidth 589 > 375；局部提高设置页隐藏 radio 选择器的优先级并固定其 1px 盒子后，个人与模型页在两个视口均无横向溢出（375/375、1440/1440）。没有改变可见卡片视觉或其他页面控件。修改后类型检查与构建再次通过，浏览器尺寸/交互/截图证据在忽略目录 `output/playwright/all-engines-*` 与 `all-connectors-desktop.png`。独立实例没有模型供应商，截图中的空目录不是线上模型配置被清除。首次在服务 ready 前导航产生 empty response、继承模式下选择产生禁用超时，均按真实状态保留，没有绕过控件或修改鉴权。

最终设置页源码 `441ebabd539bd2853e289bdd8a6874ab84cef496` 已提交、推送并经 main_temp 发布。`4cd1386` 为未部署的中间 UI 构建。最终两架构镜像的 API、Worker、MinIO 可执行文件 SHA-256 均与已完成启动/重启及默认启用验收的 916fcba 镜像逐项一致，后端、部署和资源目录 Git diff 为空，因此未重复同一套后端测试或模型调用。新增前端组件、类型/构建、真实浏览器操作及视口检查用于验证本轮变化。

| 最终产品引用（441ebab，取代上述 916fcba 当前入口） | Registry Digest |
|---|---|
| `latest` / `2026-11-03-441ebabd539b` 多架构 Index | `sha256:29199bdc8ba085c128456bfefea62806779777cb85a2f8d23ed679679cf13038` |
| `2026-11-03-441ebabd539b-arm64` 平台 Manifest | `sha256:1def605c999aaed276dfbf2c853558ed4ab12045777ebb2d5d96ff8389164f80` |
| `2026-11-03-441ebabd539b-amd64` 平台 Manifest / 线上固定引用 | `sha256:7d9f0c800640c2bebe0361c374147f4a48acca78c583182eb8f3c39071bf72b8` |

Runtime 的对应版本标签复用相同两架构平台 Manifest，latest Index 仍为 `e87886e2…`。四个版本引用先核验成功，再更新两仓库的 latest/架构别名；远端 latest 各恰含 ARM64、AMD64，两类架构别名各恰含相应单平台。源码仍使用精确 main_temp 的干净 git archive 构建，未纳入历史未提交文件。

切换前再次确认没有活动执行并生成 root-only 冷备份 `/srv/agent-platform/backups/before-settings-ui-441ebab`，校验通过。最终线上 healthy / RestartCount 0，维护标记清除；管理员实际登录后仍为五引擎和 20 条 User/Administrator 目录，11 条生产 CLI 证据全部保留，旧 GitHub bundle 仍不记通过，平台默认未重写。公网前端 index SHA-256 为 `1a68c9f933c18516e966c2e84ac2f0b101a3311bda24c06c7057ed31cdad5a0c`，与两架构镜像及本次浏览器所用构建产物一致。原网站继续 HTTPS 200。新建的浏览器验收实例与标签隔离卷已清理；最终只追加本次前端证据，没有将启动检查扩写为全部真实第三方业务或完整 Production Conformance 通过。

### 同日：智能助手列表渲染空值修复（已上线）

用户截图显示 SmartAssistantsPage 读取 null.length 后整页列表不呈现。通过所属账号的真实 API 只读核验：两条智能助手记录仍存在，均为 draft，未绑定知识库字段为 JSON null，分享配置是正常对象。没有输出名称、提示词、分享 Token 或供应商凭据；没有恢复、删除或重建任何用户记录。

前端 API 边界新增 SmartAssistant 响应规范化，将 null/缺省 knowledge_base_ids 转为空数组，保留实际绑定和其余字段；覆盖列表、详情、创建、更新、复制、状态、图标/Widget 图标及分享 Token 响应。空列表也规范化为数组。后端接口、数据库和权限语义不变。通过同一 HTTP client 将真实空值形状送入 Vue 卡片渲染，覆盖未绑定与已绑定两个条目；此前仅有 [] 的组件 fixture 未覆盖此故障。

AC-ASSISTANT-LIST / COD-002/006、TST-001/002/005、G-WEB：新回归修复前 3 项失败，修复后客户端及列表/详情/分享相关 85 项通过，完整前端 701 项、web-typecheck、web-build、diff 检查通过。截图堆栈与线上字段已直接定位，未为已确认的空值原因增加无关假说。上线和保留记录核对结果待补充；不将 API 记录存在写成已完成真实账号浏览器列表验收。

修复源码 `ef9a92084251dc54018091594270a2527930e127` 已经 main_temp 集成、干净 archive 构建并上线。产品 `2026-11-03-ef9a92084251-arm64` 平台 Manifest 为 `sha256:cd4312eee91aca0b13e36f06c7e8f38264173a3bcb558e1ea7f36b0eea49ed1e`，amd64/线上固定引用为 `sha256:b509124a2e0662805f93e4d22e330a54f389595de96ea129a9e9fcf09ca10baa`。两个仓库四个架构版本先完成推送与核验，再更新 latest 和架构标签；产品 latest Index 为 `sha256:279a53afd30da24aea1f0b585d32e5fc3f43dd9a786ea99ddb9bbd2fc26bd62c`，Runtime 继续复用相同不可变平台镜像。

后端/部署/资源 Git diff 为空，两架构 API、Worker、MinIO 可执行文件 SHA-256 均与此前通过安装/重启验收的镜像相同，未重复未受影响的后端和模型检查。切换前活动执行为零，已生成 `/srv/agent-platform/backups/before-assistant-null-ef9a920` 私有冷备份并校验。部署后服务 healthy / RestartCount 0，两个原有助手的 ID 与 draft 状态保持相同，原始知识库字段仍为 null（在客户端规范化，未执行数据库修复或写入），五引擎与 20 条目录仍可用。公网前端 index 指纹 `7f44e0a40dd3e38b9ebdd99d26ad7d39d6dc45c69643c8b1e9ed396f40c5f956` 与本次通过回归测试的本地构建及两个镜像一致，维护标记已清除。验证采用真实所属账号 API 只读检查及同形 HTTP 响应的 Vue 渲染回归；未另行登录该账号的浏览器或修改其助手状态/内容，不把这些结果写成真实对话或发布验收。
### 2026-11-03：恢复 HTTPS 部署的扫码注册基础设施（已上线，公众号待管理员配置）

AC-REG-1：线上管理员 `GET /api/v1/admin/registration-methods` 可重复返回 `available=false`、微信/飞书均 version 0；数据库 registration_settings 为 0 行。只读身份检查证实 accounts 未装配 registration_broker、服务账号缺少 manage-identity-providers、admin-only 标记缺失。截图提示是部署装配遗漏，不是删除了外部身份或隐藏现有配置。

修复在一体化 HTTPS 启动 seam 生成独立 RSA 2048 PKCS8/随机 client secret，0600 持久化并重启复用，向既有 Broker 配置注入当前公开 Origin；复用扫码身份脚本的共享最小权限同步，不授予 manage-realm、不改其他资料校验或密码。Provider 开关仍默认关闭。HTTP localhost 不启用，原配置损坏拒绝替换，不将密钥写入环境、镜像层或日志。

AC-REG-2：新增配置测试先红（2 个缺失装配用例），随后覆盖密钥重启/Origin 变化复用、既有系统密钥不变、0600、HTTP 关闭、损坏拒绝、命令失败不泄漏和真实共享身份同步的幂等/权限 allowlist。`make deploy-test` 通过。关联 COD-003/004、TST-001/005 和 G-DEPLOY/G-SEC。真实供应商配置、回调握手及手机确认尚未验证，不能将基础设施恢复记为微信登录验收通过。

发布前 `make test`、`make build`、独立的 `python3 scripts/configure-registration-identity_test.py`、`git diff --check` 和 AGENTS/CLAUDE 对比通过。通用 Go 测试未配置 PostgreSQL DSN，Skip 不算数据库集成通过。镜像 Smoke 新增显式 `--https-broker`，用于实际启动/重启验证公钥和密钥文件稳定，HTTP loopback 运输不替代 TLS/手机验收；目标镜像执行结果待记录。

发布源 `dd94a667a691a18e196c30400c6addbc7c8b3073` 已推送并经 `main_temp` 的精确 Git Archive 构建。产品 `2026-11-03-dd94a667a691` / `latest` 多架构 Index 为 `sha256:971a3a51ad162a7e96d9f2be438eaba136d98bcc4ef3a875b11a20bd1fbf9ce6`，ARM64 Leaf `sha256:bdd1b2d991001d644bfbfc7be342cbc135fa27e33087bf22284e2ccd9280e211`，AMD64 Leaf `sha256:da43e845fc2be87aecc2a5c2e37c28337530c2ce2b04494613a552083fb5e4f1`。四个版本引用核对后更新两个仓库的 latest/arm64/amd64，并逐一核验平台集合和 Digest。Runtime 源码未变，复用已验证的 ARM64 `79f313b3…` / AMD64 `a3f1a38d…` 平台镜像，仅增加同版本标签；Runtime Index 仍为 `sha256:e87886e26da0f098c874f4ca1349b6a4f92ed484b5a7d87e7a6fa69c458ba3f8`，不声称重新构建了 Runtime。

两架构均执行真实 `smoke-unified-image.py --https-broker`，通过新卷初始化、匿名 401、公开 RSA JWKS、0600、独立非 root/只读/无网络 Runtime、保留数据重启和两类密钥文件稳定检查。补充组件测试用线上同形状的稀疏 JSON（两个方法仅有 provider/callback_url）经过真实 HTTP Client 渲染，确认微信/飞书两份表单、四个空 Secret 输入框均存在且无 Vue 错误；目标组件 4 项测试、`make web-typecheck`、`make web-build` 通过。产品前后端及 MinIO 二进制/入口均与上一版逐项 SHA-256 相同；前端入口仍为 `7f44e0a40dd3e38b9ebdd99d26ad7d39d6dc45c69643c8b1e9ed396f40c5f956`。

失败与恢复：初次 ARM 构建参数误填 Revision，主动取消，修正后核对实际镜像 Label，未发布错误标签；AMD 上传一次 Registry TLS 超时，保留原日志后重试成功。Mac 上首次 AMD 模拟 Smoke 容器以 1 退出且被默认清理，具体子进程原因无法从保留日志恢复，未将这次记作通过；随后保留诊断实例完整重跑，启动和重启均通过，最后仅删除该带测试标签的容器/卷。原失败日志保留为 `/tmp/registration-smoke-amd64-initial-failed.log`。实际 Linux 上本次受控切换首次即健康，没有用 Mac 结果替代生产结果。

线上先确认活动执行为 0 并开启维护，停止 app，建立 `/srv/agent-platform/backups/before-registration-dd94a66` 完整冷备份及 Compose/env 恢复点，再固定 AMD64 RepoDigest 启动；备份 SHA-256 校验通过，未演练数据恢复。真实 HTTPS 管理员登录和只读接口返回 registration available=true、五引擎可用、User/Admin 目录各 20；匿名管理员接口 401，公网微信 JWKS 200。Keycloak 服务账号窄权限与两个 admin-only marker 均实测为 true；registration_settings 仍为 0 行。新 Broker 文件权限 0600；完整 state.json 与冷备份逐字段相等，管理员凭据、公开 Origin 和 Runtime 设置保持原值，production/runsc 未降级，服务 healthy、restart=0、维护已解除。原站从服务器本地 SNI 检查为 200，本机直连其域名 TLS reset，未将后者记为外网可用，也未更改原站配置。

用户明确选择自行在后台填写公众号配置，不迁移其他部署的 Secret。当前已恢复“账号管理 → 注册方式”配置能力，两个供应商入口仍关闭；保存并开放后才会在登录页显示。没有填写虚构公众号凭据、调用真实供应商、发送消息或扫码，没有真实用户浏览器/手机登录验收；完整 Production Conformance、真实供应商配置/回调握手和手机首次/返回登录仍未验证。
