# 小鹅通连接器

`xiaoe` 0.1.0 使用小鹅通真实 HTTPS Streamable HTTP 服务 `https://agent.xiaoe-tech.com/mcp`，通过浏览器登录、店铺选择和 OAuth + PKCE 连接。用户要求发布到平台目录；当前回调域名注册受上游限制，用户已明确同意先发布并标注授权阻塞。

## 选型与上游证据

2026-10-02 查询 npm registry 确认 `xiaoe-cloud-cli@0.5.37` 存在，安装入口的 SHA-512 为 `MI1QW+jqnlTLrMwfxZWN0NsVe+TPMdIUv4Uuqx8SoWSOUyKMM5sPiOhMI+Lo+W3fg9vfUffwwlxnMHkVb+zM/A==`。包的 postinstall 下载平台原生二进制，不能把 npm wrapper 本身当成不可变跨架构 bundle。实际 macOS ARM64 CLI 的 `--help`、`auth --help`、`auth online --help`、`schema --help` 确认有课程、订单等 API 命令及浏览器授权。该 CLI 未进入最终包，未据此宣称生产运行通过。

[上游连接器配置的公开镜像](https://github.com/ahang1598/doubao-workbuddy-qwenwork-skills/tree/main/workbuddy/connectors/marketplace/connectors/xiaoe-cloud-cli)指出远程 MCP 端点。随后直接查询小鹅通服务验证：

- 未授权 initialize 返回 401，`WWW-Authenticate` 指向 `/.well-known/oauth-protected-resource/mcp`，scope 为 `mcp`。
- [Resource metadata](https://agent.xiaoe-tech.com/.well-known/oauth-protected-resource/mcp)确认唯一 resource 与 Bearer Header；[Authorization metadata](https://agent.xiaoe-tech.com/.well-known/oauth-authorization-server)确认 `/oauth/authorize`、`/oauth/register`、`/oauth/token`、`/oauth/revoke`，支持 S256、authorization_code、refresh_token、mcp 和 offline_access。
- OAuth 客户端注册对示例 HTTPS 域名和 localhost 回调返回 201。真实平台 `https://workspace.example.com/api/v1/connectors/xiaoe/oauth/callback` 在本机及部署主机均返回 HTTP 566 和上游安全拦截页面。未改用 localhost，也未把手动 Token 作为替代授权方案。
- 品牌 SVG 直接来自[小鹅通授权服务资产](https://agent.xiaoe-tech.com/oauth/assets/xiaoe-icon.svg)，最终包与前端使用相同字节。

配套 Skill 的操作路由来自上游使用说明，实际授权工具目录、账号和店铺范围、读写调用仍未验证。以实时 MCP Schema 为准；OAuth 的 `mcp` scope 不是细粒度读写 scope。

## 构建、验证与发布

```bash
python3 scripts/connectors/xiaoe/build.py --output outputs/xiaoe/xiaoe-0.1.0.zip
python3 -m unittest discover -s scripts/connectors/xiaoe -p 'test_*.py' -v
go -C backend test ./internal/xiaoemcp/... ./internal/connectorpackage/... ./internal/service/workspace/... ./internal/data/workspace/gormrepo/... ./internal/data/workspace/runtimeexecutor/...
make test
make build
make web-typecheck
make web-build
```

构建器实际调用 `connectorpackage.Parse`，旁边生成 receipt，区分上传文件 SHA-256 和平台规范化 ZIP 的 SHA-256。发布器复用 exact Revision，按乐观锁激活 Publication，检查 User 目录只有一个正式条目且有品牌投影；不创建临时 CLI Definition，不自动安装到任意用户。配置中的管理员凭证只在内存读取，不进入 ZIP 或证据。

平台代码先提交并集成 `main_temp`，再经仓库完整发布门禁部署；在部署主机运行：

```bash
python3 scripts/connectors/xiaoe/publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package /tmp/xiaoe-publication/xiaoe-0.1.0.zip \
  --evidence /tmp/xiaoe-publication/publication.json
```

发布前检查已有 Publication 和 Installation；已有符合包校验和授权接入的修订优先复用。发布修订不会替现有用户升级安装或完成授权。

## 授权与执行边界

只有 `xiaoe` source、OAuth 模式、精确 MCP URL、单一 Egress 域名且无自定义 Header/环境变量的管理员发布包可使用该 provider adapter。私人包不能借 source 调用平台登录适配器。动态注册 public client，授权与交换均绑定 resource、回调和 PKCE；密封 state 绑定 owner/flow，回调保存加密 code，所有者认证完成请求一次性消费后交换。错误、取消、过期和重放 fail closed。

refresh token 保存在独立平台加密字段，MCP Runtime 只收到短期 Bearer Token；快照使用授权记录的实际 AAD。过期授权可在连接器详情刷新。历史 MCP 快照没有 package 字段时保持原行为；新托管 MCP 快照冻结 package Object Key 与 SHA-256，验证 ZIP 并只读挂载其 Skill，再调用 MCP 工具。这个包不包含 CLI，CLI bundle × Runtime Conformance 不适用；完整模型 Production Conformance、真实账号授权及店铺操作未验证。

安装后「连接」尝试被 HTTP 566 拒绝时，页面明确说明回调域名被拦截及联系小鹅通放行／配置正式域名的处理办法。上游解除限制后再完成真实授权及业务验收。

## 本次发布结果

已在 `main_temp` 集成并部署 `xiaoe-20261002T1915`，目录 Revision 为 `996c6c5b-7ba9-4a71-a7c2-60b0b42ab308`，状态 available。包规范化 SHA-256 为 `cfc5ca7f6f6670b41416dc594b1d308fa3fdf2a5507e4386c65071f1ac07c543`。真实页面已验收品牌图标、连接入口和域名拦截提示；诊断安装没有凭证，已清理。真实授权和业务调用受上游回调限制，仍未验证。详见[发布证据](../../../docs/evidence/agent-workspace/2026-10-02-xiaoe-connector.md)。

部署 source 上传后修正了发布辅助脚本的 User catalog 路径，实际发布与幂等重跑使用修正后的独立副本；后续发布应从最新 `main_temp` 获取脚本，不能使用这次旧部署 source 内的发布辅助脚本。API/Worker/前端已经包含本次全部功能。
