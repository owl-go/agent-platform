# 可灵 AI MCP Connector

本包使用[可灵官方接入指南](https://klingai.com/app/mcp/guide)中的中国区 HTTPS Streamable HTTP MCP：`https://klingai.com/mcp`。MCP 与 CLI 互斥，本修订不安装 CLI，也不接入第三方可灵代理。

`source: kling-ai`、版本 `0.1.0`、执行身份为每个 User 独立授权的可灵账号。品牌图片来自官网 `https://klingai.com/logo-180x180.png`，SVG 内嵌原始 PNG。包包含 metadata、该图标、恰好一个 `mcp.json` 和一个 Skill。官方列出 16 个工具；Skill 说明身份/模型发现、生成、查询、动作和主体管理及上传限制。实际可调用工具与参数仍由官方工具发现返回。

资源中心的连接器市场与已安装目录均将本连接器归入“设计创作”。

## 构建与发布

```bash
python3 scripts/connectors/kling-ai/build.py
python3 scripts/connectors/kling-ai/publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package /path/to/kling-ai-0.1.0.zip \
  --evidence-directory /path/to/evidence
```

Builder 真正执行当前仓库的 `connectorpackage.Parse`，再保存规范化 ZIP 与 `package-validation.json`；临时 Go Validator 会自动清理。生成物位于被 Git 忽略的 `outputs/connectors/kling-ai/`。Publisher 默认只暂存不可变 Revision；`--activate` 在实际回调域名 DCR 通过后激活 Publication，并验证 User 目录只包含该服务的一个条目。发布服务代码遵守 `main_temp` 集成规则；ZIP 生成和 Revision 暂存不等于连接器已发布或所有用户已安装。

## 浏览器 OAuth

平台内置 `klingmcp` Driver，仅在 source、OAuth 模式、固定 URL、单一 Egress Host 与无自定义 Header/Environment 全部匹配时启用。它使用官方注册、授权和 Token 端点；Scopes 固定为 `generation.create`、`generation.read`、`account.credit.read`。平台 HTTPS 回调为 `/api/v1/connectors/kling-ai/oauth/callback`。S256 verifier 与 code 加密存储，state 绑定原 User 与单次 Flow；回调保存授权码，由原 User 的完成请求原子领取后交换。可灵发现 metadata 未声明必须返回 `iss`，因此允许省略，但若存在必须严格匹配 `https://klingai.com/auth`。

运行凭证只包含 `MCP_BEARER_TOKEN` 和注册 Client ID；Refresh Token 单独加密保存，不物化到 Runtime。MCP Snapshot 使用该 Authorization 的实际 AAD，保留旧手填凭证的 AAD 兼容。Token 到期后运行拒绝启动；用户可以在设置中刷新或重新连接。断开清理平台授权，不取消已提交的生成任务。

## 当前证据与授权限制

2026-10-01：官方指南、Resource metadata、Authorization Server metadata 与 MCP 401 challenge 已核实；普通 HTTPS 域名的动态注册返回 201，当前部署 `workspace.example.com` 的 HTTPS 回调注册返回 405 安全拦截，本机浏览器与 Linux 部署主机均复现。该上游限制不能用手填 Token 作为替代方案。

本修订尚未完成真实可灵账号授权、授权后 tools/list、图片/视频生成或 Linux + runsc MCP 执行验证。用户明确要求先发布目录。使用 `--activate --allow-unverified-oauth` 可以在授权未验证时先发布；仍检查平台回调适配已部署，并核对正式目录与图标。此选项不创建授权或允许未授权执行，不能把上述协议发现、Fixture 或 ZIP Parse 作为真实账号成功证据。当前包将上传 Egress 限制在 `klingai.com`，上游签名上传到其他域名时应报告限制，需另行审查域名后更新修订。

本地验证：实际 ZIP Parse 与重复构建一致；`go test ./internal/klingmcp ./internal/service/workspace ./internal/connectorpackage ./internal/data/workspace/gormrepo ./internal/data/workspace/runtimeexecutor`、`make test`、`make build`、`make web-typecheck`、`make web-build` 已执行。全量 Go 测试中缺少真实服务配置的集成测试会 Skip，不能视为远端验收；MCP Snapshot 的真实 PostgreSQL 测试另在临时 PostgreSQL 17 中执行。

平台暂存 Revision：`2c548c49-355a-4336-ab6a-786dc9559e1c`；规范化包 SHA-256：`6a43abe0dfa5b29c3d7fe8992ef12a7c2899068f1b61531e14dbc1b2cef6aee7`。2026-10-01 已从 `main_temp` 集成提交 `932fc4c` 部署服务与前端，发布版本 `kling-ai-20261001T085200Z`，正式 Publication 为 available、version 1。Administrator 与 User 目录路由均只有一个可灵条目，使用该 Revision 和官方 PNG。当前 Administrator 作为 User 的 Installation 为 `8d765a2a-d938-418d-bdf6-425c4d706600`，未授权；发布不自动安装到其他 User。集成门禁通过后端全量测试/构建、46 个前端文件的 423 项测试、类型检查和生产构建，线上服务健康检查通过。

Publisher 的三个 Fixture 测试覆盖暂存幂等且无额外写入，以及激活前的回调检查和真实 User 目录路由。显式未验证发布保留平台回调检查（400、no-store、no-referrer）和正式目录核对。真实页面核对有品牌图标、需要设置状态和连接按钮；点击连接到达已部署授权适配，但上游拒绝注册，页面显示操作失败，没有创建授权。MCP 详情只显示安装包校验，不宣称 Runtime 已验证。非敏感目录与发布响应保存在忽略的 `outputs/connectors/kling-ai/`。
