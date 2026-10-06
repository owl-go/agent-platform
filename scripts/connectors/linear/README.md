# Linear MCP Connector Package

本目录构建 `linear` 1.0.0，接入 [Linear 官方 MCP](https://linear.app/docs/mcp) 的 HTTPS Streamable HTTP 端点。包包含 metadata、官方图标、单个 MCP manifest 和 companion Skill；不包含 CLI 或凭证。工具由当前已授权服务端发现，官方文档支持事项、项目与评论的查询、创建和更新；本仓库尚无真实 Linear 账号业务验收。

## 构建与安装

```bash
python3 scripts/connectors/linear/build.py --output /absolute/path/linear-1.0.0.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/linear-1.0.0.zip
```

构建无需网络和额外依赖，使用固定文件清单、时间和权限生成 ZIP。必须再执行当前 Go parser，普通 ZIP 检查不能代替平台验证。User 在连接器页面上传 ZIP 会创建私人 Installation；Administrator 暂存及发布 Revision 是另一条流程。发布前先查询目录中的 `linear` Publication 和当前账号的 Installation，复用或升级符合要求的修订。此修订已发布到平台目录，实际状态见下方发布证据。

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
  --config /srv/agent-workspace/config/platform.env \
  --package /absolute/path/linear-1.0.0.zip \
  --normalized-sha256 '<SHA-256 returned by connector-package-validate>' \
  --evidence-directory /absolute/path/linear-publication-evidence
```

## 2026-10-01 发布证据

通过 `main_temp` 的 Commit `9c21bc3` 部署 Release `linear-20261001T091000Z`，API、Worker、Egress Controller 和 Web 健康检查通过。[线上连接器目录](https://workspace.example.com/resources?tab=connectors) 中 `linear` 的 Publication 状态为 `available`，版本为 1，包版本为 `1.0.0`。

- Revision：`95e0a7bc-30ab-42f3-93f3-5fb81dc4e6f1`。
- 当前 Go parser 验证的 normalized package SHA-256：`677938b0e9d4a479d13e481ae011b784e4c4d3a25f317e75469d3aca9e9bbd39`。它不是 ZIP 文件字节的 SHA-256。
- Administrator 与 User 目录各只有一个正式条目；重复执行发布器复用了同一 Revision 与 Publication 版本。
- Linear `/register` 实际接受平台 HTTPS callback，空 callback 请求返回 400 并带 `no-store`、`no-referrer`。
- 仅在验收用 Platform Administrator 账号安装：Installation `14ca6cb6-9fd1-4e3f-96c6-2c7747747a39`，状态 `active`、`authorized=false`、Authorization 数量 0。没有安装或升级其他 User。
- 线上浏览器确认官方 PNG 品牌图标已加载、安装后“连接”可操作，并成功打开官方 MCP `/authorize`，随后跳转 `linear.app`。上游页面停留在加载状态，未完成真实账号授权。桌面与 390×844 移动端详情截图已保存，移动端页面宽度与 scrollWidth 均为 390。
- `make test`、`make build`、`make web-typecheck`、`make web-build` 通过；前端全量 426 项、Python 发布器 5 项通过；一次性 PostgreSQL 上的 Linear Snapshot 集成测试通过。部署脚本的一般镜像 smoke 与服务健康检查通过。

非敏感目录响应位于忽略目录 `outputs/linear/publication`，ZIP 位于 `outputs/linear/linear-1.0.0.zip`，截图位于 `output/playwright/linear`。临时平台凭证文件、浏览器会话和远程发布临时文件已清理。人工注入的短期平台登录态未附带续期能力，浏览器验收后期遇到登录态过期；这次检查不能作为正常 OIDC 登录／续期的验收。

仍未验证：真实 Linear 账号换码、刷新、工具发现或业务调用，以及 Linear MCP 在 Linux + gVisor 中的执行、完整 Production Conformance 和远端存储 Conformance。目录发布与浏览器入口检查不替代这些证据。
