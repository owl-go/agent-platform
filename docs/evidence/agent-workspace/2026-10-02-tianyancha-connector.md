# 天眼查连接器验证 — 2026-10-02

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 包与上游

- source `tianyancha`，version `1.0.0`，MCP Streamable HTTP，官方端点 `https://mcp.tianyancha.com/mcp`，目录分类“行业数据”。
- 通过 `go -C backend run ./cmd/connector-package-validate ../outputs/tianyancha/tianyancha-1.0.0.zip` 实际调用当前 `connectorpackage.Parse`；规范化 SHA-256 `8e1b4b5d2eb1a7a9ab914a85820e51e9162742a069d3d148c770b3701eeaabec`，恰好一个 MCP manifest、一个 Skill，无 CLI bundle 或凭证。
- 官方 Protected Resource / Authorization Server metadata 返回 200；issuer `https://capi.tianyancha.com/oauth`，S256 PKCE、动态 public client 注册、authorization code、refresh token。本修订只请求 `mcp:tools.call`。
- `/mcp` 与 `/v1` 未授权 initialize 返回 401，包含 protected-resource metadata、所需 scope 与缺少 Authorization 的说明；这不是已授权工具发现通过。
- 本机 DCR 返回 201，接受实际回调 `https://workspace.example.com/api/v1/connectors/tianyancha/oauth/callback`，返回的 URI 与 public client auth `none` 匹配；未记录 client ID 或凭证。
- 发布前 Administrator Publication 与当前 Administrator 的 User Installation 目录各无天眼查条目。
- 官网 favicon 原始 32×32 像素转换为 PNG，包 SVG 与目录使用同一资产。

## 已执行检查

- `gofmt` 与 `git diff --check`。
- `go -C backend test ./internal/tianyanchamcp/... ./internal/service/workspace/... ./internal/connectorpackage/...`。
- `python3 -B -m unittest discover -s scripts/connectors/tianyancha -p 'test_*.py'`：6 个发布验证用例通过。
- `pnpm install --frozen-lockfile`；ExtensionManager 定向测试 103 个用例通过。
- 完整前端 `pnpm test`：46 个文件、452 个用例通过。
- 前端 `pnpm typecheck` 与 `pnpm build` 通过。
- `WORKSPACE_TEST_POSTGRES_DSN=<一次性 PostgreSQL 16 DSN> make test` 和 `make build` 通过；包含天眼查 Snapshot owner/授权 AAD/过期/Refresh Material 隔离数据库测试。
- 协议测试覆盖 PKCE、DCR、scope、回调 issuer/过期/篡改/重放、刷新、Provider 错误脱敏、重定向拒绝。平台仅精确 GET 回调免 Bearer 认证，POST、相邻路径和其他业务端点仍要求认证。

## 验证边界

真实天眼查账号授权、已授权 tools/list、企业业务 tools/call、真实账号刷新、Linux + gVisor 模型 Runtime 及完整 Production Conformance 尚未执行。公开文档、包验证、DCR 和浏览器入口不能替代这些证据。线上目录与错误入口验收如下；地区限制仍未解除。

## 部署侧授权阻塞

第一次部署 `tianyancha-20261002T1330` 已通过备份、Runtime smoke、服务健康与 Web 检查。远端发布前 DCR 检查返回 HTTP 419、errorCode 301000、message `bannedLocation`，官方明确当前服务器所在地区不支持访问。三种 User-Agent 均返回相同错误；不将该请求记为注册成功。最终包描述明确披露授权阻塞；连接器可在目录创建，实际账号授权仍受上游地区限制。

地区错误处理追加后重新执行目标 Go、完整 `make test`、`make build`（本次未配置数据库 DSN，数据库验证沿用此前已实际执行的一次性 PostgreSQL 全门禁）、完整前端测试和 Web typecheck/build。新增测试验证地区错误分类、Provider 载荷不泄露、显式披露发布选项及前端中文错误提示。

## 最终发布与目录验收

- 功能提交 `cfeb153` 与地区限制提交 `7b058df` 已推送到 `codex/tianyancha-connector`。通过 `main_temp` 集成，最终发布源为 `3d4b228`，保留了同时到达的 Moka 更新。
- 最终发布 `tianyancha-region-20261002T1335` 完成；复用此前执行门禁（`SKIP_DEPLOY_GATES=1`），合并后再次执行完整 `make test`、`make build`、Web typecheck 与完整前端测试：46 文件 / 462 用例通过，生产 Web 构建由发布流程完成。备份校验、Runtime/CLI Builder smoke、服务健康与部署源检查通过。
- 备份 `/srv/agent-workspace/backups/pre-tianyancha-region-20261002T1335`；部署源 `/srv/agent-workspace/src.release-tianyancha-region-20261002T1335`。
- 最终 Publication available，Revision `a57eb19f-8734-4600-8002-7df05c527b5b`，规范化 SHA-256 为本文包验证栏中的最终摘要。发布器用明确披露选项记录 `callback_registration=blocked_region`、`account_verification=not_run`；并未记录 OAuth 注册成功。再次发布复用同一修订。
- User 和 Administrator 目录各仅一个正式天眼查条目，品牌 PNG 正常投影。Playwright 浏览器实际显示“行业数据”、天眼查品牌图标、版本 1.0.0、安装包校验通过及地区阻塞说明。
- 在管理员自己的 User 视图临时安装，未授权时“连接”入口可操作。点击后服务端真实地区限制映射为 `tianyancha_region_blocked`，页面显示联系天眼查确认受支持部署地区的中文提示；没有手动 Token 表单，也没有已授权工具执行。
- 截图 `output/playwright/tianyancha-details.png` 与 `tianyancha-region-blocked.png` 已实际查看。验证 Installation 未授权、没有任何 Authorization 后通过当前 version 清理。没有给其他 User 安装、授权或升级。
- 最终 ZIP 与非敏感发布 API 证据保留于忽略目录 `outputs/tianyancha`。一次性 PostgreSQL 已移除；临时浏览器状态、短期登录文件和远端暂存 ZIP/证据已清理，无 CLI build Definition。
