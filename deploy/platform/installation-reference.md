# 全新服务器安装（运维）

一般安装请直接运行 `make install`，见[首次安装](installation.md)。本页只保留外部数据库、对象存储和自定义基础设施的手工参考；尚未通过全新服务器端到端安装验收。

按下面顺序阅读，需要接入可选能力时再查看对应技术文档：

1. [服务器要求](#环境与服务器规划)
2. [配置文件](#配置文件与填写顺序)
3. [身份与执行环境](#首次安装身份文件与-sandbox)
4. [启动和登录](#启动登录与模型配置)

## 环境与服务器规划

| 用途 | 要求及来源 |
| --- | --- |
| 开发 / 发布工作站 | Git、Go 1.25 或兼容工具链（`backend/go.mod`）、Node.js 24、pnpm 11.19.0（CI 配置）、Make、Python 3、OpenSSH、rsync；构建镜像还需 Docker。 |
| 完整执行服务器 | Linux，Docker Engine + Compose v2、可用的 `runsc` Runtime、iptables、root 部署权限；部署脚本还使用 Python 3、curl、sha256sum、tar。优先选择可运行已验证镜像的 linux/amd64；其他架构须另验固定 CLI、基础镜像和 gVisor，不能继承 amd64 证据。 |
| 持久依赖 | PostgreSQL、MinIO 或阿里云 OSS、Keycloak 及其 PostgreSQL。随附 Compose 用私有网络和持久卷部署这些服务；镜像版本以 [`deploy/platform/.env.example`](../../deploy/platform/.env.example) 的固定 Digest 为准。 |
| 网络 | 域名解析到入口服务器；公开 TCP 80/443，UDP 443 用于 HTTP/3；SSH 仅向部署来源开放。数据库 5432、MinIO 9000/9001、API 8080、Worker 管理端口 9090 和 Docker Socket 不公开。允许必要的 HTTPS 模型、Registry、OIDC、Connector 及 DNS 出站，执行容器使用受管 Egress。 |

仓库没有经过压测的最低服务器配置。小规模试用可先规划 **8 vCPU、16 GiB RAM、100 GiB 可用 SSD**，这只是同机 API/Worker、数据库、Keycloak、MinIO 和少量任务的容量估算，不是已验证最低值。当前模型 Runtime 单容器限制为 2 CPU、4 GiB 内存、512 PID、2 GiB tmpfs；CLI Connector 默认另有 1 CPU、1 GiB 内存与 256 MiB tmpfs 限制。tmpfs、多个执行阶段、数据库、镜像构建和对象文件都会增加宿主压力。Workspace 单个配额最多 1 GiB，镜像、对象存储和备份需要另留空间。

生产容量应按实际同时活跃的 Session / Run / Connector 容器、模型响应时长、队列、数据库连接和存储增长测量。单 Workflow 的 Run 按队列串行，不代表整台 Worker 只占一个容器。Runtime 镜像构建可移到工作站或 CI。RAGFlow、Embedding 和本地模型不包含在上面的估算中；它们需要独立容量规划。跨主机扩容与外部依赖配置见[服务架构](../../docs/technical/service-architecture.md)，随附 Compose 不是任意多节点的一键安装器。

## 获取代码与开发检查

```bash
git clone https://github.com/owl-go/agent-platform.git
cd agent-platform
pnpm --dir frontend install --frozen-lockfile
go -C backend mod download
make test
make build
make web-typecheck
make web-build
```

前端开发：复制 [`frontend/.env.example`](../../frontend/.env.example) 为忽略的 `frontend/.env.local`，填写可访问的 OIDC 配置及 `VITE_API_PROXY_TARGET`，执行 `pnpm --dir frontend dev --host 127.0.0.1`。API 与 Worker 接受 `-config /absolute/path/platform.yaml`：

```bash
go -C backend run ./cmd/api -config /absolute/path/platform.yaml
go -C backend run ./cmd/worker -config /absolute/path/platform.yaml
```

需先准备下面的数据库、账号、对象存储和 YAML 环境变量。启动命令不会自动供应这些依赖。macOS 可用于开发、单元测试和 Web 构建，不能替代 Linux + `runsc` 执行验收。修改 Protobuf 后使用 `make generate`；生成工具要求见 [`backend/Makefile`](../../backend/Makefile)。

## 配置文件与填写顺序

API/Worker 读取同一份严格 YAML，`${NAME}` 从进程环境展开。未知字段、缺失变量、非法 URL、缺失凭据或不安全 Sandbox 配置会拒绝启动。私有 env、账号导入、SSH 文件和部署 YAML 放在 Git 外，部署时不要开启 `set -x` 或输出展开后的 Compose 配置。

1. 选择域名、服务器、安装根目录与 Registry，准备 TLS / OIDC 入口。
2. 创建独立数据库、对象存储、Keycloak 和平台加密凭据。
3. 配置 Keycloak realm、Web client、后台 service account 和 Bootstrap Administrator。
4. 配置持久目录、Egress 与固定 Runtime Digest，保持引擎及可选能力关闭直到验收。
5. 启动依赖及 API，验证后启动 Worker；管理员在产品内添加模型连接并保存平台默认执行配置。

以服务器上的 `/srv/agent-workspace` 为通用安装根目录。在服务器放置集成后的源码目录，进入该源码根目录，再准备外部文件：

```bash
export INSTALL_ROOT=/srv/agent-workspace
sudo install -d -m 0700 "$INSTALL_ROOT/config" "$INSTALL_ROOT/backups" "$INSTALL_ROOT/run-credentials"
sudo cp deploy/platform/.env.example "$INSTALL_ROOT/config/platform.env"
sudo cp deploy/platform/config/platform.https.yaml "$INSTALL_ROOT/config/platform.https.yaml"
sudo cp deploy/platform/config/keycloak-realm.json "$INSTALL_ROOT/config/keycloak-realm.json"
# 使用编辑器修改上述三个副本；不要修改成已包含真实 Secret 的仓库文件。
sudo chmod 600 "$INSTALL_ROOT/config/platform.env"
```

env 文件是部署者控制的 Shell/Compose 文件；填写值时正确引用特殊字符，不放命令替换。建议密码使用独立的高熵 URL-safe 值（例如 `openssl rand -hex 32`），避免数据库 URI 中未经编码的特殊字符。`DATA_ENCRYPTION_KEY` 使用 `openssl rand -base64 32` 生成的 32 字节 Key，保存在密钥管理/受限文件中并备份；不能在升级时随意替换，否则已加密凭据无法解密。

| 文件 / 配置项 | 必填与填写方法 |
| --- | --- |
| `platform.env`：`PUBLIC_HOST`、`ACME_EMAIL` | HTTPS 部署必填。示例 `workspace.example.com`、`operations@example.com`；使用自己可解析的域名和运维邮箱。 |
| `POSTGRES_*`、`KEYCLOAK_DB*`、`*_IMAGE` | Compose 必填独立密码与 RepoDigest。产品与身份数据库分离；镜像必须是 `repository@sha256:<64位小写摘要>`，不能用 `latest`。 |
| `MINIO_ROOT_USER/PASSWORD/BUCKET` | 随附 MinIO Compose 必填；初始化服务建私有 Bucket。外部对象存储填写 YAML 的 `object_store`，并自行保证私有 Bucket。 |
| 四个 `VITE_OIDC_*` | Web 构建必填：authority=`https://workspace.example.com/identity/realms/agent-platform`，client ID=`agent-platform-web`，redirect=`https://workspace.example.com/auth/callback`，logout redirect=`https://workspace.example.com`。构建后改变这些值需要重新构建 Web。 |
| `OIDC_ISSUER/AUDIENCE` | API 必填，与 authority 完全一致；audience=`agent-platform-api`，由 Web client 的 audience mapper 加入 Token。 |
| `KEYCLOAK_BASE_URL/REALM/SERVICE_CLIENT_ID/SERVICE_CLIENT_SECRET` | 账号管理必填：HTTPS base=`https://workspace.example.com/identity`，realm=`agent-platform`，service client=`agent-workspace-admin`，Secret 与私有 realm 导入/Keycloak 控制台一致。 |
| `BOOTSTRAP_ADMIN_SUBJECT/EMAIL` | 必填，与 realm 内初始 `platform-admin` 用户的稳定 ID / 邮箱一致；YAML 的 `accounts.bootstrap_username` 也须匹配。`KEYCLOAK_ADMIN_*` 是身份系统的管理账号，`PLATFORM_ADMIN_PASSWORD` 是产品初始管理员密码，两者使用不同凭据。 |
| `DATA_ENCRYPTION_KEY` | 必填，Base64 编码的 32 字节 Key；API 与 Worker 使用同一个值。 |
| `WORKSPACE_ROOT/WORKSPACE_KNOWN_HOSTS` | 必填绝对路径，例如 `/srv/agent-workspace/workspaces`、`/srv/agent-workspace/config/known_hosts`；known_hosts 使用管理员核验过的 Git SSH 主机公钥，不能关闭 Host Key 校验。 |
| `CREDENTIAL_TEMP_ROOT/SANDBOX_RESOLVER_FILE` | 执行必填，例如 `/srv/agent-workspace/run-credentials`、`/srv/agent-workspace/config/sandbox-resolv.conf`；Credential 目录在 Worker 与宿主同路径挂载。 |
| `RUNTIME_IMAGE`、YAML `worker.runtimes` | 固定 Registry RepoDigest、CLI 版本与镜像一致。**新安装将每个 `available`、`native_resume` 设为 `false`**；只有对应 Digest 的真实验收通过后才分别开启。解析器支持字段不能当作验收。 |
| `AGENT_EGRESS_NETWORK/SUBNET/DNS_SERVERS` 与 YAML `sandbox` | 三处保持一致：env、YAML、宿主 Egress 配置。示例受管私网 `172.30.0.0/24` 必须避开已有网络，Resolver 必须是明确允许的公网 DNS。Controller Socket 保持现有容器协议路径。 |
| `OIDC_REALM_FILE/KEYCLOAK_THEME_ROOT/WEB_RELEASE_ROOT` | HTTPS overlay 必填外部绝对路径：realm 文件、构建后的主题、包含 `current` 静态发布目录的 Web 根目录。 |
| YAML `message_channels` | 可选，默认关闭。启用时 API/Worker 均开启，callback 为平台 HTTPS origin，并按供应商配置真实应用/授权；见[渠道规格](../../docs/technical/workflow-message-channels.md)。 |
| YAML `worker.cli_builder` | 执行能力可选、默认关闭；启用前填 Builder RepoDigest、受管 Egress 和超时，并取得 bundle × Runtime Digest 验收。日常 `make deploy` 保留已有 Builder 与 Runtime Digest。只有基础设施升级脚本需要可写 Registry。 |
| YAML `retrieval` 与 `RAGFLOW_*` | 可选，默认不配置；RAGFlow 接入见下文。`MODEL_RELAY_UPSTREAM` 也可选，仅已有模型网关才配置，普通供应商 HTTPS API 不需要自建 Relay。 |

修改 `platform.https.yaml` 的 `database.dsn`、`object_store` 和 `accounts` 可接外部服务。随附 Compose 会构造 `DATABASE_URL` 并启动本机依赖；外部 PostgreSQL 用私有网络、合适 TLS 的 DSN 修改 YAML，并通过部署自有 Compose override 移除不需要的依赖。不能只改一个 env 值就声称本机数据库已变成远端。阿里云 OSS 字段参考 [`platform.aliyun-oss.example.yaml`](../../deploy/platform/config/platform.aliyun-oss.example.yaml)，Provider 选择集中在 `object_store.provider`；切换后也要调整 Compose 依赖与 env 映射。详见[对象存储规格](../../docs/technical/object-storage.md)。

## 首次安装：身份、文件与 Sandbox

私有 `keycloak-realm.json` 是模板副本：修改初始用户的 `id`/邮箱，使其与 `BOOTSTRAP_ADMIN_*` 一致，保留 `${PLATFORM_ADMIN_PASSWORD}`、`${KEYCLOAK_SERVICE_CLIENT_SECRET}` 环境引用。将 Web client 的 Redirect URI 收紧为实际 `/auth/callback`，Web Origin / post logout URI 与上述值一致，保持 Authorization Code + PKCE、关闭 Direct Access Grants。模板不会给后台 service account 自动授予账号管理权限。

首次 Keycloak 导入后，在目标 realm 的 Clients → `agent-workspace-admin` → Service account roles 中配置允许用户创建、禁用、密码重置及读取 groups/members 的权限。传统 `realm-management` 角色可从 `manage-users`、`view-users`、`query-users`、`query-groups` 配置起，按部署的权限模式验证这些实际接口；不要直接授予跨 realm 超级管理员。操作参考 [Keycloak 官方服务账号与管理权限说明](https://www.keycloak.org/docs/26.8.0/server_admin/)。已有 realm 不会因为重新 `--import-realm` 自动覆盖配置，更新需在身份系统中明确执行。

构建主题并配置文件权限（在服务器源码根目录执行）：

```bash
sudo python3 scripts/build-identity-theme.py "$INSTALL_ROOT/identity-themes/releases/initial"
sudo ln -s releases/initial "$INSTALL_ROOT/identity-themes/current"
sudo install -d -m 0700 -o 65532 -g 65532 "$INSTALL_ROOT/workspaces"
# known_hosts 必须事先写入核验过的主机 Key；即使不使用 SSH Git 也准备常规文件供只读挂载。
sudo touch "$INSTALL_ROOT/config/known_hosts"
sudo chown 65532:65532 "$INSTALL_ROOT/config/platform.https.yaml"
sudo chmod 400 "$INSTALL_ROOT/config/platform.https.yaml"
sudo chown 1000:0 "$INSTALL_ROOT/config/keycloak-realm.json"
sudo chmod 400 "$INSTALL_ROOT/config/keycloak-realm.json"
sudo chmod 644 "$INSTALL_ROOT/config/known_hosts"
```

安装并向 Docker 注册 `runsc`，参考 [gVisor 官方 Docker 安装步骤](https://gvisor.dev/docs/user_guide/install/)。CLI broker 需要 `runsc` 的 `--host-uds=open` 配置，不能放宽为 `create`/`all`。确认 `docker info --format '{{json .Runtimes}}'` 含 `runsc`。模型 Runtime 保持非 root、只读 Rootfs、全 Capability 移除、资源限制与受控挂载。执行 overlay 中 Worker 为目录归属操作使用 root 和有限 Capability，并接宿主 Docker Socket；不要将它与模型 Runtime 权限混为一谈。

配置宿主 Egress（root 操作；网络、DNS 与 YAML 一致）：

```bash
sudo env AGENT_EGRESS_NETWORK=agent-public-egress \
  AGENT_EGRESS_SUBNET=172.30.0.0/24 \
  AGENT_DNS_SERVERS='223.5.5.5 1.1.1.1' \
  AGENT_RESOLVER_CONFIG_FILE="$INSTALL_ROOT/config/sandbox-resolv.conf" \
  bash deploy/sandbox/configure-public-egress.sh
```

该脚本安装限制性网络规则及 Resolver 文件，应配置为随 Docker 启动恢复；systemd 示例见 [`agent-platform-egress.service`](../../deploy/sandbox/agent-platform-egress.service)，修改其中示例安装路径和 env 文件后安装。仅在确需 Runtime 访问同机公开 HTTPS 入口时配置 `AGENT_ALLOWED_HOST_HTTPS_IPS`；不能允许整个宿主网络。Egress Controller 使用执行 overlay 的宿主网络和 `NET_ADMIN`，Worker 只使用窄 Unix Socket 协议。详细边界见[Sandbox Runner](../../docs/technical/sandbox-runner.md)。

构建/推送 Runtime 到自己的 Registry，获取远端 RepoDigest 填到 env；本地 Image ID 不可代替：

```bash
RUNTIME_IMAGE_REGISTRY=registry.example.com/agent-workspace make runtime-images
docker push registry.example.com/agent-workspace/runtime:1.0.0
docker push registry.example.com/agent-workspace/cli-builder:1.0.0
docker image inspect --format '{{json .RepoDigests}}' registry.example.com/agent-workspace/runtime:1.0.0
```

`make runtime-images` 的元数据默认写 `build/runtime-images`，不提交生成目录。Linux Worker 上先按[Production Conformance](../../docs/technical/production-conformance.md)准备测试仓库、模型凭据、对象存储、恶意 Egress 测试端点和证据目录，再执行 `make production-conformance-preflight`，之后才运行适用的 `make runtime-image-smoke`、`make sandbox-conformance` / `make production-conformance`。这些命令不是无凭据的快速启动检查。一个共享 Digest 不表示五种引擎、MCP、Native Resume 或 CLI Connector 都已通过；没有证据的能力继续关闭。固定 CLI 与镜像构建说明见[Runtime Images](../../docs/technical/runtime-images.md)。

## 启动、登录与模型配置

在发布工作站设置四个与服务器一致的 `VITE_OIDC_*`（见上表），先构建并上传 Web，服务器上的 `WEB_RELEASE_ROOT` 也填写同一路径：

```bash
WEB_DEPLOY_HOST=deploy@example.com \
WEB_RELEASE_ROOT=/srv/agent-workspace/web \
make web-deploy
```

在服务器源码根目录，使用三份 Compose 配置。下面的函数只在当前 Shell 定义，不会打印 Secret：

```bash
export INSTALL_ROOT=/srv/agent-workspace
export PLATFORM_CONFIG_FILE="$INSTALL_ROOT/config/platform.https.yaml"
platform_compose() {
  sudo env PLATFORM_CONFIG_FILE="$PLATFORM_CONFIG_FILE" docker compose \
    --env-file "$INSTALL_ROOT/config/platform.env" \
    -f deploy/platform/compose.yaml \
    -f deploy/platform/compose.https.yaml \
    -f deploy/platform/compose.execution.yaml "$@"
}
platform_compose config --quiet
platform_compose up -d postgres minio minio-init identity-db identity
# 首次启动先建立 OIDC/TLS 入口，避免 API 等待身份系统与入口相互等待。
platform_compose up -d --no-deps caddy
curl --fail https://workspace.example.com/identity/realms/agent-platform/.well-known/openid-configuration
# 在 Keycloak 中补齐上述 service account 权限后启动控制面；API 应用追加式 Migration。
platform_compose up -d --build api
platform_compose exec -T api wget -qO- http://127.0.0.1:8080/readyz
platform_compose up -d --build egress-controller worker
platform_compose ps
curl --fail https://workspace.example.com/api/healthz
curl --fail https://workspace.example.com/api/readyz
```

等待身份 discovery 可用再继续；服务仍在初始化时重试健康探测，不跳过校验。访问 `https://workspace.example.com`，用私有导入配置的 `platform-admin` 初始密码登录。API Bootstrap 使用稳定 Subject 确认不可删除的初始 Administrator，不能只按邮箱授予权限。Administrator 在产品中管理 User 与同步 Identity Group；这不授予访问 User 私有对话的权限。

首次启动自动初始化 **8 个 Expert、12 个 Skill、20 个 Connector Package**，不需要手工新建定义。资源与当前平台的原版定义一致，完整 Skill 附件、CLI Bundle 和 Expert 指导内容随服务二进制分发；三个 Create Skill/Expert/Connector 包继续使用仓库内维护的原版。默认 Expert/Skill 原版不可编辑或删除；需要调整时创建另名自定义资源。重复启动不增加副本，受管理的原版按发布版本更新，已有同名管理员资源及私人资源不被覆盖。

20 个包中 **9 个 MCP、11 个 CLI**；MCP 定义仍需本安装完成配置/测试和用户授权，CLI 默认禁用。旧部署的账号、授权、API Key、用户安装和 Conformance 证据不会随资源复制。CLI 管理员须在当前 Linux + runsc Worker 对准确 Bundle SHA-256 × Runtime RepoDigest 完成验证后再发布；沿用各 [`scripts/connectors`](../../scripts/connectors) 中的构建/发布流程，不能手工改数据库状态跳过证据。资源清单、版本规则、现有资源保护与失败恢复见[默认资源分发](../../docs/technical/default-resources.md)。

Administrator 添加 Model Provider Connection：填写供应商类型、官方/受控 HTTPS Endpoint、写保护 API Key，加载模型目录并验证连接。选择经过验收、协议兼容的 Runtime Engine / Provider Model，直接保存为 Platform Execution Default；供应商可连通不等于引擎能执行。为 User 配置 Daily Credit Allocation 和必要的 Model Credit Rate 后，普通用户登录即可继承平台默认，或在 Personal Settings 选择允许的个人配置，再创建 Session / Workflow。Credits 是产品使用单位，不是供应商金额或财务对账。

知识检索是可选项：在共享 YAML 增加[知识库配置中的 `retrieval` block](../../docs/technical/knowledge-base-rag.md)，填写 `RAGFLOW_ENDPOINT/API_KEY/DEPLOYMENT_ID/EMBEDDING_MODEL`，同时供 API 与 Worker 使用。Endpoint 必须是无 path/query/用户信息的 HTTPS origin（本地测试可用 loopback HTTP），部署身份在同一持久数据安装中保持稳定；默认未配置时不提供检索。先验证指定 RAGFlow / Embedding 镜像与完整上传、解析、Ready、权限及召回契约，再允许依赖该知识的 Workflow 或 Smart Assistant。随附 [RAGFlow 本地开发示例](../../deploy/ragflow-local/README.md)不是生产高可用部署，独立存储、备份、网络和资源需另行规划。图片模型及 Prompt Optimization 在管理界面独立配置，不随文字模型连接自动启用。
