# 天眼查 MCP Connector Package

`tianyancha` 1.0.0 通过官方托管的 HTTPS Streamable HTTP
`https://mcp.tianyancha.com/mcp` 查询企业数据，目录分类为“行业数据”。
官方也提供 `tyc-cli`，其本地配置与登录流程无需放入本 MCP 包。

```bash
python3 scripts/connectors/tianyancha/build.py --output outputs/tianyancha/tianyancha-1.0.0.zip
go -C backend run ./cmd/connector-package-validate ../outputs/tianyancha/tianyancha-1.0.0.zip
python3 -B -m unittest discover -s scripts/connectors/tianyancha -p 'test_*.py'
```

构建器打包固定文件清单、时间和权限，离线生成无凭证的 ZIP。
`connectorpackage.Parse` 是最终结构与规范化 SHA-256 的验证依据。
生产服务器请求 OAuth 注册实际返回 HTTP 419 `bannedLocation`：当前新加坡地区不受支持。目录明确标出授权阻塞，连接失败显示联系天眼查确认支持地区的提示。上游限制解除前不能完成授权或宣称业务可用。

发布器默认拒绝注册失败；只有 `--allow-region-blocked`、精确的官方地区错误及包内阻塞说明同时存在，才允许发布目录条目，并记录 `blocked_region`。此选项不改变网络路径或绕过授权。

发布器复用相同修订并检查 User 与 Administrator 目录唯一条目及品牌图标；
发布不自动授权、安装或升级其他 User 的 Installation。

## 上游与授权

2026-10-02 实际核对的官方资料：

- [官方 CLI 源码及 MCP 说明](https://github.com/tyc-tech/tyc-cli)
- [Protected Resource Metadata](https://mcp.tianyancha.com/.well-known/oauth-protected-resource/mcp)
- [Authorization Server Metadata](https://capi.tianyancha.com/.well-known/oauth-authorization-server/oauth)
- [官方品牌 favicon](https://www.tianyancha.com/favicon.ico)

Metadata 返回 200，issuer 为 `https://capi.tianyancha.com/oauth`，支持 S256
PKCE、authorization code、refresh token 和动态注册 public client。
注册、授权、Token 端点为 issuer 下的 `/register`、`/authorize`、`/token`。
资源为 `https://mcp.tianyancha.com/mcp`。本修订只请求 `mcp:tools.call`，
没有请求额外的 `mcp:quota.read`。未授权 MCP initialize 返回带发现信息的 401。
上游 DCR 接受实际平台 HTTPS 回调且返回匹配的 URI 与 `none` client auth。

平台的审核适配只匹配本 source、OAuth、固定官方端点、单一 MCP Egress
域名且无自定义 Header/Environment。平台 OIDC redirect URI 的 HTTPS origin
决定 `/api/v1/connectors/tianyancha/oauth/callback`；仅该精确 GET 路径免平台认证。
回调通过加密 state、owner/flow AAD 与 CAS 绑定单次授权；授权码单次消费。
缺失 `iss` 被允许，若存在则必须匹配 issuer。请求禁止 HTTP 重定向。

Access Token 以 `MCP_BEARER_TOKEN` 经现有 MCP 配置注入 Runtime；Refresh Token
单独加密并留在平台，过期、断开及 owner 不匹配拒绝新执行。
API 进程访问审核过的 OAuth 域名；模型 Runtime 的 Egress 仅允许 MCP 域名。
本包没有手动 Token 授权入口，也没有 CLI bundle 或构建暂存 Definition。

图标保留官网 favicon 的原始 32×32 像素，转换为 PNG，包内 SVG 与目录图片
使用同一品牌资产；它不表示天眼查对本包作出了认证。

## 发布与验证边界

```bash
python3 scripts/connectors/tianyancha/publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package /absolute/path/tianyancha-1.0.0.zip \
  --normalized-sha256 '<connectorpackage.Parse SHA-256>' \
  --evidence-directory /absolute/path/evidence
```

官方工具目录涵盖企业工商、股权、风险、知识产权、经营、历史和人员数据，
MCP tools/list 的 AI 入口与 CLI 内置业务清单不同。Skill 先发现实际工具与
Schema 再定位企业并查询所需能力；权限、额度与无结果分别处理。
真实账号的 tools/list、tools/call、Token refresh 和 Linux + gVisor 模型 Runtime
必须单独验证，不能从公开文档、401、DCR 或包验证推断通过。
实际执行记录见 `docs/evidence/agent-workspace/2026-10-02-tianyancha-connector.md`。
