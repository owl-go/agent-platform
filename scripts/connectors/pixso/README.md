# Pixso MCP Connector Package

`pixso` 1.0.0 使用官方公有云 HTTPS Streamable HTTP 服务 `https://pixso.net/mcp`，以各 User 自己的 Pixso 账号执行。包含 metadata、官方品牌图标、一个 MCP manifest 和 companion Skill，无 CLI、Token 或其他凭证。

```bash
python3 scripts/connectors/pixso/build.py --output /absolute/path/pixso-1.0.0.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/pixso-1.0.0.zip
```

构建器只打包固定文件清单，使用固定时间和权限，可离线复现。平台解析器是 ZIP 验证依据。发布前查询现有 Publication 和目标 User Installation，复用相同修订；发布不自动安装或升级 User 的既有 Installation。

## OAuth 与执行

官方 OAuth metadata（2026-10-01 核对）声明 issuer `https://pixso.net`、S256 PKCE、public client 动态注册、authorization code 和 refresh token，scope 为 `mcp:connect`。注册、授权和 Token 端点分别是 `/api/user/pixso/oauth2/register`、`/api/user/pixso/oauth2/authorize`、`/api/user/pixso/oauth2/token`。

平台复用现有浏览器 OAuth 生命周期，新增审核过的 Pixso adapter。它只匹配 `source=pixso`、`auth_mode=oauth`、固定官方 URL、单域名 Egress 且无自定义 Header/Environment 的 MCP 修订，避免上传包覆盖凭证传输。平台 OIDC redirect URI 的 HTTPS origin 决定 `/api/v1/connectors/pixso/oauth/callback`；只有精确 GET 回调免平台账号认证。安装后“连接”启动浏览器授权，回调用加密 state 绑定 owner/flow，并用 CAS 拒绝重放。上游未声明强制回传 `iss`；允许省略，若存在则必须精确匹配 Pixso issuer。授权码在 owner-authenticated 完成 API 中单次消费，失败后需重连。

Access Token 仅以 `MCP_BEARER_TOKEN` 注入现有 Runtime MCP 配置；Refresh Token 单独加密保存在平台，不交给 Runtime。刷新保留注册 Client ID 和 `mcp:connect` scope。过期授权拒绝执行，User 可以刷新或重连。API 访问固定官方 OAuth 端点并拒绝重定向；包内 Egress 仅允许 `pixso.net`，未覆盖素材下载到其他域名。写操作遵守现有平台策略，Skill 不能扩展能力或替代批准。

官方工具表支持设计结构、素材、代码生成和云端编辑；Skill 只列 Remote MCP 范围，参数由授权后服务的实际 Schema 决定。本包尚无真实账号的 tools/list 或 tools/call 证据，不能把文档覆盖说成业务验收通过。私有部署不使用此公有云包。

## 品牌与验证

图标来自 Pixso 官网 favicon `https://pixso.net/assets/media/favicon2-94712c33544afd45.ico`，取原始 32×32 图像无缩放转成 PNG，供包内 SVG 和平台目录 PNG data URL 使用。此资产标识外部服务，不表示 Pixso 官方认证此包。

```bash
python3 -B -m unittest discover -s scripts/connectors/pixso -p 'test_*.py'
go -C backend test ./internal/pixsomcp/... ./internal/service/workspace/... ./internal/connectorpackage/... ./internal/data/workspace/gormrepo/... ./internal/data/workspace/runtimeexecutor/...
pnpm --dir frontend test src/components/ExtensionManager.test.ts
make test
make build
make web-typecheck
make web-build
```

协议测试覆盖 DCR、PKCE、scope、回调篡改/issuer/过期/重放、刷新、上游错误脱敏和重定向拒绝。平台测试覆盖回调认证边界、凭证拆分、规范端点匹配和 Snapshot AAD/owner/expiry（数据库测试需要 PostgreSQL 环境）。发布器先验证已部署回调与上游接受真实 HTTPS callback，再暂存精确 parser SHA-256 的修订，检查 User 和 Administrator 目录各只有一个正式条目及品牌图标。

```bash
python3 scripts/connectors/pixso/publish.py --config /opt/agent-platform/config/platform.env \
  --package /absolute/path/pixso-1.0.0.zip --normalized-sha256 '<parser SHA-256>' \
  --evidence-directory /absolute/path/pixso-publication-evidence
```

执行过的验证和发布状态记录在 `docs/evidence/agent-workspace/2026-10-01-pixso-connector-publication.md`。Linux + gVisor 模型 Runtime、完整 Production Conformance、真实 Pixso 授权及业务读写必须分别报告，不能从包验证或目录可见推断。
