# 微信问题被执行准入拒绝的现场恢复 — 2026-10-04

## 症状与根因

owner 在微信发送两个普通问题，都收到固定“暂时无法执行”状态。对应 Inbox 在 03:35:16 UTC 和 03:36:08 UTC 进入 rejected，reason=dependencies_unavailable，均无 Run。渠道仍 enabled、validation=passed、health=connected；不是微信连接或输入状态调用导致 Run 失败。

临时诊断在生产数据库事务内读取现有 Inbox 与原 Message Channel Conversation，调用真实 `continueRunConversationTriggered`，再调用 `validateQueuedSnapshotAvailability`。事务无论成功失败都回滚，不领取队列、调用模型、结算 Credits 或发送消息。`TestDiagnosticChannelRollback`（交叉编译 Linux 测试可执行文件，仅运行这一测试）稳定复现：后续 Run 构建成功，但 queued CLI Connector availability 校验失败。该会话唯一的托管 CLI Connector 为飞书；Installation 与 Authorization 状态都为 active，但 Authorization 已过期。未删会话、改 specialist/资源选择或绕过授权检查。

## 恢复与验证

使用部署已有 owner 凭证完成正常 OIDC Authorization Code + PKCE，再调用 owner-scoped `POST /api/v1/connectors/{installation_id}/authorizations/{authorization_id}/refresh`，带当前 expected_version。平台使用已存 refresh material 调用供应商并按既有身份、权限与版本校验保存新授权。HTTP 200，新 Authorization 为 active。令牌、外部身份、二维码、授权链接与消息正文未打印或存入证据。

刷新后运行相同生产事务诊断：expired=false，连接器 Snapshot 解析成功，原会话可创建有效后续 Run，测试 PASS（0.16 秒）；所有临时 Run/Event/Selection 变更回滚。诊断源码、二进制和授权刷新脚本均从本地及服务器删除。没有产品代码、配置、Migration、镜像、Web 或 Runtime 改动，不执行新的发布。

现有契约对 rejected Inbox 保持 durable rejection，不自动重跑原问题；请 owner 发送新问题进行真实模型执行和手机输入状态验证。截至本记录，尚未获得刷新后普通问题的实际回答/手机输入状态确认。此次为已过期授权的恢复，不证明自动续期能力；若授权再次过期，仍需要现有刷新或重新授权流程。已有 PostgreSQL 集成测试覆盖过期授权拒绝及版本化授权刷新，本轮未重复运行整套测试、构建或 Linux/gVisor Conformance。
