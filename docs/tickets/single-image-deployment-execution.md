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
