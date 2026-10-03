# Workflow Message Channels 部署验证 — 2026-10-03

## 发布标识

- 功能分支：`codex/workflow-message-channels`；功能提交：`ba9c1e4`、`3137c5f`、`c1293ef`。
- 集成分支：`main_temp`；实际部署源 Commit：`0bf38108f3a0affae47c4967177075156a973784`。
- 发布 ID：`workflow-channels-20261003-1`；部署约于 10:29 UTC 完成。
- 公开入口：`https://47-237-108-63.sslip.io`。
- 源目录：`/opt/agent-platform/src.release-workflow-channels-20261003-1`。
- Web 目录：`/opt/agent-platform/web/releases/workflow-channels-20261003-1`。
- 发布前备份：`/opt/agent-platform/backups/pre-workflow-channels-20261003-1`。
- 原源目录：`src.release-assistant-visitor-cookie-20261003-1`；原 Web：`assistant-scroll-20261003-1`。

## 集成与部署门禁

合并保留 `main_temp` 既有 RAGFlow 配置校验、知识库检索与 ingestion Worker、Pixso/天眼查/Linear/小鹅 OAuth Callback 和身份界面语言行为，同时接入渠道配置与收发循环。四个冲突文件经过人工合并；`make generate` 重新生成合并后的 Proto/OpenAPI/TypeScript/Wire 契约。

实际通过：

```bash
make generate
go -C backend test ./internal/platformconfig/... ./internal/wiring/workspaceworker/... ./internal/service/workspace/...
# WORKSPACE_TEST_POSTGRES_DSN 指向临时 postgres:17-alpine 实例：
make test
make build
pnpm --dir frontend test
make web-typecheck
make web-build
git diff --check
PLATFORM_RELEASE_ID=workflow-channels-20261003-1 make deploy
```

前端为 52 个文件、532 项测试。正式部署未设置 `SKIP_DEPLOY_GATES`，再次完成 Go 测试/构建、前端测试/类型检查，并按真实公开 OIDC 配置构建 Web；两轮 Go 全量测试均连接真实临时 PostgreSQL，渠道事务与完整 Migration 链没有因缺少数据库而 Skip。首次测试误用 key/value DSN，被需要 URL DSN 的测试 helper 拒绝；改用 URL DSN 后全量通过，没有修改测试绕过失败。

业务库与身份库 `pg_dump -Fc` 备份通过 `pg_restore -l`；数据库、配置和前一版源/Web 指针通过 `SHA256SUMS`。统一 Runtime 与 CLI Builder 镜像 smoke 通过。候选 Runtime 的 CLI Connector 复验正常退出，实际为 `verified 0 active CLI Connector bundles`，不代表新增 bundle 验收。API、Worker、Egress Controller 与 Caddy 已切换本次版本，最新 Migration 检查、OIDC discovery 和 HTTP→HTTPS 跳转通过。

## 平台配置与渠道循环

发布后备份当前配置到发布备份目录的 `channel-activation-config.yaml`，开启共同的 API/Worker 配置，并重新创建两个服务：

```yaml
message_channels:
  enabled: true
  callback_base_url: "https://47-237-108-63.sslip.io"
  max_connections: 64
  approved_endpoints: []
```

API 和 Worker 容器内检查确认配置一致。其余限制采用已校验默认值，既有 Data Encryption Key 保持不变。未新增供应商凭证、发送者、群或私网地址。

Worker `/metrics` 实际返回以下四个循环的 started=1、fatal=0：`message-channel-inbox`、`message-channel-delivery`、`message-channel-connections`、`message-channel-maintenance`。重新创建后 API、Worker 与 Egress Controller 均 healthy，检查服务日志的 panic/fatal/error 级记录数均为 0。

## 迁移、产物与公网验证

只读数据库查询确认新增 Migration 各有记录，Checksum 与集成源一致：

| Migration | SHA-256 |
|---|---|
| `000066_workflow_message_channels.sql` | `af8d28040cc9653f2581ec15e71b38b16ec6c0bd143555c93a94fa944ec30dea` |
| `000067_message_channel_transports.sql` | `25020473b8ccfcffdc884292269fed8fac62d8646c16a151c9dea75396deb221` |

`workflow_message_channels_provider_check` 包含 Telegram、Discord、Slack、DingTalk、Feishu、Matrix、WhatsApp、Signal、WeCom、WeChat、QQ Bot、BlueBubbles 和 Yuanbao 的全部 13 种 provider。Inbox、Delivery 与 Receive Cursor 表存在；未删除渠道配置数量为 0。未查询私有凭证或内容。

本地集成源与远端源的五个文件 SHA-256 一致：运输注册表、Worker 装配、Workspace Proto、渠道设置组件和生成 TypeScript 契约。构建产物、远端 current 与公网下载的 `index.html` SHA-256 一致：`75586d9627f5c9f2d4fa4eaa485087d146822843b02dada95913805002930889`。

本机公网实际检查：

- Web `/`、`/api/healthz`、`/api/readyz` 返回 200，健康与就绪分别为 `ok`、`ready`。
- 匿名访问渠道管理接口返回 401。
- Telegram/Slack/WhatsApp/QQ Bot 的有效 UUID 回调路由，在使用不存在的探测渠道 UUID 时均返回 503 `callback_rejected`；这是路由已接入且查找失败的结果，不代表供应商签名验收。
- WhatsApp 无效 GET challenge 返回 401；Telegram 非 UUID Callback Path 返回 401，没有扩大 OIDC 豁免范围。

服务镜像 ID：

- API：`sha256:889698d27ca74595674a303885e7ff05c07b228da5302fcc048c130456ef8e78`。
- Worker：`sha256:82550fda89d741c1ab615b0b16af924d6a8c22a72f562d348e81dad2d8d50c5e`。
- Egress Controller：`sha256:ccd474838cc671d6afd20f1dc107ac711adf0f9e3c2666bf607446ce030dbd70`。

本机检查日志：`/tmp/agent-platform-wmc-deploy.log`、`/tmp/agent-platform-wmc-activation.log`、`/tmp/agent-platform-wmc-remote-evidence.log`、`/tmp/agent-platform-wmc-public-evidence.log`、`/tmp/agent-platform-wmc-deploy-production-preflight.log`。部署后验证结果另保存到服务器 `/opt/agent-platform/evidence/workflow-channels-20261003-1`。

## 验证边界

本记录证明代码发布、平台配置开启、迁移、渠道循环、健康、接口认证边界与 Web 产物一致性。供应商账号仍需 owner 配置、允许受众和双向验证；Matrix、Signal、BlueBubbles 还需 Administrator 批准精确 HTTPS Endpoint，Signal/macOS Bridge 未由本轮安装。

目标 Linux 的 `make production-conformance-preflight` 实际失败：缺少专用 Git Fixture、Work/Evidence Root、Sandbox 网络探测、固定 Conformance Runtime Image、五个 Runtime 的测试 Model/Credential Directory 以及专用存储 Conformance 配置。未执行完整 Production Conformance、Linux Sandbox Conformance、远端 MinIO/OSS 集成或真实 IM→模型 Runtime→原聊天连续问答，也未完成认证浏览器的桌面/移动操作验收。本轮没有新增 Runtime Capability，不以镜像 smoke、服务健康或 mock 声称这些边界已经验收。
