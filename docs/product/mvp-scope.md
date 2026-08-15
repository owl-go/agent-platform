# Agent Platform MVP 范围

## 目标结果

企业内部用户可以把编码任务委托给四种 Agent Runtime 中任意一种，跨多个可恢复 Run 持续协作，并得到通过标准 Git SSH 推送、经过测试的 Review Branch。

## MVP 包含

### 产品界面

- 平台配置：Runtime 镜像、Adapter、模型、Endpoint、凭证、仓库和限制
- Agent Studio：Draft、验证、风险审批、Release 和 Deprecated
- Conversation Workspace：任务、消息、事件、Diff、成本、审批、运行控制和任务完成
- Operations Console：Run 检索、诊断、基础设施重试和 Kill

### 执行能力

- Claude Code CLI、Hermes Agent、OpenClaw 原生 Agent 和 Codex CLI
- 每种 Runtime 一个不可变 Docker 镜像
- 专用 Linux Worker 上的 Docker + gVisor Sandbox
- PostgreSQL 队列、Lease 与 Reconciler
- Interrupt、Resume、Cancel、超时和安全的基础设施重试
- 统一 Object Storage Provider，并支持阿里云 OSS 与 MinIO
- Artifact 与 Workspace Snapshot 在两种对象存储实现上保持一致行为

### 编码流程

- 通过标准 Git 支持 GitHub.com 和自建 GitLab
- 配置仓库 URL 和 SSH 私钥
- 每个 Coding Task 一个 Session 和 Review Branch
- 多 Run 协作与单 Workspace 写者
- 允许修改仓库内全部文件
- 高风险变更计划审批
- 仓库质量命令与 Secret 扫描
- Commit 并 Push Review Branch
- 用户手工创建、审查和合并 PR/MR，并手工完成 Task

### 模型与 Memory

- 管理员配置 Model Endpoint 和平台凭证
- Runtime 直接调用 Endpoint
- 每个 Run 冻结 Model Binding
- Token 与金额 Model Budget
- Working Memory、Session Memory 和用户确认的 Agent Memory
- 包含在一个 Run 内的 Runtime 原生 Subagent

### 安全与运维

- 部署时选择企业 OIDC Provider
- 企业与团队 RBAC
- 非 root、只读根文件系统的 gVisor Container
- 允许任意公网，但拒绝私网和控制面
- 临时凭证注入与脱敏
- 标准事件、SSE 重放、REST、Webhook 和 OpenAPI
- Audit、Retention、Cost、Log 与 Runtime Capability 可见性

## MVP 不包含

- 平台级多 Agent、Child Run、委派图和结果仲裁
- User Memory 与 Tenant Knowledge
- Agent Marketplace
- 平台级 Prompt、MCP 和 Skill Registry
- GitHub/GitLab API 集成
- 自动 Issue 同步、PR/MR 创建、Review 同步或 Merge
- 仓库数据分类与模型供应商合规验证
- Model Gateway、跨 Provider 路由和自动模型回退
- Evaluation Platform、Canary、灰度发布和自动回滚
- Kubernetes、Temporal、Redis、Kafka、NATS 或其他消息总线
- 用户提供任意 Runtime 镜像
- 外部多租户计费与数据驻留控制

## 实施阶段

### Phase 0：Runtime 与隔离 Spike

验证四种 Runtime 在 gVisor 中运行、Adapter 标准事件、取消、重建、Secret 脱敏和 Review Branch Push。

### Phase 1：控制与执行骨架

交付 PostgreSQL Schema、Go API、Go Worker、Lease/Reconciler、Sandbox 生命周期、模型与仓库凭证、阿里云 OSS/MinIO Provider 和 Event Stream。

### Phase 2：Agent 生命周期

交付 Agent Draft、验证、风险审批、不可变 Release、Repository Binding 和 Runtime/Model 选择。

### Phase 3：编码闭环

交付 Coding Task、Session Memory、多 Run Workspace、审批、Diff、质量门禁、Git Commit 和 Review Branch Push。

### Phase 4：产品界面

交付 Vue Agent Studio、Conversation Workspace 和 Operations Console。

### Phase 5：生产门禁

验证容量目标、故障恢复、Retention、Audit、Model Budget、Agent Memory、安全不变量和全部 MVP 验收标准。
