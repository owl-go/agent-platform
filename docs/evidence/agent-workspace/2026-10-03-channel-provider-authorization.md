# 十三种消息渠道按实际接入方式配置 — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 行为与实现

全部十三个入口使用独立的接入声明，包含实际凭证字段、接收方式、外部身份标签、接入指南与私聊/群聊限制。配置分为账号接入和消息接收：确认账号身份后才设置受众；编辑已保存的账号可保留凭证，重新授权使用同一实际身份校验。

| 渠道 | 此平台已实现的账号接入 / 消息接收 |
|---|---|
| Telegram | Bot Token / 平台验证时注册 Webhook |
| Discord | Bot Token / Gateway；需对应 Intent 与频道权限 |
| Slack | Bot OAuth Token、Signing Secret / Events API 回调 |
| 钉钉 | 应用 Client ID/Secret、Corp ID / Stream |
| 飞书/Lark | App ID/Secret、Tenant Key、服务区域 / 官方长连接 |
| Matrix | 已批准的 Homeserver、Access Token / 新消息轮询，显式未加密 Room ID |
| WhatsApp | Business Cloud API 应用与账号凭证 / 回调，仅用户发起私聊回复窗口 |
| Signal | 已批准的 signal-cli HTTPS 服务、Bridge Token、Account ID / RPC 与 SSE |
| 企业微信 | 智能机器人 Bot ID/Secret / API 长连接 |
| 微信 | 腾讯 iLink 扫码授权 / 长轮询，仅私聊 |
| QQ Bot | 腾讯扫码绑定或已有 App ID/Secret / HTTP 回调；仍需供应商回调配置 |
| BlueBubbles | 已批准的 macOS HTTPS 服务与密码 / 新消息轮询，仅单人聊天 |
| 元宝 | App Key/Secret / 官方 WebSocket |

扫码实现依据 [腾讯微信 iLink 协议](https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol.md) 和腾讯发布的 [QQ Connector 1.2.0](https://www.npmjs.com/package/@tencent-connect/qqbot-connector)。QR ticket、QQ AES-GCM 绑定 Key 与取得的凭证仅在服务端加密保留；前端收到二维码内容、状态与非敏感账号身份。服务端临时授权绑定 owner/workflow/provider/region/编辑 channel/version，5 分钟过期，有全局与每 owner 数量上限；取消、到期或保存成功销毁。二维码 PNG 在浏览器本地编码，不访问远程二维码图片或执行供应商 HTML。一次性 login_id 保存通过现有渠道凭证加密与 Repository 事务，不能跨作用域或重放。

扫码/凭证 Identify 成功只表示账号接入。保存仍为关闭、未验证；真实允许聊天中的测试消息及原聊天回复通过后才可启用。没有提供尚未实现的 OAuth、Socket Mode、个人 WhatsApp 扫码或 Signal 设备绑定入口。

## 实际检查

- Go Application、Adapter、HTTP Service 目标测试与 race 检查通过；覆盖临时状态加密、凭证不返回前端、owner/workflow/provider 隔离、过期、取消、重放、会话数量上限、QQ Secret GCM 完整性、微信配对码与可信 baseurl/redirect 限制。
- 真实 PostgreSQL 17 的完整 GORM Repository 测试通过（26.087 秒），包括此前资源查询状态隔离回归与消息渠道事务测试。全量 `make test`、`make build` 通过。
- 全量前端 52 个文件、551 个测试通过；覆盖全部渠道的实际字段、微信/QQ 扫码进度、过期与关闭取消、服务端 login_id 保存、直接聊天限制和 Matrix 必填房间。`make web-typecheck`、`make web-build` 通过。
- `make verify-generated`、`make breaking`、`git diff --check` 通过。合并 `main_temp` 后再次通过完整 Go 测试/构建、前端类型/构建及 551 个测试。
- Playwright 使用临时本地 fixture 检查实际组件：微信在 1280×900 与 390×844 正常展示 QR、状态和操作，QQ 可切换扫码与应用凭证，飞书有区域及其专用字段。fixture 仅用于视觉检查，没有把模拟响应作为生产授权证据；临时入口已清理。
- 首次候选验证发现微信官方约 30 秒返回等待状态，与 API 30 秒 unary deadline 冲突，等待请求返回 422。独立官方请求确认 `ret=0,status=wait`；修复将供应商单次长轮询限制为 20 秒，本地轮询期限返回 waiting，调用者取消仍传播。回归测试实际等待本地期限，确认状态可继续使用且先于 API 期限返回；目标测试和 race 检查均通过。故障候选没有切换为正式 API。

## 发布与真实接口验收

后端功能 `eb7ffcc` 与期限修复 `9ba5799` 从 `main_temp` 集成版本 `44105ad3b1620bcac2f594c01cb63c46edbb7eca` 构建。前端接收提示修正 `6201194` 经 `main_temp` 集成为 `e42b74f362cf4678e8f5f11b9afcb3b50bd9d913`，保留此前 Workflow Knowledge Input 的发布内容。API/Worker 与 Web 发布均为 `channel-provider-auth-20261003-1`；没有从功能分支发布。

发布前业务数据库备份经 `pg_restore -l` 与 SHA-256 清单校验，配置、原源码/Web 指针及 API/Worker 镜像记录保存在服务器 `/srv/agent-workspace/backups/pre-channel-provider-auth-20261003-1`。候选 API 仅绑定回环地址，使用正常 OIDC Authorization Code + PKCE 登录验证；没有打开 Direct Access Grant、创建新用户、绕过认证或修改 User Workflow。

最终候选和正式公网 `https://workspace.example.com` 均实际通过：

- CLI Enablement、Expert、Expert Team、Skill、MCP、CLI Definition、Connector Catalog、Connector Installation、Command Approval 九个资源接口为 200。
- 微信与 QQ 各自调用官方服务取得真实二维码：start 为 200/waiting、非空 QR，响应无凭证字段；poll 为 200/waiting；cancel 为 200，取消后 poll 为 404。
- 每次探测结束仅注销本次登录取得的 Refresh Token。没有保存响应正文、QR 内容、Session ID、Token 或账号凭证；没有发送真实聊天消息或创建渠道配置。

切换前无非终态 Run，先停止 Worker，重建 API/Worker 并等待 healthy，再切换源码指针；Web 经现有发布脚本构建与原子激活。API 镜像 `sha256:c1e86f282fa835948a84cd7d7a419344819ce5058595fb4001cb927ced44a04c`，Worker 镜像 `sha256:b3e14ecc1ada40a176636451842c2c4387d7a9c87aedf25d721c457451f0f908`。公网 healthz/readyz 分别为 ok/ready；启动日志 ERROR/FATAL/PANIC 为 0，四个消息渠道循环均 started=1、fatal=0。Migration 名称/checksum 与配置 SHA-256 保持一致，记录数不变：Expert 8、Skill 13、Connector Installation 20、Message Channel 0。临时候选容器已删除，脱敏证据位于服务器 `/srv/agent-workspace/evidence/channel-provider-auth-20261003-1`。

公网 Web 的 index.html 和入口引用的资源，以及 Workflow Detail JS 均与本地生产构建逐字节一致；index.html SHA-256 为 `bb161591958bdd64b55b8b8d4af806cfda240713df6398bf8552f4bc9372e3b9`，Workflow Detail 资源为 `WorkflowDetailPage-DRSa-pD_.js`。

## 验收边界

真实二维码申请、轮询与取消已经验收；账号持有人的扫码确认、取得实际账号凭证后的完整绑定，以及十三个渠道各自的真实聊天接收、模型执行和回复仍需要对应账号验收。协议/状态测试、Identify 或生成二维码不等于 IM 闭环已通过。本次没有改 Runtime、Sandbox 或 Migration，没有新增 Linux/gVisor、Runtime Digest 或 Production Conformance 证据。临时授权使用单 API 进程内状态，重启需重新授权，多 API 部署须采用粘性路由或实现满足相同边界的共享状态存储。
