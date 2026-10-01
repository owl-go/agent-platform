# 墨刀 CLI Connector Package

`modao` 0.1.1 是本仓库维护的 CLI 桥接器，通过[官方 Streamable HTTP MCP](https://modao.cc/hc/articles/470)连接墨刀 AI。官方当前推荐远程 HTTP，没有提供本指南所需的独立墨刀 CLI；本包不依赖旧版 `@modao-mcp/modao-proto-mcp`，也不把它称为官方 CLI。

官方指南允许 OAuth 与个人空间令牌，[官方接入页](https://modao.cc/feature/ai-mcp.html)使用覆盖所有列出客户端的令牌方案。本修订选择个人空间令牌，使用官方 `modao-token` Header。它不实现浏览器 OAuth。真实授权账号拥有操作结果和生成权益；官方未公布细粒度 scopes，因此 manifest 的 scopes 为空，不表示账号拥有任何额外权限。

## 构建与调用

CLI 没有 npm 依赖、安装脚本或本地凭证文件。构建器校验仓库中固定版本的 CLI SHA-256 和官方品牌资产 SHA-256，并生成确定性的 tar/ZIP。目标 Runtime 必须含 Node 24.15.0；传入平台已配置的真实 Registry RepoDigest。ZIP 包含 metadata、品牌 icon、CLI manifest、不可变 bundle 和 companion Skill。

```bash
python3 scripts/connectors/modao/build.py \
  --runtime-image '<configured-repository>@sha256:<64-lowercase-hex>' \
  --output /tmp/modao-0.1.1.zip
go -C backend run ./cmd/connector-package-validate /tmp/modao-0.1.1.zip
node scripts/connectors/modao/modao.mjs --help
```

本地先在终端环境中设置 `MODAO_TOKEN`，然后运行 `tools` 和 `schema generate html` 读取真实授权目录及参数格式，再运行 `account status`。业务命令仅接受 `--json '<对象>'` 或 `--stdin`，参数格式以当前服务端 Schema 为准。托管 broker 不转发 stdin，因此托管调用使用 `--json`，本地直接运行可使用 `--stdin`；本地直接运行不经过平台的批准 broker。托管命令必须通过 `agent-cli`，使用 [companion Skill](SKILL.md) 中的 capability、User 身份与高风险 target。五个生成/导入命令为高风险，账号/任务读取和三种诊断命令为低风险；所有请求固定访问 `modao.cc`，不接受 URL、Header、Token 参数或 raw MCP 调用。

平台使用已有 `connector_package` 认证 driver。所有者范围内的 Connect API 接收 `identity_ref: user`、空 scopes 及 `credentials_json` 中的单个 `modao_token` 字段，通过平台加密存储保存；单次业务进程收到 `CONNECTOR_CREDENTIALS_JSON`。提供平台 credentials JSON 时，不回退到本机环境中的另一个令牌。CLI `auth`/`status` 用 `get_account_status` 检查连接；没有 Token 的 status 是未连接。`unauth` 提示断开平台授权，不撤销墨刀令牌，也不清空平台凭证。墨刀账号中删除令牌才会撤销上游访问。

构建同时输出 `modao-0.1.1.source.zip`。source ZIP 的 npm 元数据、可执行文件、能力与资源限制和最终包一致；不可变 bundle 使用平台 `cli-connector-bundle` 构建器生成，本地与 Worker 可逐字节核对。进程限额为 64，以容纳 gVisor 与 Node 异步线程；目标 Runtime 中已验证异步线程可启动。

发布命令必须在授权部署主机使用受保护的配置，凭证只在进程内读取，不写入包或证据：

```bash
python3 scripts/connectors/modao/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package /tmp/modao-0.1.1.zip \
  --source /tmp/modao-0.1.1.source.zip \
  --evidence-directory /tmp/modao-publication-evidence
```

发布器创建专用临时 Definition，让 Worker 构建并记录 exact bundle × Runtime Conformance，然后暂存、激活正式 Revision，并软删除无用户使用的临时 Definition。重跑复用已验证修订。它不会安装到任意 User；Installation 必须在明确的目标账号范围内创建。平台代码按仓库规则先集成 `main_temp` 再部署。

已安装且未授权的墨刀卡片提供“连接”入口，弹窗包含一个密码类型的个人空间令牌字段和官方指南链接。成功清空表单，失败可重试，取消会清空令牌；会话选择器将未连接的墨刀导向该设置页面。表单和跳转已通过组件测试，实际部署与目录验收状态以发布证据为准。

## 线上发布证据（2026-10-01）

`main_temp` 集成提交 `1d9a637` 经完整部署脚本发布为 `modao-20261001`，API、Worker、Egress Controller 与 Web 健康检查通过，备份位于部署主机 `backups/pre-modao-20261001`。正式版本为 0.1.1：

- Revision：`c32ea922-896c-4a2f-809f-b30300bafd51`，Publication `available`。
- Installation：`5dd2c4f9-f2a3-418f-9767-b52f7d3e680c`，所有者 `platform-admin@agent-workspace.local`，状态 `active`、未授权。
- Bundle SHA-256：`fecf444238a9144daa83f6a8dfc152e0976078998a1e2e89a1934034b5534f5e`。
- Runtime：`sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b`，平台 Worker 真实 Conformance 通过并已入库。
- 临时 Definition：`322e9103-2254-474b-8aab-5c53cb181ff7`，无用户使用，已通过管理员 DELETE 软删除；重跑复用相同 Revision，没有新增 Definition。

管理员与 User 目录各有一个墨刀正式条目，Conformance 为 true。Playwright 实际检查桌面与 390×844 移动端：卡片和会话选择器均加载 158px 官方品牌资产；“连接”打开单个密码类型令牌输入与官方链接，取消后关闭，重新打开仍为空（输入清空另由组件 fixture 覆盖）；移动端文档宽度 390、表单边界 20–370，无横向溢出。会话选择器点击未授权墨刀能到达 `/resources?tab=connectors`。线上未提交任何真实或测试墨刀令牌；真实账号权益、生成和导入仍未验证。

0.1.1 本地 18 项协议测试、5 项构建测试、4 项发布生命周期测试通过；目标 Runtime 在 Linux + runsc、无外网、非 root、只读 Rootfs、512 MiB/1 CPU/64 进程下通过全部 18 项协议测试。当前最终 ZIP 已通过实际 Go Parse，完整 `make test`、`make build`、前端 46 文件/402 项测试、`make web-typecheck` 和生产构建通过。未运行完整模型 Runtime Production Conformance 或存储 Conformance；本次没有更改其契约。包、非敏感 API/构建证据位于被忽略的 `outputs/modao`，截图位于 `output/playwright/modao`。

## 验证

```bash
node --test scripts/connectors/modao/modao.test.mjs
python3 -m unittest discover -s scripts/connectors/modao -p 'test_*.py' -v
go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...
make test
make build
pnpm --dir frontend test src/components/ConnectorIcon.test.ts
make web-typecheck
make web-build
```

0.1.0 历史验证（2026-10-01）：本地 18 项 Node 测试与 5 项 Python 构建测试通过；Python 测试调用当前 `connectorpackage.Parse` 检查真实 ZIP，并启动包内可执行文件。目标包及完整 Go 测试/构建、前端图标测试/typecheck/生产构建通过。远程 Linux + `runsc` 使用已配置 Node 24.15.0 Runtime，在无外网、非 root、只读文件系统、资源限制下完成隔离构建、全部 18 项协议测试和仓库 `cliconnector.DockerConformance.Test`。该 suite 对 bundle `3f8bd824aa55fdba19b23bb6145d3fafe9e52c6124efea4762e2b49a14842a9a` × Runtime `sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b` 的启动验证通过，平台数据库中尚未记录它。临时远端源码、产物和验证程序已清理，未创建临时 Definition。非敏感证据与交付包位于被忽略的 `outputs/modao`。

协议 fixture 校验官方 Header、初始化通知、会话与协议版本、发现及实际 tool-call 路由；覆盖 JSON/SSE、UTF-8 分片、未知工具/命令拒绝、凭证反射脱敏、权限拒绝、重定向、超时和输出限额。fixture 是模拟 upstream。无凭证请求官方端点得到 401；真实已授权工具 Schema、个人空间、账号权益、生成与导入仍未验证。没有运行完整模型 Runtime Production Conformance、存储 Conformance 或业务 API 写入；本次未改动镜像、存储及其契约。

品牌图标来自[墨刀官方产品导航资产](https://images.modao.cc/images/images-2025/navbar/icon-navbar-product-proto@4.png)，包内 SVG 嵌入与前端 PNG 使用相同字节。
