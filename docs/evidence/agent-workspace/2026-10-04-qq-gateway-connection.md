# QQ 扫码绑定、上线与工作流闭环 — 2026-10-04

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 原因与修复

用户反馈手机 QQ 授权页面一直“连接中”，机器人离线。生产数据库已有有效加密配置，但渠道未验证且停用；真实 Token、账号身份与 Gateway 查询均通过。原注册表只给 QQ 注册 HTTP callback Receiver，没有扫码绑定所需的官方 WebSocket 上线连接。扫码成功取得 App ID/Secret 不等于机器人已上线。

通过实际 NewTransports 注册表和 ChannelConnections 监督器构造确定性回归：`go -C backend test ./internal/data/messagechannel -run TestQQGatewayRegisteredConnectionReachesInbox -count=1` 最初失败，提示 `QR-bound QQ bot never authenticated online or received its message`。修复后同一测试通过：HELLO → Identify → READY → C2C 消息进入 Inbox，取消关闭 Socket。

QQ 改为独立 StreamReceiver；官方 Gateway URL 固定到 api.sgroup.qq.com 的 websocket 路径，拨号校验公网 DNS，不接受私网、其他域名、端口、query 或编码路径。Identify 只订阅 C2C/群 @ 所需的 Intent；接收原生 sender/chat/message ID，沿用公共授权、脱敏、Inbox、Run Conversation 与 Delivery。READY/RESUMED 通过可选 ChannelStreamHealthReceiver 更新连接健康，不创建用户消息、Run 或虚假的验证结果。心跳缺少 ACK 会关闭连接；进程内重连按官方 close code 选择 Resume、重新 Identify 或拒绝。Inbox 提交成功才推进 Resume Sequence。HTTP callback 签名逻辑保留协议测试，产品不再注册 QQ callback，前端不要求填写回调地址。

协议依据：[腾讯 QQ 官方 WebSocket SDK](https://github.com/tencent-connect/qqbot-agent-sdk/blob/main/src/qqbot_agent_sdk/websocket.py) 与 [Opcode/Intent/close-code 定义](https://github.com/tencent-connect/qqbot-agent-sdk/blob/main/src/qqbot_agent_sdk/dto.py)。账号保存仍未验证、停用；只有真实验证窗口或已启用配置维持连接，停用/删除继续停止接收。

## 已执行验证

- QQ 注册表、连接监督、原聊天 Inbox、READY 不生成消息、Resume 与提交边界、群 @ 归一化、心跳与缺少 ACK、取消、非法帧、地址拒绝和 close-code 恢复测试通过。原 QQ QR AES-GCM、签名与被动回复预算测试继续通过。
- `go -C backend test -race ./internal/data/messagechannel ./internal/biz/workspace/application`；最后 close-code 修正再执行目标 QQ race 测试。
- 功能分支与 main_temp 的 `make test`、`make build`；QQ 配置及 i18n 前端测试、`make web-typecheck`、`make web-build`、`git diff --check` 通过。最终集成版本目标前端测试 28 项通过。
- 本次未新建 PostgreSQL 测试数据库；环境依赖的集成测试 Skip 不作为通过证据。实际生产 PostgreSQL 的配置、Inbox、Run 与 Delivery 闭环另行检查。没有 Runtime、CLI Builder、Sandbox 或 Migration 变更，未新增完整 Linux/gVisor Production Conformance 证据。

## 发布与现场验收

功能提交 8a7dd99、恢复修正 874655f 均提交并推送，分别经 main_temp 合并 6fc18b6、e18c7cc；最终发布源码为 main_temp 的 f5529d7eecec8cdc06e6ddb6c1bd703d2672f0fb。发布 qq-gateway-20261004-2 的业务 pg_dump 与配置备份经 SHA-256 检查，pg_restore -l 检查；记录旧镜像及源码/Web 指针，位于 /srv/agent-workspace/backups/pre-qq-gateway-20261004-2。候选 API readiness 通过，切换前没有非终态 Run 或活跃 Session Message，停止旧 Worker 后切换 API/Worker。第一次候选检查漏带线上 edge 网络，OIDC 域名无法解析；补齐相同网络后通过，没有绕过启动检查。

最终 API 镜像 sha256:37979bb1b20495f280f66b012d8d87d32c8dde6a3261a15ec1f960ab57565c83；Worker 镜像 sha256:1998f533e1a34c788c28c0bb2c881a91c763fa7efaecec977e9a5bf5ab72d0d4。两者 healthy，公网 healthz/readyz HTTP 200。Web 同版本已发布，公网 HTML SHA-256 为 9e0e2d5b9ed59225577a33014dbc2f6c1ea79b9c532644f8df6651f5cf155168。启动 fatal/panic 行数为零。安全发布检查在 /srv/agent-workspace/evidence/qq-gateway-20261004-2。

通过正常 owner OIDC/API 为现有绑定启动真实验证；Gateway 握手后 connected。用户在 QQ 发送验证文案，数据库确认 validation Delivery sent，用户确认“收到验证回复，连接已完成”。随后通过 owner API 启用当前已验证配置。用户报告普通问题暂未出现时，后台只有验证 Inbox；没有该问题的上游到达证据，不能断言它在连接切换时丢失。后来确认两条 QQ 普通消息已准入：两次 Run succeeded、两条 answer Delivery sent，复用一个 Run Conversation，触发来源为 qqbot；用户确认“现在可以了”。验证消息没有创建 Run。

最终发布保持已有验证、启用与会话历史，Worker 重启后重新连接。手机连接与普通消息闭环已实测；跨进程未提交消息补收、群聊真实账号、自然长时间掉线及封禁/权限错误未做生产故障注入，不作通过声明。临时凭证探测、owner 操作与候选容器工具清理，不保存 Token、App Secret、二维码、外部身份或消息正文。
