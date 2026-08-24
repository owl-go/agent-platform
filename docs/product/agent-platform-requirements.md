# Agent Platform 产品需求

状态：已确认  
产品：企业内部 Coding Agent Platform  
初始规模：50 名内部用户、10 个并发 Run

## 1. 产品目标

建设一个供企业内部团队配置、发布、使用和运维 Coding Agent 的平台。第一条完整业务链路从用户提交编码任务开始，以 Agent 完成代码修改、验证并将 Review Branch 推送到 GitHub.com 或自建 GitLab 结束，最终代码审查与合并由用户负责。

平台必须让 Agent 执行具备可复现、可观测、可中断、可恢复、成本可控和环境隔离能力。

## 2. 用户角色

### Platform Administrator

配置企业身份、Agent Runtime 镜像、Runtime Adapter、模型、Endpoint、凭证、平台限制和运行策略。

### Agent Builder

创建 Agent Draft，选择 Runtime 和模型策略，将 Agent 绑定到仓库，执行验证并发布 Agent Release。

### Agent User

创建 Coding Task，与 Coding Agent 多轮协作，审批高风险操作，观察 Run，审查代码变更并决定任务是否完成。

### Run Operator

检索和诊断 Run，查看日志与成本，中断或终止执行，并重试符合条件的基础设施故障。

角色可授予在企业或团队范围。不同团队默认不能互相查看 Session、代码、Artifact、Agent Memory 和凭证。

## 3. 产品原则

1. 只有不可变的 Agent Release 可以启动 Run，Draft 永远不能直接执行。
2. Run 启动时冻结 Agent Release、Runtime 镜像、Model Binding、Repository Binding、凭证和限制。
3. Sandbox 是临时执行环境，不是事实源。
4. Runtime 差异通过 Runtime Capability 明确声明，不假设四种 Runtime 完全等价。
5. 人类负责最终代码审查、PR/MR 创建和合并。
6. 公网访问不能暴露企业私网和平台控制服务。
7. 模型成本属于产品预算；计算与网络边界属于平台安全限制。
8. Runtime 原生 Subagent 保持在一个 Run 内，不能扩大该 Run 的权限和预算。

## 4. 核心领域模型

### 4.1 Agent 与 Release

Agent 可以跨仓库复用。Agent Builder 编辑 Agent Draft，验证通过后创建不可变 Agent Release。

```text
Draft -> Validating -> Ready -> Released -> Deprecated
                    \-> Blocked
```

低风险 Agent 可由 Agent Builder 自助发布；具有高风险能力的 Agent 需要另一位授权人员审批。紧急凭证撤销、Runtime 镜像封禁和平台 Kill Switch 可立即作用于已发布 Agent，但不修改历史 Release。

### 4.2 Repository Binding

Repository Binding 负责将可复用 Agent 配置到一个仓库，至少包含：

- Git 仓库地址与默认目标分支
- SSH 私钥引用
- Git Commit 作者名称和邮箱
- 允许的 Agent Runtime 与默认 Runtime
- Model Policy 与默认 Model Budget
- Agent 指令和仓库指令
- 构建、格式化、静态检查与测试命令
- Egress Policy 与构建 Credential Profile 引用
- 当前配置版本与验证结果

Repository Binding 更新只影响之后创建的 Run。

### 4.3 Coding Task、Session、Run 与 Attempt

一个 Coding Task 对应一个 Session。Session 固定 Repository Binding、目标分支和 Review Branch。用户每次要求 Agent 继续、修正或验证都会创建新 Run；基础设施重试在同一个 Run 中创建 Attempt。

Coding Task 状态：

```text
Created -> Active <-> WaitingForUser -> Completed
                              \------> Cancelled
```

Run 状态：

```text
queued
provisioning
running
waiting_confirmation
interrupting
interrupted
resuming
recovery_required
completed
failed
cancelled
```

Run 失败不会自动结束 Coding Task。Interrupt 保留可恢复状态；自动基础设施重试耗尽后进入非终态 `recovery_required`，只有 Run Operator 或 Organization 级 Platform Administrator 可记录安全原因并恢复。用户错误、策略错误和已经产生终态 Event 的 Run 不可恢复；Cancel 永久结束当前 Run，但用户仍可在同一 Session 创建新 Run。

### 4.4 Workspace 并发

只读 Run 可以并发执行。修改代码的 Run 必须获得 Session 的 Workspace Write Lease，平台必须阻止两个 Run 同时写同一个 Code Workspace。

## 5. 核心用户流程

1. Platform Administrator 配置 Runtime 镜像、Runtime Adapter、模型 Endpoint、API Key、仓库 SSH 私钥和平台限制。
2. Agent Builder 创建 Agent Draft，配置指令、Runtime、模型、仓库、Model Budget 和质量命令。
3. 平台验证 Draft 并发布不可变 Agent Release。
4. Agent User 提交自由文本编码需求，或提交包含标题、正文和可选链接的 Issue Snapshot。
5. 平台创建 Coding Task、Session、Review Branch 名称和首个 Run。
6. Go Worker 领取 Run，创建或恢复 Code Workspace，在 Docker + gVisor Sandbox 中启动所选 Runtime 镜像。
7. Coding Agent 分析仓库；策略要求时先展示计划并等待审批；随后修改文件并执行验证命令。
8. Conversation Workspace 流式展示标准 Runtime Event、命令、文件变化、Diff、审批、错误和模型成本。
9. 质量门禁通过后，Agent Commit 并通过 Git SSH 推送 Review Branch。
10. Coding Task 进入 `WaitingForUser`，用户在平台外自行创建和审查 PR/MR。
11. 用户选择继续创建 Run、完成任务或取消任务。

## 6. 功能需求

### 6.1 平台配置

- 注册、更新、禁用和查看每种 Runtime 的独立镜像与 Adapter。
- 注册模型、Endpoint 和平台统一管理的 API 凭证。
- 注册仓库地址、默认分支、SSH 凭证和 Git 身份。
- 配置 Object Storage Provider，首期支持阿里云 OSS 和 MinIO。
- 配置企业与团队角色、Model Budget 上限和 Execution Limit。
- 封禁 Runtime 镜像、模型、凭证、Agent Release 或活跃 Run。

### 6.2 Agent Studio

- 创建和编辑 Agent Draft。
- 配置指令、Runtime、模型策略、Repository Binding、Model Budget 和发布风险。
- 验证 Runtime、模型、仓库、凭证和质量命令配置。
- 展示字段级错误和 Validation Report。
- 完成所需审批后发布不可变 Agent Release。
- Deprecated Release，同时保留历史可追溯性。

### 6.3 Conversation Workspace

- 从用户文本或 Issue Snapshot 创建 Coding Task。
- 在多个 Run 之间保持同一个 Session。
- 展示消息、计划、命令事件、文件变化、Diff、验证结果、审批、错误和模型成本。
- 允许用户批准或拒绝风险操作。
- 允许用户 Interrupt、Resume 或 Cancel Run。
- 验证通过后 Push Review Branch。
- 允许用户明确选择继续、完成或取消 Coding Task。
- 如实展示 Runtime Capability 差异，不伪造一致体验。

### 6.4 Operations Console

- 按团队、Agent、Repository Binding、Coding Task、状态、Runtime 和时间检索 Run。
- 展示 Run/Attempt 时间线、Runtime 镜像 Digest、Model Binding、日志、错误、Usage、成本和 Sandbox 生命周期。
- Interrupt 或 Kill 活跃 Run。
- 只重试符合条件的基础设施故障。
- 查看经过脱敏的审计事件与 Artifact 引用。

### 6.5 API 与事件

- 提供 Agent、Release、Repository Binding、Coding Task、Session、Run、Approval、Memory 和 Artifact 的 REST API。
- 提供支持游标重放的 Run SSE 标准事件流。
- 提供 Task、Run、Approval 和终态变化的签名 Webhook。
- 发布可生成 Go 与 TypeScript Client 的 OpenAPI 契约。
- 所有写接口支持 Idempotency Key；可变资源支持乐观版本控制。

## 7. Agent Runtime

首批 Production Runtime：

- Claude Code CLI
- Hermes Agent
- OpenClaw 原生 Agent 模式
- Codex CLI

OpenClaw 不得在平台 Run 内再次代理 Claude Code 或 Codex。每个 Run 只选择一种 Agent Runtime。

每种 Runtime 使用独立、不可变的镜像。Agent Release 固定 Runtime、Adapter 版本、CLI 版本和镜像 Digest。任一组件更新都产生新镜像；除非被安全策略封禁，已有 Run 恢复时继续使用原镜像。

### 7.1 Runtime Adapter 契约

所有 Runtime Adapter 必须支持：

- 非交互启动
- 标准完成与失败结果
- 取消与超时
- 进程退出状态
- Code Workspace 挂载
- 命令与文件变化观测
- 最终结果与 Diff 收集
- 凭证临时注入且不持久化
- 基于 Session Memory 和 Workspace Snapshot 的平台级重建

Adapter 通过 Capability 声明流式输出、原生 Resume、Subagent、结构化结果和详细 Usage 等可选能力。

### 7.2 Production Runtime 门禁

四种 Runtime 都必须通过真实 gVisor PoC，证明能够：

1. 非交互启动；
2. Clone 示例仓库；
3. 读取并修改代码；
4. 运行测试；
5. 产生标准完成/失败结果和 Diff；
6. 响应 Interrupt、Cancel、超时和强制终止；
7. Container 重建后继续产品流程；
8. 防止 API Key 和 SSH 私钥进入日志、Diff、Artifact 与 Snapshot；
9. Push Review Branch。

缺失的可选 Capability 必须准确声明并在 UI 中展示。

## 8. 模型与成本

Platform Administrator 注册 Configured Model、Endpoint 和 Credential Profile。模型一旦配置，即可处理任意 Repository Binding 的代码。MVP 不做仓库数据分类，也不验证模型供应商的数据保留、训练、地区或合规政策。

每个 Run 冻结 Model Binding。Runtime Container 直接调用已配置 Endpoint；MVP 不建设 Model Gateway、跨 Provider 路由或自动模型回退。

Model Budget 只覆盖 Token 和模型金额。预算从平台、团队、Agent Release、Repository Binding 到 Run 逐层收紧。用户只能降低继承上限。达到软阈值时要求 Agent 收尾；达到硬阈值时 Interrupt Run。

CPU、内存、磁盘、进程数、运行时长、并发和网络属于 Execution Limit，不属于 Model Budget。

## 9. Memory 与 Subagent

MVP 包含：

- Working Memory：一个 Run 内的计划和中间状态；
- Session Memory：一个 Coding Task 跨 Run 的消息、摘要、确认决策、结果和 Workspace 引用；
- 基础 Agent Memory：Agent 跨 Coding Task 复用的稳定经验。

Agent 推断先形成 Memory Candidate，只有用户确认后才能成为 Agent Memory。用户可以查看、修改、删除和禁用 Agent Memory。

MVP 不包含 User Memory 和 Tenant Knowledge。源代码正文、Secret 和模型私有推理不得写入 Agent Memory。

Runtime 原生 Subagent 共用父 Run 的权限、Model Budget、Execution Limit、Code Workspace、Session Memory 和生命周期。MVP 不暴露 Child Run，也不建设平台级多 Agent 编排。

## 10. Git

- 通过标准 Git Transport 支持 GitHub.com 和自建 GitLab。
- 使用 Repository Binding 中的 SSH 私钥引用进行 Clone 与 Push。
- 每个 Coding Task 使用一个稳定 Review Branch。
- Commit 使用平台配置的 Git 作者；审计数据记录 Agent Release、Run 和任务发起人。
- 永远不直接写目标分支，也不自动合并。
- 不依赖 GitHub/GitLab API、App、OAuth、Webhook、Issue 同步、Review 同步或自动 PR/MR。
- Issue 标题、正文和可选链接由用户提交，不从外部平台同步。
- Agent 可以修改仓库内任意文件，但高风险变更仍受计划审批、质量检查和 Secret 扫描约束。

## 11. 安全需求

### 11.1 Sandbox

- 每个写 Run 使用一个 Docker + gVisor `runsc` Container。
- 使用非 root 用户、只读根文件系统和最小 Linux Capability。
- 只有 Code Workspace 是可写业务存储。
- 禁止挂载 Docker Socket、宿主目录、平台数据库凭证、调度凭证和 Secret Manager 凭证。
- Runtime 镜像固定 Digest，禁止 Run 内升级 CLI。
- 限制 CPU、内存、磁盘、PID、时长、并发和网络。
- gVisor 或所需隔离属性不可用时 fail-closed。
- Container 到期后清理临时凭证、网络和存储。

### 11.2 网络

Sandbox 可以通过 Egress Policy 访问任意公共互联网。策略必须阻止宿主机、云元数据、loopback 逃逸、私网地址、平台控制服务和其他私有基础设施。访问私网必须获得显式临时授权。

网络层必须防止 DNS rebinding、重定向到私网和代理绕过，并审计网络访问与拒绝事件。

### 11.3 凭证

- 模型 Key、仓库 SSH Key 和构建凭证保存在 Secret Manager。
- 只向当前 Runtime 和 Run 注入必要凭证。
- 优先使用短期或用途受限的凭证。
- 从消息、命令、日志、Diff、Artifact、Agent Memory 和 Workspace Snapshot 中脱敏 Secret。
- Runtime 镜像不得包含凭证或登录状态。

### 11.4 不可信输入

Issue 文本、仓库文件、网页内容、依赖文档和工具结果都是不可信任务数据。它们不能覆盖平台 Policy、扩大凭证或网络权限、关闭审批或改变 Agent Release 限制。

### 11.5 推理隐私

平台记录状态、用户可见的计划与解释、命令、文件变化、Usage、错误和最终结果，不持久化或展示私有 Chain of Thought。

## 12. 可靠性与保留

### 12.1 Run 调度

Go Worker 通过 PostgreSQL Lease 领取排队 Run，Reconciler 回收过期 Lease 和孤儿 Container。只有可安全重试的已知基础设施故障可以自动创建 Attempt。

Agent 失败、测试失败、审批拒绝、Policy 拒绝、Model Budget 耗尽和结果不确定的外部副作用不得自动重试。

### 12.2 持久化

- PostgreSQL：配置、Task、Session、Run、Event、Budget、Memory 和 Audit。
- Object Storage Provider：大型日志、Artifact 和 Workspace Snapshot；部署时可选择阿里云 OSS 或 MinIO。
- Docker Volume：活跃 Code Workspace。
- Git Review Branch：最终代码交付。
- Sandbox 磁盘不得成为恢复所需状态的唯一副本。

Object Storage Provider 必须向上层提供一致的对象写入、读取、元数据、校验、删除、生命周期和临时签名访问语义。两种实现分别使用原生阿里云 OSS 接口和 MinIO S3-compatible 接口；Bucket、Region/Endpoint、Path Prefix、TLS 和 Credential Profile 均通过部署配置提供，应用代码不能依赖某一种 Provider 的专有对象地址。

对象存储 Bucket 默认不得公开。用户下载 Artifact 时由平台完成授权并生成短期访问地址。阿里云 OSS 与 MinIO 必须通过同一套 Provider Conformance Test，至少覆盖大文件分段上传、校验失败、对象不存在、临时地址过期、删除和生命周期清理。

### 12.3 默认保留期

- Runtime Container：Run 完成或 15 分钟保温结束后销毁。
- Workspace Revision：Task 关闭后保留 30 天。
- 普通 Run Event 与命令输出：保留 90 天。
- Artifact：保留 90 天，支持提前删除。
- 安全与授权审计：保留一年。
- Secret 明文：永不作为产品数据持久化。

## 13. 初始服务目标

- 50 名内部用户
- 10 个并发 Run
- Run 默认 30 分钟，最长两小时
- 每个 Session 最多 50 个 Run
- 排队到开始 P95 小于 60 秒
- Cancel 请求在 10 秒内开始终止执行
- 基础设施故障在五分钟内恢复或进入明确失败状态

## 14. MVP 验收标准

1. 非平台开发者可以配置、验证、发布和使用 Coding Agent。
2. 四种 Production Runtime 全部通过 Conformance PoC。
3. 用户可以提交任务、跨 Run 协作、查看 Diff 和成本，并得到已 Push 的 Review Branch。
4. Worker 或 Sandbox 故障后，Run 能恢复或明确结束，且不产生重复执行。
5. Interrupt、Resume、Cancel、Approval 和 Model Budget 状态一致。
6. Runtime 凭证不会进入持久化的用户可见数据。
7. 公网访问无法到达私有基础设施或平台控制服务。
8. 每个 Run 可追溯到 Agent Release、Repository Binding、Runtime 镜像、Model Binding、用户、事件、成本和 Artifact。
9. Agent Memory 经过用户确认，并支持查看、修改和删除。
10. Agent Studio、Conversation Workspace 和 Operations Console 展示一致的 Run 状态。
11. 阿里云 OSS 与 MinIO 均通过 Object Storage Provider Conformance Test，切换 Provider 不改变 Artifact 和 Workspace Snapshot 的产品行为。

## 15. 部署时确定的选项

需求阶段不固定具体 OIDC Provider、PostgreSQL 部署方式、Secret Manager、云厂商、Worker 数量、Runtime 版本和 Model Provider。对象存储限定支持阿里云 OSS 与 MinIO，由部署配置选择。实际部署选择必须满足本文要求。
