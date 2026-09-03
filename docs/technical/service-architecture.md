# 服务端架构

状态：Agent Workspace 当前实现

## 结构

后端是两个 Go Kratos 进程：`cmd/api` 提供认证后的控制面，`cmd/worker` 领取会话回复、工作流 Run、定时触发和 MCP 测试。Wire 只负责显式装配；所有运行配置来自严格校验的 YAML。

业务分为两个限界上下文：

- Account：OIDC 身份、本地 User 投影、管理员创建/启停账号和密码重置。
- Workspace：Session、Workflow、Run Conversation、Run、Expert、Model Provider Connection、Provider Model、MCP Server、Skill 与 Personal Settings。

Domain 与 Application 不依赖 GORM、HTTP、对象存储、Runtime CLI 或 YAML。`internal/data` 实现 PostgreSQL、Runtime、Keycloak 等端口；`internal/service` 只做 Proto/HTTP 映射、身份提取与公开错误转换。

## 所有权

除管理员账号操作外，每个查询和写入都以认证 User ID 过滤。管理员不能借助管理权限读取其他 User 的会话、工作流、模型或扩展。跨 User ID 与不存在资源使用相同的 Not Found 语义。

## 事务与并发

- Session 发消息在一个事务中创建 User Message 和排队中的 Assistant Message；同一 Session 同时只有一个生成任务。
- Run Conversation 的首个 Run 创建时冻结 Workflow、Expert、Provider Model、Model Provider Connection 版本、Model API Protocol、Endpoint、Runtime、环境变量与扩展配置；后续追问创建新的不可变 Run 并复用该快照和 Workspace。一个 Run Conversation 同时只有一个 Run 可执行，同一 Workflow 的所有 Run 仍串行执行。API Key 通过版本化凭证引用在 Worker 领取时加载，不进入普通 Snapshot JSON。
- Worker 按 Session 或 Workflow 维护隔离的 Warm Runtime Container 租约。租约不共享 User 或资源边界；执行结束立即停止并清理单次凭证，空闲 30 分钟后回收 Container 定义。
- Run 状态与终态 Event 在同一 Repository 事务提交；Event Sequence 从 1 单调递增且只有一个终态。
- 更新使用 Version 乐观锁；外部 Workflow API 创建 Run 还使用 `Idempotency-Key` 保存响应。
- Worker 使用 PostgreSQL `FOR UPDATE SKIP LOCKED` 领取任务。进程崩溃后的悬挂任务由运行超时和后续对账收口，不暴露为产品控制。

## API

`backend/api/workspace/v1/workspace.proto` 是普通 JSON API 的权威契约。用户认证使用 Bearer OIDC Token。Workflow API Key/API Secret 只允许通过 HTTP Basic 调用该 Workflow 的 Token Exchange；返回的 15 分钟 JWT 通过 Bearer Header 启动和查看该 Workflow 的 Run，不代表 User 身份，也不能访问其他产品 API。

工作流历史中的每一行是一个 Run Conversation。`GET /api/v1/workflows/{workflow_id}/runs/{run_id}/turns` 按顺序读取所有 Run；`POST` 同一路径提交追问并排队一个新 Run。已经终态的 Run 永不重开，因而事件顺序、终态和 Artifact 审计边界保持不变。

两个流式端点有意使用手写 Handler：

- `GET /api/v1/workflows/{workflow_id}/runs/{run_id}/events`：SSE 历史回放、实时事件与 Heartbeat。
- `GET /api/v1/sessions/{session_id}/messages/{message_id}/events`：按 Owner 隔离持续推送 Assistant Message 快照。快照只暴露受限的产品进度阶段与已脱敏答案，不暴露 Runtime 原始事件、命令内容或模型私有推理；完成、失败或取消后关闭连接。
- `GET /api/v1/workflows/{workflow_id}/workspace/download?path=...`：认证后流式下载 Workspace 文件。
- Workspace HTTP API 只提供目录查看、文本预览和文件下载；Git Clone 由 `/api/v1/workflows/{workflow_id}/git-source` 设置入口完成。

## Secret

Model Provider API Key、Workflow Secret 环境变量、MCP Secret、Git HTTPS 密码/Token 和 Git SSH 私钥使用服务端数据密钥加密。读取 API 只返回 `configured`，不返回明文。执行时 Secret 物化为单次任务的 0600 文件，经公共 Entrypoint 导入；Runtime 输出、Event、结果和 Artifact 在持久化前使用精确值脱敏。

## 数据库

当前产品以全新基线 Migration `000001_agent_workspace.sql` 建库，后续修正只通过不可变的追加式 Migration 演进；`000005_model_provider_connections.sql` 将早期 Model Profile 数据清空并替换为 Model Provider Connection、Provider Model 与版本化凭证结构，后续 Migration 删除模型类型字段。Provider Model 优先来自供应商 `/models`，失败或不支持时使用平台维护的厂商默认列表，User 也可显式补充。从旧企业控制面切换前必须备份并重建业务数据库；不支持把旧 Organization/Team/Agent Release 数据猜测性映射为新 User 私有数据。
