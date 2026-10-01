# 连接器使用指引部署验证 — 2026-10-01

## 发布标识

- 集成分支：`main_temp`，部署源 Commit：`154e8394122ca3db2bede654797fbd87544ea1f6`。
- 功能 Commit：`0218cb9`；合并保留当前平台名称/描述、卡片布局、Run Approval identity、Notion/钉钉项目浏览器授权和墨刀/企业微信 Token 连接入口。
- 发布 ID：`connector-guides-20261001T062000Z`。
- 源目录：`/opt/agent-platform/src.release-connector-guides-20261001T062000Z`。
- Web 目录：`/opt/agent-platform/web/releases/connector-guides-20261001T062000Z`。
- 发布前备份：`/opt/agent-platform/backups/pre-connector-guides-20261001T062000Z`。
- 前一版本：`modao-20261001`。
- 公网入口：`https://47-237-108-63.sslip.io`。

## 已执行的检查

- `make generate` 成功，合并后的 Protobuf/OpenAPI/TypeScript 契约已重新生成。
- `go -C backend test ./internal/service/workspace/...` 通过。
- 相关前端 Vitest：4 个文件、154 个用例通过，包含指引点击后安装并跳转、连接器选择、待授权草稿、未发送断言、当前安装版本示例、以及详情连接动作仍打开墨刀 Token 表单的回归测试。
- `make test`、`make build`、`make web-typecheck`、`make web-build` 和 `git diff --check` 通过。
- 正式 `make deploy` 再次执行完整本地门禁，未设置 `SKIP_DEPLOY_GATES`；完整前端 Vitest 为 46 个文件、409 个用例通过。
- 业务库和身份库备份通过 `pg_restore -l`；业务库、身份库、配置及前一版源/Web 指针均通过 `SHA256SUMS` 校验。
- Linux 上统一 Runtime 和 CLI Builder 镜像 smoke 检查通过。部署中的候选 Runtime 连接器复验入口正常退出，日志为 `verified 0 active CLI Connector bundles for configured Runtime`；本轮没有新增 bundle 复验证据。
- API、Worker 和 Egress Controller 从本次源目录启动且健康，Caddy 运行。部署脚本验证最新 Migration 恰有一条记录、OIDC discovery、HTTP 到 HTTPS 跳转，以及所检查服务日志没有 panic/fatal/error 级记录。
- 发布后从本机访问公网 `/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`；公开 Web 正常返回。
- 线上源中的 Proto、连接器响应映射、详情、资源目录、输入框和 Session 页面共六个文件 SHA-256 与本地集成工作区一致。
- 本地产物、远端 Web 当前目录和公网下载的 `index.html` SHA-256 一致：`58d5475517d7c0498ecdc579ff0ff7133b1e7e0580404c5c4177b7c93c2859d6`。
- 只读查询确认六个可用 Connector Publication 的当前 Metadata 均已有中英文示例：钉钉各 2 条、飞书各 4 条、墨刀各 2 条、Notion 各 3 条、钉钉项目各 2 条、企业微信各 3 条。该检查不读取 User 私有授权或凭证。

## 验证边界

本记录证明集成门禁、备份、部署源、服务健康和 Web 产物一致性。使用指引选择与填入但不发送的行为由本地组件/集成测试验证；未在发布后使用真实企业账号完成浏览器点击链路或外部连接器调用，也未运行完整 Production Conformance。此次发布没有改变 Runtime Capability、Connector bundle 或账号授权契约。
