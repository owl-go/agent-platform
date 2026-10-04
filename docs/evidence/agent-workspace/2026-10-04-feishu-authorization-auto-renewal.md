# 飞书 Connector Authorization 自动续期 — 2026-10-04

## 实现与范围

owner 在微信普通问题被拒绝后要求自动续期。根因是原 Run Conversation 引用的飞书 CLI OAuth 授权已过期。新增 Application 续期用例：Worker 每分钟检查已选飞书授权，到期前五分钟刷新；渠道 Inbox 准入前也检查其 owner 的授权。续期通过现有 Feishu Adapter 进行，HTTP 不持有数据事务，单次限时二十秒。成功轮换加密 Access/Refresh Token，保持账号身份、加密 AAD 和版本化审计；返回的 scopes 可缩小，不能扩大。省略 scope/refresh token 时沿用已有值。旧 access_token + 独立加密 refresh credential 也可迁移为 JSON 凭证。

只选择当前 active owner、active Installation、已选 grant、available Publication 与飞书 CLI Driver。保存前重查这些条件及 Authorization version；撤销、停用、卸载、切换账号或取消选择不会被自动恢复。手动 API 刷新共享 PostgreSQL session advisory lock，并在外部请求前拒绝 stale expected_version，避免消耗已轮换的 refresh token 后才发现版本冲突。Inbox 在并发刷新期间延后准入。失败保留原存储授权、按 grant 冷却五分钟，重启清空冷却；无有效 refresh credential 仍需 owner 重新授权。日志仅 provider 和固定 renewed/failed，不含供应商原始错误或凭证。

当前后台自动续期支持托管飞书 CLI OAuth；其他 provider 保留各自生命周期，未声称一律支持自动续期。供应商有效期按实际返回值保存，未增加“永不过期”配置或修改其授权时长。

## 已执行验证

- Application 场景：已过期、即将过期、已被手动续期、锁竞争、撤销、供应商失败与冷却、缺少 refresh token、账号改变、scope 扩大/缩小、历史凭证和保存版本冲突。相关测试实际通过 race 检查。
- PostgreSQL 17 临时数据库执行完整 Migration 链及全部 TestChannel、续期 eligibility、授权选择与版本化刷新集成测试。真实 channel follow-up 先遇到过期依赖，再经自动续期准入，保留 root Conversation、第二轮 Run 和一条 refresh audit。覆盖手动/后台共享锁、幂等释放，以及断开、owner 停用、Installation 停用、取消选择、Publication 停用、错误 Driver 和其他 owner 均不能续期。临时数据库容器已删除。
- HTTP 回归验证 stale expected_version 和正在刷新均在任何供应商调用前返回既有 412 version_conflict，释放已持有锁。
- 目标包测试、功能分支 make test/make build、main_temp make test/make build、git diff --check 实际通过。前端未改动，不重复 Web gates。Runtime/CLI Builder/Sandbox 镜像不变，没有新增 Linux/gVisor Digest Conformance。

## 发布与实际供应商验证

功能提交 4b203f7，经 main_temp 合并 b3d5a3a 并推送；发布 feishu-renewal-20261004-1。预发布备份 business pgdump/config tar 经 SHA-256 检查，pg_restore -l 检查，保存旧镜像/源码/Web 指针于 /opt/agent-platform/backups/pre-feishu-renewal-20261004-1。候选 API healthy/ready，切换前非终态 Runs 与活跃 Session Messages 均为零；停止旧 Worker 后发布 API 与 Worker。

API 镜像 sha256:1328e78cc260c313f66af09886858ca4377f9047034eb58fdf3f1bcaead1913a；Worker 镜像 sha256:bc86a12598bf31f26f7a3a248aa1a57262fff6b9f33a03cfb03aec5b5511e9be。两者 healthy，公网 healthz/readyz HTTP 200，Worker 启动 ERROR/FATAL/panic 行数零；独立 connector-authorization-renewal loop started=1、fatal=0。Web 指针不变，公网 HTML SHA-256 保持 62799bb3e81f305fa77c180386678936177b9e4143aab935920150469dc654ce。没有新 Migration 或配置修改。

实际提前续期探测使用同一 Application、真实 Repository/Cipher 和官方 HTTPS Feishu Adapter。仅在探测的内存 Repository wrapper 中提前到期判断，不修改数据库到期日以制造故障；调用供应商并通过生产保存边界提交新加密授权。renewal_event=renewed，Version 增加一，账号一致，ExpiresAt 延长，测试 PASS（0.64 秒）。未发送业务消息、调用模型、改变资源选择、消费微信 Cursor 或生成 Run。源码、二进制与探测已删除，密钥/令牌/账号正文没有输出或写入证据。服务器安全记录位于 /opt/agent-platform/evidence/feishu-renewal-20261004-1。

此证据验证一次真实续期和到期条件的自动化回归；尚未跨真实两小时自然到期周期观察后台刷新，也不证明供应商刷新凭证永久有效。微信手机“正在输入”显示仍需 owner 实测确认。
