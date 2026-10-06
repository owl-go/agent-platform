# 小鹅通连接器发布证据 — 2026-10-02

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布结果

- 用户要求发布到平台连接器目录，并明确同意沿用当前域名、先发布并标注授权阻塞。
- 功能提交 `3af3a0d`，分支 `codex/xiaoe-connector`；集成提交 `3e82748`，保留最新 OpenBoost 集成的提交 `8999b25`，目录阻塞说明提交 `8c1e188`，均已推送 `main_temp`。未绕过 `main_temp` 发布。
- 完整部署脚本 `scripts/deploy-platform.sh` 未跳过门禁，发布标识 `xiaoe-20261002T1915`。Source `/srv/agent-workspace/src.release-xiaoe-20261002T1915`、Web `/srv/agent-workspace/web/releases/xiaoe-20261002T1915`、备份 `/srv/agent-workspace/backups/pre-xiaoe-20261002T1915`。
- 部署的服务与 Web 源码来自 `8c1e188`；发布辅助脚本的目录 API 路径修正 `4ff8762` 在部署源上传后提交，实际发布使用该修正后的独立脚本。此修正不改变 API、Worker 或前端代码，不将已上传的 immutable source 宣称为 `4ff8762`。
- [平台目录](https://workspace.example.com/resources?tab=connectors)中 `xiaoe` 0.1.0 为 `available`，MCP 模式、OAuth driver。
- Revision `996c6c5b-7ba9-4a71-a7c2-60b0b42ab308`；实际 Parse 规范化包 SHA-256 `cfc5ca7f6f6670b41416dc594b1d308fa3fdf2a5507e4386c65071f1ac07c543`。
- Administrator 修订目录与 User catalog 各一个正式小鹅通条目。真实发布重跑复用相同 Revision，Publication version 保持 1；未建立临时 CLI Definition。

## 上游与授权边界

先验证 `xiaoe-cloud-cli@0.5.37` 的 npm 来源及下载二进制，实际运行 macOS ARM64 的 help、auth online help 与 schema help。最终选择直接使用小鹅通远程 MCP `https://agent.xiaoe-tech.com/mcp`，没有将未完成 Runtime Conformance 的 CLI 放进包。

小鹅通服务的未授权 initialize 返回 401；其 [Resource metadata](https://agent.xiaoe-tech.com/.well-known/oauth-protected-resource/mcp) 和 [Authorization metadata](https://agent.xiaoe-tech.com/.well-known/oauth-authorization-server) 确认 resource、OAuth 注册/授权/交换/刷新端点、S256 和 mcp/offline_access scopes。

真实平台回调 `https://workspace.example.com/api/v1/connectors/xiaoe/oauth/callback` 的动态注册在本机和部署主机均被 HTTP 566 安全策略拒绝。示例 HTTPS/localhost 注册成功不算真实平台授权通过，也没有用于绕过上游限制。按用户要求保留当前域名，在目录描述及连接失败提示中明确标注阻塞。

固定 provider policy、加密 state、owner/flow 绑定、PKCE、issuer 校验、一次性 code 消费、独立 refresh token 密文，以及短期 MCP Bearer 凭证投影已有协议和应用测试。托管 MCP 快照冻结同修订 Package Object Key/SHA-256，验证包完整性并挂载配套 Skill。Skill 的操作候选来自上游使用说明，实际工具以授权后的实时 Schema 为准。

## 已执行验证

- 最终四文件 MCP ZIP 实际调用仓库 `connectorpackage.Parse`，包内有品牌 SVG 和一个配套 Skill；构建与发布 3 项 Python 测试通过。
- 小鹅通 OAuth、服务回调、包解析、GORM 投影与 Runtime executor 目标 Go 测试通过。合并后完整 `make test`、`make build` 通过。
- 整合后的完整前端 46 文件 / 449 用例通过，`make web-typecheck`、生产 Web 构建及 `git diff --check` 通过。
- 正式部署执行数据库/配置备份校验、Runtime 与 CLI Builder image smoke、已安装 CLI bundle 候选 Runtime 复核、API/Worker/Egress Controller/Caddy 健康检查及 Web 发布检查。bundle 复核日志为 0 个 active bundle；不把它当成小鹅通 CLI 的 Conformance。
- 发布后的 Administrator publication-health、修订目录与 User catalog 返回相同 Revision/包 SHA，正式条目数量为 1。MCP 的 conformance_available 表示平台安装包校验通过，不等同于真实业务工具调用通过。

## 浏览器与清理验收

使用隔离的 Playwright CLI 浏览器登录线上平台管理员自己的 User 视图。实际市场卡片及详情显示官方蓝色小鹅品牌图标、版本 0.1.0、安装包校验通过和回调域名阻塞说明。截图已人工查看。

在该账号临时安装，详情显示可操作的连接按钮；点击后真实 HTTP 566 被平台映射为 xiaoe_oauth_callback_blocked，页面显示联系小鹅通放行回调域名或配置正式域名的提示，没有手动 Token 表单。诊断 Installation `1e4de3c9-18e8-4e58-8fc1-56764b93c0f8` 保持未授权，授权列表为零。检查该精确 ID、无凭证及当前 version 后通过乐观锁 DELETE 清理；User 目录仍为一个 available 小鹅通条目，诊断安装已不存在。未给其他 User 安装或升级。

本地最终包和非敏感 API 证据保存在被忽略的 `outputs/xiaoe`，截图位于 `output/playwright/xiaoe-details.png` 与 `output/playwright/xiaoe-authorization-blocked.png`。短期登录凭证文件、浏览器持久状态和远端临时发布目录在验收后清理；永久部署、备份、Publication 和 Revision 保留。

## 尚未验证

真实小鹅通账号与店铺授权、授权后的 tools/list、业务读写、账号范围和权益仍未验证。未执行完整模型 Production Conformance、完整 Sandbox Conformance 或真实远端存储 Conformance；环境缺失的集成 Skip 不记为真实环境通过。此包不含 CLI，bundle × Runtime Conformance 不适用。上游放行当前回调后需补真实授权与业务验收。
