# 天眼查连接器验证 — 2026-10-02

## 包与上游

- source `tianyancha`，version `1.0.0`，MCP Streamable HTTP，官方端点 `https://mcp.tianyancha.com/mcp`，目录分类“行业数据”。
- 通过 `go -C backend run ./cmd/connector-package-validate ../outputs/tianyancha/tianyancha-1.0.0.zip` 实际调用当前 `connectorpackage.Parse`；规范化 SHA-256 `dea4568d1f79fd439507cef21049a7d74196875708d06164e1edd5e53bb0b081`，恰好一个 MCP manifest、一个 Skill，无 CLI bundle 或凭证。
- 官方 Protected Resource / Authorization Server metadata 返回 200；issuer `https://capi.tianyancha.com/oauth`，S256 PKCE、动态 public client 注册、authorization code、refresh token。本修订只请求 `mcp:tools.call`。
- `/mcp` 与 `/v1` 未授权 initialize 返回 401，包含 protected-resource metadata、所需 scope 与缺少 Authorization 的说明；这不是已授权工具发现通过。
- DCR 返回 201，接受实际回调 `https://47-237-108-63.sslip.io/api/v1/connectors/tianyancha/oauth/callback`，返回的 URI 与 public client auth `none` 匹配；未记录 client ID 或凭证。
- 发布前 Administrator Publication 与当前 Administrator 的 User Installation 目录各无天眼查条目。
- 官网 favicon 原始 32×32 像素转换为 PNG，包 SVG 与目录使用同一资产。

## 已执行检查

- `gofmt` 与 `git diff --check`。
- `go -C backend test ./internal/tianyanchamcp/... ./internal/service/workspace/... ./internal/connectorpackage/...`。
- `python3 -B -m unittest discover -s scripts/connectors/tianyancha -p 'test_*.py'`：5 个发布验证用例通过。
- `pnpm install --frozen-lockfile`；ExtensionManager 定向测试 102 个用例通过。
- 完整前端 `pnpm test`：46 个文件、451 个用例通过。
- 前端 `pnpm typecheck` 与 `pnpm build` 通过。
- `WORKSPACE_TEST_POSTGRES_DSN=<一次性 PostgreSQL 16 DSN> make test` 和 `make build` 通过；包含天眼查 Snapshot owner/授权 AAD/过期/Refresh Material 隔离数据库测试。
- 协议测试覆盖 PKCE、DCR、scope、回调 issuer/过期/篡改/重放、刷新、Provider 错误脱敏、重定向拒绝。平台仅精确 GET 回调免 Bearer 认证，POST、相邻路径和其他业务端点仍要求认证。

## 验证边界

真实天眼查账号授权、已授权 tools/list、企业业务 tools/call、真实账号刷新、Linux + gVisor 模型 Runtime 及完整 Production Conformance 尚未执行。公开文档、包验证、DCR 和浏览器入口不能替代这些证据。线上发布、目录及入口结果在实际完成后补充。
