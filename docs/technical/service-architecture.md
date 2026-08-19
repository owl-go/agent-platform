# 服务端领域架构

## 目标结构

后端采用 Go Kratos 的全局职责分层，并在每层内部保留限界上下文。业务边界不以 HTTP、数据库表或框架命名；依赖保持单向：

```text
Service / Server (Proto HTTP / Gin SSE / Worker)
              |
              v
              Biz
              |
              v
   Aggregate / Usecase / Port
              ^
              |
Data (GORM / Runtime CLI / Object Storage)
```

`backend/internal/biz` 定义聚合、实体、值对象、领域规则、Repository 端口和用例；`backend/internal/data` 实现端口；`backend/internal/service` 负责 Proto/HTTP 转换、身份与授权调用及公开错误映射；`backend/internal/server` 负责 Kratos 生命周期。`backend/cmd/api` 与 `backend/cmd/worker` 只加载严格配置、调用 Wire Injector 并运行 App。GORM Model、YAML Config 和 HTTP DTO 不得作为领域实体复用。

## 限界上下文

| 上下文 | 当前核心模型 | 代码位置 | 状态 |
|---|---|---|---|
| Execution | Run 聚合、Attempt、Run Lease、Run Event | `backend/internal/biz/execution`, `backend/internal/data/execution` | Phase 1：领取、续租、恢复、查询与 Worker 编排已实现 |
| Runtime | Agent Runtime、Runtime Adapter、Capability、标准 Event Contract | `backend/internal/agentruntime`, `backend/internal/runworker` | Phase 0 |
| Runtime Catalog | Runtime Image、不可变 Digest、Capability、生产状态与封禁 | `backend/internal/biz/runtimecatalog`, `backend/internal/data/runtimecatalog` | Phase 1：Biz、Data 与 Proto HTTP 已实现 |
| Model Catalog | Credential Profile、Configured Model、Secret Ref、凭证撤销 | `backend/internal/biz/modelcatalog`, `backend/internal/data/modelcatalog` | Phase 1：Biz、Data 与 Proto HTTP 已实现 |
| Source Control | GitHub.com、自建 GitLab、Git SSH 来源、Repository Binding | `backend/internal/biz/sourcecontrol`, `backend/internal/data/sourcecontrol` | Provider 与 Binding 已实现 |
| Workspace | Code Workspace、Sandbox、Workspace Snapshot、Write Lease | `backend/internal/sandbox`, `backend/internal/conformanceartifact` | Phase 0/骨架 |
| Artifact | Artifact、Object Key、保留与临时访问 | `backend/internal/biz/artifact`, `backend/internal/objectstore` | 授权列表、下载与保留清理已实现 |
| Run Approval | 高风险计划请求、审批决定、Run 等待状态 | `backend/internal/biz/approval` | 中立事务 Workflow、RBAC 与 Proto HTTP 已实现 |
| Webhook | Webhook Delivery、签名、重试与 Delivery Lease | `backend/internal/biz/webhook`, `backend/internal/data/webhook` | 投递循环与事务事件源已实现 |
| Agent Lifecycle | Agent、Agent Draft、Validation、Release Approval、Agent Release | `backend/internal/biz/agentlifecycle`, `backend/internal/data/agentlifecycle` | Biz、Data 与 Proto HTTP 已实现 |
| Collaboration | Coding Task、Session、Session Memory、Memory Candidate、Agent Memory | `backend/internal/biz/collaboration`, `backend/internal/data/collaboration` | Proto HTTP、Workspace Write Lease 与 Git Workflow 已实现；真实外部 Git/Runtime 联调后置 |
| Identity & Governance | Organization、Team、User、Role Grant、Audit | `backend/internal/biz/identity`, `backend/internal/biz/audit` | RBAC、范围查询、审计、OIDC Token 验证与当前 User 引导已实现 |

未实现上下文不会因为数据库中已有预留表就视为已交付。实现新能力时先在对应上下文建立 Domain 和 Application，再增加 HTTP/GORM 等 Adapter。

## Execution 聚合边界

`Run` 是聚合根，负责以下不变量：

- Run 状态只能按领域状态机转换；终态不可继续推进。
- Claim 只允许从 `queued` 或 `resuming` 进入 `provisioning`，并产生唯一的新 Attempt。
- 一个 Run 同时最多有一个有效 Run Lease；续租和推进必须持有未过期 Token。
- 完成结果只能为 `completed`、`failed` 或 `cancelled`，Usage 与 Error 必须是合法 JSON，模型成本必须是非负十进制金额。
- 过期 Lease 只在已知基础设施故障语义下进入 `resuming`；达到 Attempt 上限后进入 `failed`。
- 聚合状态、Attempt、Run Lease 与对应 Run Event 在同一 PostgreSQL 事务中提交。

`backend/internal/biz/execution/domain` 不依赖 GORM。`backend/internal/biz/execution/application` 通过 Repository 端口执行用例。`backend/internal/data/execution/gormrepo` 使用 GORM Transaction 和 PostgreSQL `FOR UPDATE SKIP LOCKED` 实现原子领取与 Reconcile。

Application Worker 统一编排 `Claim -> MarkRunning -> Renew -> Finish`。租约丢失时取消 Runtime Processor，且不再提交终态，由 Reconciler 决定安全恢复或失败；Runtime Processor 返回的内部错误不会原样写入持久化终态。

Run 控制命令使用 Version 乐观锁和 Idempotency Key。Interrupt 将活跃 Run 切换到 `interrupting`；Worker 在最长五秒的续租检查周期内取消 Runtime，随后确认 `interrupted` 并释放 Run/Workspace Lease。Resume 只允许 `interrupted -> resuming`，由普通 Claim 创建新 Attempt。Cancel 和 Operator Kill 立即提交终态、取消活跃 Attempt 并撤销 Lease；Kill 额外记录安全的 `operator_killed` 终态错误。Agent User、Agent Builder 和 Platform Administrator 可协作式 Interrupt/Resume/Cancel；Run Operator 可 Interrupt/Cancel/Kill，但不能代替用户 Resume。

Run Approval 与 Release Approval 是两个不同概念。前者绑定单个 Run 的高风险计划或变更请求：创建 Pending Approval 与 `running -> waiting_confirmation` 在同一事务提交，批准后恢复 `running`，拒绝后以 `approval_rejected` 终止 Run、取消 Attempt 并撤销 Lease。只有 Agent User、Agent Builder 和 Platform Administrator 可申请或决定；Run Operator 只保留运行控制职责。Runtime Adapter 只有在确实暂停风险动作后才能发起该请求，Runtime 事件到此应用用例的适配随真实四 Runtime 联调完成。

## 身份与授权边界

HTTP 只接受 Bearer Token，并通过 `backend/internal/biz/identity/application.TokenVerifier` 端口获得已经验证的 OIDC Subject 与 Organization Slug。部署可以选择显式 `deny_all` 或严格 YAML 配置的通用 OIDC Adapter；后者通过 Provider Discovery 和 JWKS 验证签名、Issuer、Audience、有效期、Subject 与 Organization Claim。JWKS 暂时不可用与未知签名 Key 分别映射为基础设施不可用和未认证，不使用可伪造的组织 Header 或开发后门。`GET /v1/me` 根据已验证身份从 PostgreSQL 引导 User、Organization、Role Grant 与可访问 Team 的安全投影；Organization 范围 Grant 可见本 Organization 的全部 Team，Team 范围 Grant 只投影对应 Team。

Run 读取同时校验 Organization 和 Team 范围。Organization 级 Role Grant 覆盖该 Organization 内的 Team，Team 级 Role Grant 只覆盖对应 Team；跨 Organization 一律拒绝。Runtime Image、Model Catalog、Source Control Provider 与 Repository Binding 写入只允许 Organization 级 Platform Administrator，Team 级管理员不能修改这些治理配置。

## Runtime Catalog 聚合边界

Runtime Image 属于一个 Organization。读取、注册、状态治理和被 Agent/Repository Binding 引用时都必须使用同一个 Organization 范围；同一 Repo Digest 可以分别注册在不同 Organization，但不得跨 Organization 可见或复用。注册后 Runtime、CLI Version、Adapter Version、Capabilities 与镜像 Repo Digest 不可变；更新必须注册新镜像。可变字段仅为 `experimental`、`production`、`blocked`、`deprecated` 状态和 Blocked Reason，并使用 Version 乐观锁。Deprecated 为不可逆终态；Blocked 必须携带原因。

Runtime Image 只有在进入 `production` 时关联并验证对应的逻辑 Conformance Evidence Object Key，才能称为 Production Runtime；仅注册、声明 Capability 或提交任意 Object Key 不代表已通过验证。服务端从配置的 MinIO 或阿里云 OSS 读取 `application/x-tar` Evidence，核验对象 Size、SHA-256 和 `artifact-kind=production-conformance`，再解析唯一的 `scenario-summary.json`；其中 Runtime、镜像 RepoDigest、CLI Version、Capability、强杀恢复、Interrupt、Cancel、Timeout、Review Branch 及 MinIO/阿里云 OSS Snapshot 必须与注册记录和 Production Conformance 契约一致。验证通过后同时保存逻辑 Object Key 与当时对象内容的 SHA-256，形成不可变证据快照；不保存 Provider URL 或签名参数。Blocked 和 Deprecated 会保留此前的证据引用与摘要供审计，界面将其显示为“已记录证据”而非“无证据”。

Runtime Image 列表按 Runtime、注册时间倒序和 ID 提供确定性排序，并通过不透明 `page_token` 分页；默认每页 20 条，单页最多 100 条。注册请求必须声明 `Idempotency-Key`，状态变更同时声明 `Idempotency-Key` 与 `If-Match`，这些 Header 是生成 OpenAPI 契约的一部分。

从旧版 Runtime Catalog 升级时，如果数据库已有 Runtime Image 且存在多个 Organization，平台不会猜测归属；如果已有 `production` 记录，也不会伪造证据。运维人员应先停写并备份数据库，再执行以下可审计的预迁移步骤，逐行填写真实 Organization 与已验证 Evidence，确认查询返回零行后再启动新版 API。Migration 使用 `ADD COLUMN IF NOT EXISTS`，因此失败后可按同一路径修复并安全重试：

```sql
ALTER TABLE runtime_images
  ADD COLUMN IF NOT EXISTS organization_id uuid REFERENCES organizations(id),
  ADD COLUMN IF NOT EXISTS conformance_evidence_key text,
  ADD COLUMN IF NOT EXISTS conformance_evidence_sha256 text;

-- 按企业自己的归属清单逐行执行；禁止用任意默认 Organization 批量代填。
UPDATE runtime_images
SET organization_id = '<verified-organization-uuid>'
WHERE id = '<runtime-image-uuid>' AND organization_id IS NULL;

-- 仅填写已由 Production Conformance 验证的逻辑 Key 与对象内容 SHA-256。
UPDATE runtime_images
SET conformance_evidence_key = '<verified-object-key>',
    conformance_evidence_sha256 = '<lowercase-64-char-sha256>'
WHERE id = '<production-runtime-image-uuid>' AND status = 'production';

SELECT id, status FROM runtime_images
WHERE organization_id IS NULL
   OR status = 'production' AND (conformance_evidence_key IS NULL OR conformance_evidence_sha256 IS NULL);
```

Credential Profile 只保存符合 URI 形式的 Secret Manager 引用。Model Catalog 的读取接口只投影 Organization Scope、类型为 `model` 的 Credential Profile，避免 Team 范围凭证元数据跨 Team 暴露。Configured Model 必须绑定同 Organization、Organization Scope、类型为 `model` 且已启用的 Credential Profile；注册或从禁用状态重新启用 Configured Model 时，Repository 会锁定对应 Credential Profile 并在写入前重新检查这个约束，使并发禁用与模型写入串行。禁用 Credential Profile 会在同一个数据库事务内禁用引用它的 Configured Model；重新启用凭证不会自动重新启用模型。

Repository Binding 保存 Git SSH 地址以及 SSH/Build Credential Profile 的安全引用，不保存或返回私钥、`known_hosts` 内容和构建 Secret。注册与更新先校验 Organization/Team、凭证 Kind/Scope、Runtime、模型和 Provider 引用；显式 Validation 会按当前依赖状态重新检查 Provider、仓库 Host、Credential、Production Runtime、Required Runtime Capabilities、Configured Model、Model Budget 和结构化质量命令，并以字段为 Key 保存可定位的 Validation Report。每个 Allowed Runtime Image 都必须提供 Binding 声明的全部 Required Runtime Capabilities。配置更新清除旧报告，依赖禁用后再次验证会如实变为失败。Agent Draft 对所选 Runtime 的额外 Capability 要求及其相对 Repository Binding 的预算收紧由 Draft Validation 继续校验。

## 幂等写事务

所有 POST/PATCH 要求 `Idempotency-Key`，可变资源额外要求 `If-Match` Version。`backend/internal/data/controlplane/gormuow` 在同一个 PostgreSQL Transaction 内提供事务作用域服务；Coding Task Launch/Continue、Run Completion 与 Run Approval 由 `backend/internal/biz/workflow` 端口和 `backend/internal/data/workflow/gormtx` 适配器协调：

1. 通过 Organization、Key、Operation 获取行锁；
2. 校验原始请求 Body 与 Version 的 SHA-256；
3. 执行对应限界上下文 Application Service；
4. 保存 HTTP Status 和脱敏后的 JSON Response Snapshot；
5. 追加不含请求/响应正文的 Audit Event，并在配置 Webhook 时创建待投递的安全元数据事件；
6. 一次提交 Key、领域变更、Audit Event、Webhook Delivery 和响应。

并发相同请求只有一个 Handler 执行，其余请求重放完全相同的持久化响应。相同 Key/Operation 配合不同请求哈希返回 Conflict；领域事务失败时 Key 占位也回滚。

## 配置边界

API 与 Worker 使用同一份严格 YAML Schema，由 `backend/internal/conf` 经 Kratos Config Source/Load/Scan 接入，并复用 `backend/internal/platformconfig` 的 fail-closed 校验。启动流程会拒绝：

- 未知字段；
- 未设置的 `${ENV_VAR}`；
- 空 DSN、空监听地址；
- 非正数 Timeout、Reconcile Interval 或 Attempt 上限；
- 不一致的数据库连接池上下限。

Secret 不提交到 YAML；部署 YAML 只保存 `${ENV_VAR}` 占位符。MinIO 与阿里云 OSS 使用不同的部署 YAML，Object Store Provider 仍通过统一领域端口供上层使用。

## Webhook 投递边界

Webhook Delivery 是持久化投递单元。Worker 使用 PostgreSQL `FOR UPDATE SKIP LOCKED` 原子领取并持有 Delivery Lease；崩溃后由后续 Worker 回收过期 Lease。非 2xx 响应按指数退避重试，达到上限后进入 `cancelled`，不会无限重试。

目标必须是无 User Info 的 HTTPS URL。请求体保持领域事件 JSON 的原始字节，签名为 `HMAC-SHA256(secret, timestamp + "." + payload)`，通过 `X-Agent-Platform-Timestamp` 和 `X-Agent-Platform-Signature: sha256=<hex>` 传递；Delivery ID 和 Event Type 使用独立 Header。HTTP 响应 Body 不写入错误或日志。

Webhook Worker 默认关闭，启用时严格校验 HTTPS `target_url`、请求超时、Delivery Lease、退避上限、Attempt 上限和至少 32 字节的环境注入 Signing Secret。Control Plane 幂等写入已在同一事务内创建 Audit Event 和 Webhook Delivery；幂等重放不重复创建。Task、Run 等后续上下文仍必须在各自业务事务中接入事件源，不能仅因投递器存在而视为已交付。

## Web 产品边界

`frontend` 是独立的 Vue + TypeScript Interface Adapter，提供 Agent Studio、Conversation Workspace 和 Operations Console 三个产品界面。前端使用生成的 OpenAPI 类型，不复制服务端领域模型，也不承担授权、幂等或状态机不变量。

页面只在未认证状态调用无身份的健康接口；受保护界面在 OIDC Authorization Code + PKCE 完成并且 `GET /v1/me` 成功前不会渲染。OIDC 状态与 Token 仅保存在浏览器 `sessionStorage`，刷新时重新引导当前 User，Token 过期或退出后立即隐藏受保护界面。Agent Studio、Conversation Workspace 与 Operations Console 使用真实路由，active Team 通过 `team` 查询参数从启动上下文 allowlist 中选择，不使用 Team Header。切换 Team 会通过带 Team Key 的路由视图销毁旧页面状态。展示边界支持 `zh-CN` 与 `en-US`，语言偏好可写入 `localStorage`，但 Token 和 Secret 不得写入。导航可按 Role Grant 提示可用能力，所有业务操作仍由服务端根据 Organization、Team 和 Role Grant 做最终授权。

## Retention 边界

Retention Worker 按严格 YAML 策略分批清理过期 Run Event、Artifact、Audit Event 和 Idempotency Key。Artifact 遵循“先删除 Object Store 对象，再软删除 PostgreSQL 元数据”的顺序；对象删除失败时保留元数据以便下次重试，对象已经不存在则按幂等成功处理。

默认配置关闭 Retention，防止开发环境或未完成对象存储验证的部署误删数据。启用时默认保留普通 Run Event 与 Artifact 90 天、安全与授权 Audit 一年；每轮批量上限为 500，避免长事务影响 Run 调度。`workspace_snapshot` Artifact 与 Session Docker Volume 按 Coding Task 关闭时间单独保留 30 天，删除成功后记录 `workspace_purged_at`；Volume 名称不符合平台 UUID 命名规则时拒绝执行 Docker 删除。

## Artifact 边界

Runtime stdout/stderr 先以 Attempt 唯一对象键写入 Object Store，再在 PostgreSQL 创建 Artifact 元数据；元数据提交失败时补偿删除刚上传的对象，避免无法授权和追溯的孤儿数据。重试 Attempt 不覆盖前一次输出。Artifact REST 只返回 ID、Run、Kind、大小、校验和、Content Type、安全 Metadata 和保留时间，不暴露 Provider Object Key；下载前按所属 Run 重新授权，并只签发五分钟访问地址。

## Agent Lifecycle 聚合边界

Agent 是 Team 范围的稳定身份；Agent Draft 是可编辑、可验证的版本；Agent Release 是发布时冻结的不可变快照。Agent Studio 的 Agent Catalog、Agent 详情以及 Draft 列表和详情都通过 Team-scoped API 读取；服务端同时使用认证身份的 Organization 和请求 Team 查询聚合，使跨 Team 查询与不存在资源返回相同的 Not Found 语义。Draft 表单中的 Repository Binding、Runtime Image 与 Configured Model 来自各自的真实目录 API，不使用浏览器内置样例。

Agent 和 Draft 创建、Draft 编辑与验证都要求 `Idempotency-Key`；编辑和验证还要求 `If-Match` 传递期望 Version。相同意图重试复用 Key，而输入或 Version 变化形成新意图。Draft 每次编辑都会递增 Version、回到 `draft` 状态并清除旧 Validation Report；Version 不匹配返回显式 Precondition Failed，浏览器保留安全表单输入并重新加载权威 Version，不静默覆盖。

Draft Validation 使用当前 Repository Binding、Runtime Image 和 Configured Model 投影生成字段级 Validation Report。它会检查 Binding 的最新 Validation Report（其中包含 Git SSH、Credential、Egress、Required Runtime Capabilities 与质量命令结果）、Runtime allowlist 与 Production 状态、Configured Model 策略与启用状态、Draft 相对 Binding 收紧的 Model Budget，以及原生 Subagent 所需 Runtime Capability。发布时不仅检查此前验证结果，还会重新执行同一依赖验证，避免依赖在验证后发生变化。

低风险 Draft 验证成功后可直接发布。启用 Runtime Subagent 等高风险能力时必须先申请 Release Approval，由另一名具有 Agent Builder 权限的人员决定；审批绑定精确 Draft Version，申请人不能自批，Draft 编辑后旧审批不会授权新版本。

同一 Draft 只能产生一个 Agent Release。Release 内容不可修改，仅允许使用 Version 乐观锁进入 `deprecated` 或 `blocked` 状态；Blocked 必须记录原因。Agent Builder 可完成常规生命周期操作，Release Block 仅允许 Organization 级 Platform Administrator。
