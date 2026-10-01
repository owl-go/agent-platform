# Linear MCP Connector Package

本目录构建 `linear` 1.0.0，接入 [Linear 官方 MCP](https://linear.app/docs/mcp) 的 HTTPS Streamable HTTP 端点。包包含 metadata、官方图标、单个 MCP manifest 和 companion Skill；不包含 CLI 或凭证。工具由当前已授权服务端发现，官方文档支持事项、项目与评论的查询、创建和更新；本仓库尚无真实 Linear 账号业务验收。

## 构建与安装

```bash
python3 scripts/connectors/linear/build.py --output /absolute/path/linear-1.0.0.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/linear-1.0.0.zip
```

构建无需网络和额外依赖，使用固定文件清单、时间和权限生成 ZIP。必须再执行当前 Go parser，普通 ZIP 检查不能代替平台验证。User 在连接器页面上传 ZIP 会创建私人 Installation；Administrator 暂存及发布 Revision 是另一条流程。发布前先查询目录中的 `linear` Publication 和当前账号的 Installation，复用或升级符合要求的修订。用户已选择平台发布；实际 Publication、目录与 Installation 状态以完成后的证据为准。

## 浏览器授权

平台必须先部署本任务中的 OAuth 适配代码；旧平台仅接受通用 MCP 手动凭证，无法完成本修订的浏览器流程。平台以 OIDC `authentication.redirect_uri` 的 HTTPS origin 构造 `/api/v1/connectors/linear/oauth/callback`，反向代理必须将该路径转发至 API。只有这一精确 GET 路径允许上游回调，授权开始、完成、刷新等 API 仍要求平台账号认证。

安装后点击“连接”，平台在官方 `/register` 注册 public client，以 PKCE S256 跳转 `/authorize`，在 callback 校验 issuer 与加密 state。完成 API 在单次消费授权状态后调用 `/token`。默认请求官方 `read write` scopes；API 显式请求 `read` 可以获得只读授权，本包端点仍是官方默认读写端点。只读专用 `/mcp/readonly` 尚未纳入本适配器。

访问令牌以 `MCP_BEARER_TOKEN` 接入现有 Runtime MCP Bearer Header 或临时环境变量。Refresh Token 单独加密保存，使用注册时的 Client ID 刷新，不进入 Runtime。Snapshot 使用该 Authorization 自己的 AAD，同时保留历史通用 MCP AAD 回退。过期授权无法开始新的调用，User 可在连接器设置刷新或重新连接。没有真实账号验证，不能声称已验证 Workspace 切换、外部账号展示或具体工具 Schema。

Runtime Egress 仅需 `mcp.linear.app`；OAuth 注册及换码由平台 API 访问同一 host，浏览器登录跳转由 Linear 管理。平台固定请求官方端点，拒绝 OAuth HTTP 重定向，不让包提供任意 OAuth 地址。超时为 manifest 中的 60 秒。未执行 Linux + gVisor 模型 Runtime、完整 Production Conformance、存储 Conformance 或业务写入。

## 品牌来源

图标来自 [Linear 品牌资产](https://linear.app/brand) 的 `Linear-Brand-Assets.zip?v=3`：包内 `icon.svg` 原样使用 `linear-icon.svg`（SHA-256 `9326c9bb47752fe51157e5e024e0282875e9163ee22825151f80999037760d78`），平台目录原样使用 `logo-dark.png`。资产仅标识此连接器的外部服务，不表示 Linear 对本包的官方认证。目录通过现有品牌图标投影返回 PNG data URL；没有修改通用 SVG 上传边界。

## 验证

```bash
go -C backend test ./internal/linearmcp/... ./internal/service/workspace/... ./internal/connectorpackage/... ./internal/data/workspace/gormrepo/... ./internal/data/workspace/runtimeexecutor/...
WORKSPACE_TEST_POSTGRES_DSN='<disposable-test-database-admin-DSN>' go -C backend test ./internal/data/workspace/gormrepo -run TestLinearMCPSnapshot -v
pnpm --dir frontend test src/components/ExtensionManager.test.ts src/pages/SkillsConnectorsPage.test.ts
make test
make build
make web-typecheck
make web-build
```

协议 fixture 覆盖 public client 注册、PKCE、回调 issuer、scope 限制、超时状态、授权码重放、刷新、上游错误与重定向脱敏。平台测试覆盖加密 owner/flow 绑定、精确 GET 回调免认证、Token/Refresh Token 分离和各 Runtime 配置中的 Bearer 注入；PostgreSQL 测试覆盖 Snapshot AAD、所有权与过期授权拒绝。前端组件测试覆盖连接入口、版本升级先于授权、浏览器跳转与独立返回页完成。

2026-10-01 上游无凭证检查：OAuth metadata 与 protected-resource metadata 返回 200，声明 S256、authorization code、refresh token 与 dynamic registration；官方 MCP initialize 返回 401 及 Bearer challenge。这些检查不证明真实注册、账号授权或 tool-call 成功。非敏感探测证据和最终 ZIP 位于被忽略的 `outputs/linear`。

发布器在授权部署主机读取受保护的环境配置，仅在进程中处理平台凭证。它验证当前部署的精确回调入口、Linear 接受真实远程回调的 public client 注册、Go parser 给出的 normalized package SHA-256，并复用符合要求的 Revision，再检查 Administrator 与 User 目录各只有一个正式 Publication。它不修改其他来源，不自动升级现有 User Installation，不创建 CLI staging Definition。

```bash
python3 scripts/connectors/linear/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package /absolute/path/linear-1.0.0.zip \
  --normalized-sha256 '<SHA-256 returned by connector-package-validate>' \
  --evidence-directory /absolute/path/linear-publication-evidence
```
