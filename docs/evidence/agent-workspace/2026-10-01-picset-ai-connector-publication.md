# Picset AI 连接器发布证据 — 2026-10-01

## 修订与部署

- 官方依据：https://picsetai.cn/developer-api。页面提供 REST API 与个人 Agent Secret Key；未提供可直接安装的官方 CLI/MCP 配置。本包为仓库自有 REST CLI 桥接器。
- Source：`picset-ai`，包版本 `0.1.0`，15 个 API 操作和 3 个只读诊断能力，User 身份；不填写未经官方公布的 scopes。
- 功能 Commit：`17313a4`，开发分支 `codex/picset-ai-connector` 已推送。
- 集成分支 `main_temp`，部署源 Commit：`ab469d9`。正式 `make deploy` 未跳过门禁；Release ID：`picset-ai-20261001T083300Z`。
- 源目录：`/opt/agent-platform/src.release-picset-ai-20261001T083300Z`，Web：`/opt/agent-platform/web/releases/picset-ai-20261001T083300Z`，备份：`/opt/agent-platform/backups/pre-picset-ai-20261001T083300Z`。
- Publication：`available`，version `1`，活动 Revision：`f27d3edc-233a-4009-b8a5-b47f97f7cf04`。
- 原始交付 ZIP SHA-256：`e97a863ac135f175c42a07ad538c5f36d4d1eb448474eda30962722ffaa9a7f4`。
- 平台规范化 Package SHA-256：`0a9798d2ada16d636dbde4ab7f57ac728cd68fb3514ce4bdb7c58468ac9a8463`。
- Bundle SHA-256：`07f71a73965b15397465cc3430c76184c69249a33326c569b7af63d789f81474`。
- Runtime：`127.0.0.1:5000/agent-platform/runtime@sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b`，Node `24.15.0`。

## 已执行检查

- `node --test scripts/connectors/picset-ai/picset-ai.test.mjs`：7 个聚焦用例通过，包括 15 个操作逐一核对方法、路由、Bearer Header、请求体与付费幂等键；拒绝未审核路径/字段、错误素材 scene、凭证参数、无幂等键付费请求、超长输入/输出，验证错误分类、重定向/超时不自动重试和精确凭证脱敏。
- `python3 -m unittest discover -s scripts/connectors/picset-ai`：2 个构建/供应策略测试通过；共享 publisher 的 4 个生命周期用例通过。
- `go -C backend test ./internal/connectorpackage/...`、`make test`、`make build`、`make web-typecheck`、`make web-build`、`git diff --check` 均通过。开发分支完整前端 Vitest 为 46 个文件、420 个用例；集成部署门禁为 46 个文件、421 个用例。
- 最终 ZIP 经真实 `connectorpackage.Parse`（`go -C backend run ./cmd/connector-package-validate`）校验，输出 1 个 Skill、18 个 capability 和上述规范化 Package/Bundle SHA-256。
- 平台隔离 Builder 从 source ZIP 生成同一 Bundle SHA-256；Linux + `runsc` Docker Conformance 对该 exact bundle 与当前 Runtime Digest 运行无网络 `--help`，记录通过。正式 Revision 的 `conformance_available=true`，Runtime Digests 含上述 Digest。
- 无凭证访问官方固定 GET 路径返回 HTTP 401、稳定 `UNAUTHORIZED`，确认 API 可达并拒绝无 Secret Key 请求；未提供假 Key。
- Administrator `/api/v1/admin/connectors/publications` 与 User `/api/v1/connectors/catalog` 各恰有 1 个 `picset-ai` 正式条目，均为 available、Conformance true，图标为品牌 PNG 投影。
- 临时构建 Definition `595369be-4482-4aed-b93f-db1b12963113` 无 User 使用关系，已通过管理员 DELETE API 软删除；目录不再含 Picset AI package build 条目。
- Playwright 真实浏览器检查资源中心：设计创作分类有唯一 Picset AI 卡片，官方品牌图标实际加载。为本轮验收临时安装后，详情显示版本 0.1.0 和 Runtime 证据；连接按钮打开 `type=password`、`name=picset_api_key` 的空输入框和官方密钥管理链接；取消成功。没有提交真实或测试 Key。
- 临时 Installation 的状态为 active、version 1、无 Authorization，图标与 Revision 投影一致；验收后经 owner-scoped DELETE API 卸载，正式 Publication 保持可安装。远端本轮临时 ZIP/source/evidence 目录已清理。
- 部署脚本完成数据库/配置备份校验、镜像 smoke、服务健康、Migration、OIDC 和公网 Web 产物检查。部署时的既有 bundle 复验打印 `verified 0 active CLI Connector bundles`；Picset 的 exact Conformance 在部署后单独完成，不能将该 0 条记录当作 Picset 验证。

本机交付物和截图位于 `/Users/frank/.codex/artifacts/picset-ai-20261001/`：`picset-ai-0.1.0.zip`、`.source.zip`、`catalog-card.png`、`connection-form.png` 与无凭证发布响应记录。截图仅包含本连接器卡片/表单。

## 验证边界

官方 OSS 上传文档只有 Bucket 域名占位示例。本修订 Egress 仅允许 `picsetai.cn`，不执行 OSS PUT、任意 URL 或结果下载；上传 URL 分配、审核和图片业务操作可用于受控外部环境已经上传的素材，纯文本 canvas-image 无需素材。未核实实际 OSS 上传域名，也未验证真实上传。所有 POST 为高风险并复用已有 broker 一次性批准边界，付费提交须在首次调用前保存幂等键。

真实 Picset Key、所属账号的请求查询、付费生成及结果交付尚未验证。生命周期 status 只表示密钥格式已配置，显式返回 `upstream_verified=false`。Mock 协议测试和无网络 --help Conformance 不证明第三方业务成功。本轮未执行完整 `make sandbox-conformance` 或 `make production-conformance`，未开启 Runtime Capability。
