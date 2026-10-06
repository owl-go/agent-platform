# AI-Hive MCP Connector Package

`ai-hive` 0.3.0 使用[官方 npm 包](https://www.npmjs.com/package/@infimind-next/ai-hive-mcp) `@infimind-next/ai-hive-mcp@0.3.0`，与用户提供的「MCP 接入」截图版本一致。上游要求 Node.js >= 20.19.0，默认 API 为 `https://ai-hive.iclip.cn/api`。采用截图中的当前账号 API Key 方式；WorkBuddy 的本地 OAuth 登录不属于本修订。密钥通过上游已支持的 `AI_HIVE_MCP_KEY` 环境变量传递。

ZIP 包含 `connector-meta.json`、`mcp.json`、[官网品牌图标](https://ai-hive.iclip.cn/brand/project-logo.svg)和 `skills/ai-hive/SKILL.md`。构建器校验 npm tarball SHA-512 与图标 SHA-256，不执行上游安装脚本；最终 ZIP 必须经当前仓库 `connectorpackage.Parse` 验证。npx 执行仍由上游版本声明解析依赖，不等于包含所有传递依赖的不可变 CLI bundle。

## 构建与协议检查

```bash
curl --fail --proto '=https' --tlsv1.2 \
  https://registry.npmjs.org/@infimind-next/ai-hive-mcp/-/ai-hive-mcp-0.3.0.tgz \
  --output /tmp/ai-hive-mcp-0.3.0.tgz
python3 scripts/connectors/ai-hive/build.py \
  --npm-tgz /tmp/ai-hive-mcp-0.3.0.tgz \
  --output outputs/ai-hive/ai-hive-0.3.0.zip
python3 -m unittest discover -s scripts/connectors/ai-hive -p 'test_*.py'
python3 scripts/connectors/ai-hive/smoke.py
go -C backend test ./internal/connectorpackage/... ./internal/data/workspace/runtimeexecutor/...
make test
make build
```

协议检查使用临时 HOME／npm cache、虚构 canary 和不可连接的本地 API 地址，仅发送 initialize、initialized、tools/list，核对 8 个工具与生成预检字段。它不会读取用户凭证或请求付费生成。临时目录退出即清理。

## 安装与授权边界

初版交付本地 ZIP；后续安装与平台发布使用受保护的部署账号配置。安装前先检查目标 User 的 `/api/v1/connectors` 和 `/api/v1/connectors/catalog`，Administrator 检查 `/api/v1/admin/connectors/publications`，已有匹配修订时优先安装／升级。需要平台安装时使用已有 ZIP 上传接口 `/api/v1/connectors/packages`；生成 ZIP 本身不创建 Installation。

包的 `auth_mode: "cli"` 表示平台的 provided-credentials 授权模式，连接器执行模式仍是 MCP。用户在蜂巢 AI 网页开启 MCP 并获取 API Key。平台 ConnectConnector 接口 `/api/v1/connectors/{installation_id}/authorization` 接收 `identity_ref: "user"`、空 scopes 和 bytes 类型的 `credentials_json`（JSON HTTP 中 base64 编码），凭证对象仅含 `MCP_BEARER_TOKEN`。平台加密保存并在本次执行中解析 `${MCP_BEARER_TOKEN}` 为 `AI_HIVE_MCP_KEY`；密钥不会进入包或参数。

该包需要本任务新增的 MCP 变量引用物化支持。旧平台把引用当作字面值时无法正常认证，必须先经 `main_temp` 集成并部署该支持。AI-Hive 卡片的“连接”入口使用密码类型 API Key 表单，并链接蜂巢 AI 网页；成功或取消时清空表单，失败时可重试。实际线上验收见后续证据。上架时还须检查正式条目唯一性、实际图标显示、可操作连接入口及目标 User／Administrator 目录。

Egress 清单只含已确认的 npm registry 和默认 API 域名。上游媒体上传依赖动态签名 PUT 地址，当前没有上传域名的真实环境证据；目标环境应按真实地址核验策略后再允许上传。工具发现不证明媒体上传、账号余额、付费生成或目标 Runtime RepoDigest Conformance 已通过。

## 本地验收证据（2026-10-01）

已执行最终 ZIP 的 `connectorpackage.Parse`、上游完整性拒绝测试、官方 0.3.0 MCP 握手和 8 个工具发现，以及 Runtime Executor／Connector Package 目标测试、`make test` 和 `make build`。全量测试命令通过不代表依赖外部环境的集成测试都执行过。凭证别名在 Claude、Codex、Hermes、OpenClaw 的配置中正确投影，缺失或空授权变量拒绝物化，不使用宿主凭证。原始 Secret 保留在精确字节脱敏集合中。该初版检查当时没有目标 Linux + runsc、真实账号或平台目录验收证据；后续发布检查见下文。

## 发布并安装

管理员发布器读取部署主机受保护的 env 文件完成平台 PKCE 登录，检查现有修订，暂存并发布经 Parse 验证的包，然后为该登录账号幂等安装／升级。它不会把同一 Authorization 共享给其他 User，也不会提交 AI-Hive API Key。

```bash
python3 scripts/connectors/ai-hive/publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package /tmp/ai-hive-0.3.0.zip \
  --normalized-sha256 '<current-connectorpackage.Parse-sha256>' \
  --evidence-directory /tmp/ai-hive-publication-evidence
```

发布后核对 Administrator／User 正式目录各只有一个 AI-Hive 条目、安装版本匹配、官网图标加载，以及未授权时可打开 API Key 表单。

## 线上发布证据（2026-10-02）

`main_temp` 集成提交 `696bf00` 经完整部署脚本发布为 `ai-hive-20261002`，API、Worker、Egress Controller、Web 和公网健康检查通过；部署备份位于 `backups/pre-ai-hive-20261002`。

- Publication：`available`，正式版本 0.3.0。
- Revision：`d5b19b31-5277-406f-a719-c035bb2b84b2`，规范化包 SHA-256 为 `9184a303e13e54c7013a290e73ca96989c64118b33748ae4d300b3bae1f68289`。
- Installation：`c910c163-4de3-4ecd-88fe-3e036d978621`，所有者 `platform-admin@agent-workspace.local`，状态 `active`、未授权。
- 重跑发布器复用了同一 Revision 和 Installation；Administrator／User 正式目录与该账号 Installation 各只有一个 AI-Hive 条目。

指定 Runtime `127.0.0.1:5000/agent-platform/runtime@sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b`（部署后 Digest 未变）在 Linux + runsc、非 root、只读 Rootfs、无外网、1 CPU／512 MiB／64 进程限制下实际通过官方 MCP 0.3.0 initialize 与 8 个工具发现。该检查使用虚构 canary 和不可连接的本地 API 地址，业务调用数为 0；依赖准备禁用 npm 安装脚本。临时依赖目录与远端发布 ZIP／证据目录均已清理。

本地及发布门禁的完整 `make test`、`make build`、`make web-typecheck`、生产 Web 构建通过；前端完整 46 个测试文件／438 项测试通过。目标连接表单与图标的相邻测试通过。Playwright 实际检查 1200px 桌面和 390×844 移动端：只有一个 AI-Hive 卡片，官方 SVG 图标加载成功（26px 原生尺寸）；未授权时“连接”打开一个空的密码类型 API Key 字段和官方获取说明链接；移动端文档宽度 390，表单左右边界为 20／370，没有横向溢出。成功清空、失败重试、取消清空与先升级再连接由组件 fixture 覆盖。

非敏感 API 与 Linux 协议证据保存在忽略目录 `outputs/ai-hive`，桌面／移动端截图位于 `output/playwright/ai-hive`。线上没有提交任何真实或测试 AI-Hive API Key。真实账号授权、余额／模型查询、媒体上传及付费图片／视频生成仍未验证；没有执行完整模型 Runtime Production Conformance 或存储 Conformance。上述 Linux MCP 协议 smoke 不能替代这些业务验收。
