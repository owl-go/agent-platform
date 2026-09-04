# 服务端架构

状态：Agent Workspace 当前实现

## 结构

后端是两个 Go Kratos 进程：`cmd/api` 提供认证后的控制面，`cmd/worker` 领取会话回复、工作流 Run、定时触发和 MCP 测试。Wire 只负责显式装配；所有运行配置来自严格校验的 YAML。

当前实现分为三个限界上下文：

- Account：OIDC 身份、本地 User 投影、管理员创建/启停账号和密码重置。
- Workspace：Session、Workflow、Run Conversation、Run、Expert、平台级 Model Provider Connection 与 Provider Model、MCP Server、Skill 与 Personal Settings。
- Credits：Credit Ledger、余额投影、Daily Credit Allocation、Redemption Code、Model Credit Rate、Credit Adjustment，以及模型执行的积分准入和结算。

Account 只向 Credits 提供 User 身份，不拥有积分状态。Workspace 通过 Credits 的 Application 端口检查准入、冻结每个 Execution Stage 的费率并结算实际消耗，不直接更新 Credit Ledger 或余额投影。三个上下文可以使用同一个 PostgreSQL 实例，但 Domain 和 Application 端口不泄漏 GORM Model。

Domain 与 Application 不依赖 GORM、HTTP、对象存储、Runtime CLI 或 YAML。`internal/data` 实现 PostgreSQL、Runtime、Keycloak 等端口；`internal/service` 只做 Proto/HTTP 映射、身份提取与公开错误转换。

## 所有权

Session、Workflow、Expert、Extension 和 Personal Settings 等 User-owned 资源的每个查询和写入都以认证 User ID 过滤。Model Provider Connection 与 Provider Model 构成平台级 Model Catalog，所有认证 User 可读取，只有 Administrator 可写；User 只保存引用全局 Provider Model 的个人默认选择。管理员可以查看账号级余额、今日用量、每日额度、兑换和人工调整记录，但不能借助管理权限读取其他 User 的会话、工作流、扩展或逐次执行消费明细。跨 User ID 与不存在资源使用相同的 Not Found 语义。

## 事务与并发

- Session 发消息在一个事务中创建 User Message 和排队中的 Assistant Message；同一 Session 同时只有一个生成任务。
- Run Conversation 的首个 Run 创建时冻结 Workflow 与有序 Execution Stage Snapshots；每个 Stage 独立包含可选 Expert、Provider Model、Model Provider Connection 版本、Model API Protocol、Endpoint、Runtime 与扩展配置，环境变量和共享 Workspace 配置保留在公共快照层。后续追问创建新的不可变 Run 并复用该快照和 Workspace。一个 Run Conversation 同时只有一个 Run 可执行，同一 Workflow 的所有 Run 仍串行执行。API Key 通过各 Stage 的版本化凭证引用在 Worker 领取时加载，不进入普通 Snapshot JSON。
- Worker 按 Session 或 Run Conversation、冻结 Expert 身份和 Runtime Engine 维护隔离的 Warm Runtime Container 租约。租约不共享执行上下文、User 或资源边界；团队成员只按顺序挂载同一轮的 Workflow 临时 Workspace。执行结束立即停止并清理单次凭证，空闲 30 分钟后回收 Container 定义。
- Run 状态与终态 Event 在同一 Repository 事务提交；Event Sequence 从 1 单调递增且只有一个终态。
- Credits 上下文以不可变 Credit Ledger 为事实来源，并在同一事务维护 Credit Balance、每日额度剩余和今日用量投影。Daily Credit Allocation 以 `(user_id, credit_day)` 唯一，消费结算以 `(execution_id, stage_position)` 唯一；重试只能重放原结算，不能重复发放或扣减。
- 每个 User 的积分模型调用串行。调用开始前事务性物化当日额度并检查正余额；每个 Execution Stage 冻结 Model Credit Rate 修订，完成后以本次输入和输出 Token 增量结算。Stage 终态、Credit Ledger 消费记录和余额投影在一个 Repository 事务中提交；单 Stage 或团队最后一个 Stage 同事务提交 Assistant Message 或 Run 终态，结算后的负余额会阻止下一次调用。
- 跨零点调用归属开始时的 Credit Day。次日额度通过首次余额读取或执行准入惰性物化，不依赖零点批处理；Personal Settings 时区变更只能从下一个 Credit Day 生效。
- 更新使用 Version 乐观锁；外部 Workflow API 创建 Run 还使用 `Idempotency-Key` 保存响应。
- Worker 使用 PostgreSQL `FOR UPDATE SKIP LOCKED` 领取任务。进程崩溃后的悬挂任务由运行超时和后续对账收口，不暴露为产品控制。

## API

`backend/api/workspace/v1/workspace.proto` 是普通 JSON API 的权威契约。用户认证使用 Bearer OIDC Token。Workflow API Key/API Secret 只允许通过 HTTP Basic 调用该 Workflow 的 Token Exchange；返回的 15 分钟 JWT 通过 Bearer Header 启动和查看该 Workflow 的 Run，不代表 User 身份，也不能访问其他产品 API。

Credits 契约允许 User 读取自己的余额和 Credit Ledger、兑换 Redemption Code，并允许 Administrator 管理账号每日额度、Model Credit Rate 修订、Redemption Code 和带原因的 Credit Adjustment。余额不足统一映射为 `insufficient_credits` 和 HTTP `429 Too Many Requests`；返回当前余额与下一次每日额度时间，不返回其他 User 或内部费率数据。

工作流历史中的每一行是一个 Run Conversation。`GET /api/v1/workflows/{workflow_id}/runs/{run_id}/turns` 按顺序读取所有 Run；`POST` 同一路径提交追问并排队一个新 Run。已经终态的 Run 永不重开，因而事件顺序、终态和 Artifact 审计边界保持不变。

两个流式端点有意使用手写 Handler：

- `GET /api/v1/workflows/{workflow_id}/runs/{run_id}/events`：SSE 历史回放、实时事件与 Heartbeat。
- `GET /api/v1/sessions/{session_id}/messages/{message_id}/events`：按 Owner 隔离持续推送 Assistant Message 快照。快照只暴露受限的产品进度阶段与已脱敏答案，不暴露 Runtime 原始事件、命令内容或模型私有推理；完成、失败或取消后关闭连接。
- `GET /api/v1/workflows/{workflow_id}/workspace/download?path=...`：认证后流式下载 Workspace 文件。
- Workspace HTTP API 只提供目录查看、文本预览和文件下载；Git Clone 由 `/api/v1/workflows/{workflow_id}/git-source` 设置入口完成。

## Secret

Model Provider API Key、Workflow Secret 环境变量、MCP Secret、Git HTTPS 密码/Token 和 Git SSH 私钥使用服务端数据密钥加密。读取 API 只返回 `configured`，不返回明文。执行时 Secret 物化为单次任务的 0600 文件，经公共 Entrypoint 导入；Runtime 输出、Event、结果和 Artifact 在持久化前使用精确值脱敏。

## 数据库

当前产品以全新基线 Migration `000001_agent_workspace.sql` 建库，后续修正只通过不可变的追加式 Migration 演进；`000005_model_provider_connections.sql` 将早期 Model Profile 数据清空并替换为 Model Provider Connection、Provider Model 与版本化凭证结构，后续 Migration 删除模型类型字段，`000014_global_model_catalog.sql` 再把已有连接与模型目录提升为全局可读资源并保留原凭证加密作用域。Provider Model 优先来自供应商 `/models`，失败或不支持时使用平台维护的厂商默认列表，Administrator 也可显式补充。从旧企业控制面切换前必须备份并重建业务数据库；不支持把旧 Organization/Team/Agent Release 数据猜测性映射为新 User 私有数据。

Credits 通过新的追加式 Migration 引入，不修改既有 Migration。Migration 为现有 User 建立上线当日的 600 Credit Allocation，兑换余额从零开始；只有在目标环境实际运行 Migration 后才能报告为已执行。
