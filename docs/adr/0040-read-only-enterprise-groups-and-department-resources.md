---
status: accepted
---

# 使用只读身份群组约束部门资源和预算

企业治理不能靠 Administrator 公共资源冒充部门协作，也不能在 Agent Workspace 内维护一套会与企业目录漂移的组织树。平台从 Keycloak 只读同步 Group、成员关系和显式 `agent_workspace_department=true` 标记；本地记录只保存授权投影、Department Credit Budget、同步时间和 Version。同步是权威快照：身份源中消失的 Group 立即变为 inactive，资源访问与 Workflow 绑定都要求当前 active membership。平台不向 Keycloak 写 Group 或成员。

保留唯一且不可降级的 Bootstrap Administrator，同时允许委派多个 Administrator 和 Resource Publisher。Administrator 管理账号、角色、同步和预算，但其角色不会绕过 User-private 或 Department membership 边界。Resource Publisher 只能在自己当前所属的 Department 创建和维护 Department Resource，不能管理其他部门或私人内容。首个落地的 Department Resource 是 Knowledge Base；这不是对 Expert、Skill、Connector 或 Workflow 模板已实现部门共享的声明。

Knowledge Base 具有创建后不可变的 `private`、`group` 或 `platform` scope。`group` scope 绑定一个 Department：成员可读，成员中的 Resource Publisher 可写。跨范围与不存在资源使用相同 Not Found 语义。离职移交要求源 User 已停用，接收者启用且是该 Department 当前的 Resource Publisher；源 User 可已从最新 membership 快照移除，事务以其实际持有的 scoped resource 为边界，只更新该 Department Knowledge Base 的 custody。同名冲突时 fail closed，不读取或转移 Session、Workflow、文件或 private Knowledge Base。

Department Credit Budget 是所有当前成员的每日聚合准入上限，不是新的余额或账单。文本和图片准入以 Group + Credit Day Advisory Lock 串行计算已结算 consumption 与活动 reservation；多个适用预算取剩余最小者并限制 User Available Credit。实际结算仍进入 User Credit Ledger，现有实际用量超过 fallback reservation 时产生负余额的语义不变。

所有治理写操作要求 reason，并追加只含 actor、action、target identifier、时间和有界数值的 Governance Audit Event。审计不得包含提示词、回答、文件名、Object Key、Secret、外部 Token 或私有资源内容。真实 Keycloak 规模、生产身份映射、设计伙伴权限演练和生产审计导出仍需单独证据；本决策只固定授权与隐私边界。
