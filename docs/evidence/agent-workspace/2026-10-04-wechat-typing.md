# 微信执行期输入状态 — 2026-10-04

## 实现边界

owner 要求微信等待工作流答案时显示原生“正在输入”。`ChannelTransport` 新增独立可选 Typing port，仅 WeChat 注册；不修改最终答案 Outbox，不把输入状态伪装成文本消息。Worker 在执行启动时异步获取入站 sender 对应的临时 typing_ticket，每五秒刷新 status=1。每次刷新先重新检查 owner、Workflow、Run、Inbox、渠道启用状态、当前配置版本、generation、受众与执行状态。排队和终态不显示，审批等待暂停，恢复后重新取 ticket。

完成、失败、取消、停用或 Worker shutdown 使用独立三秒 context 清除 status=2。正常执行先等待有界清理，再提交终态，避免下一轮已开始时上一轮迟到的清除覆盖新状态。输入状态接口错误不使 Run 失败、不改变 Credits、连接健康或答案投递；网络无法完成清除时仅尝试有界清理。ticket、Bot 凭证与 context_token 只在 host transport session 使用，不进入 Runtime、Snapshot、Delivery Payload 或日志。永久日志只有 provider 和固定 lifecycle event。

## 实际检查

- Adapter 测试核对实际入站参与者和 context_token、一次获取临时 ticket、连续 status=1 刷新与 status=2 清除；拒绝供应商非零/格式错误返回、缺失/错误 ticket、HTTP 拒绝、群聊和缺失回复上下文，不调用消息接收或答案发送接口。
- Application/Worker 测试核对连续刷新、取消、幂等清理、审批暂停与恢复、授权撤销、unsupported provider、Session 排除、获取票据与刷新失败、成功/失败/User cancellation/shutdown，以及清理先于终态提交。相关 Application 与 Adapter 测试实际通过 race 检查。
- 临时 PostgreSQL 17 执行完整 Migration 链与全部 `TestChannel` 集成测试。新查询覆盖运行/排队/审批/终态、owner/Workflow 隔离、渠道停用/删除、配置版本和 generation 漂移、owner 停用、Workflow 删除、取消请求；真实 GORM Scan 使用显式导出投影字段。已有渠道准入、上下文、回复及运行历史测试保持通过。临时容器已移除。
- 功能分支 `make test`、`make build`、目标四包测试、`git diff --check` 实际通过；`main_temp` 再次执行完整后端测试与构建。前端未改动，本轮没有重复 Web 测试或构建。

## 发布与现场核查

功能提交 `ec144eb`，经 `main_temp` 合并为 `0733e90b11d249fe8bcf597c8255698d268a72fd` 并推送；发布标识 `wechat-typing-20261004-1`。只重建和发布 Worker，镜像为 `sha256:9135ca94ec22bcea382bfe19c3e08c4105e69d815a4abdc6d4b653b5aec72824`。切换前无 queued/running/waiting Run 或 generating/waiting Session Message，停止旧 Worker 后启动新 Worker并等待 healthy。API 镜像仍为 `sha256:916fd12d691dc881669a08518cad636c8f58c6363ba84bca869f08ff00ae7d18`，Web 指针不变。

发布前备份 `/opt/agent-platform/backups/pre-wechat-typing-20261004-1`：业务 pgdump 经 `pg_restore -l` 和 SHA-256 校验，配置归档 SHA-256 校验，保存旧源码/Web 指针、API/Worker 镜像与 Migration ledger。配置 SHA-256 和全部 Migration name/checksum 切换后相同。回退使用保存的旧 Worker 镜像与源码指针；没有数据库变更。API/Worker 均 healthy，四个渠道循环 started=1、fatal=0，Worker 启动 ERROR/FATAL/PANIC 行数为零，公网 healthz/readyz 正常。公网 HTML SHA-256 保持 `62799bb3e81f305fa77c180386678936177b9e4143aab935920150469dc654ce`。

只读真实账号探测在进程内解密最新已准入消息的凭证和回复上下文，使用实际 sender/context_token 请求腾讯官方 HTTPS `getconfig`；拒绝重定向，仅记录 HTTP 状态、数值返回码和 ticket 是否存在。结果 HTTP 200、ret=0、ticket_present=true，见 `/opt/agent-platform/evidence/wechat-typing-20261004-1/readonly-ticket.json`。该探测未发送输入状态或文本、消费 Cursor、创建 Run 或调用模型。临时探测已从本地和服务器删除，凭证及响应 ticket 未保存。

已请 owner 发送普通微信问题并确认等待时显示输入状态、答案到达后消失。截至记录时尚无新版本的手机显示确认，也没有普通 Run 的实际 typing lifecycle 日志，因此不能把票据可用和本地测试记为手机显示验收。Runtime、Sandbox、CLI Builder 镜像不变，本轮没有新增 Runtime Digest、Linux/gVisor 或 Production Conformance 证据。
