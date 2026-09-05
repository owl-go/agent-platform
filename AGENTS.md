# Agent Workspace 仓库指引

本文件是仓库级 AI 编码指引。目标是让改动建立在现有领域语言、模块边界和证据之上，而不是从目录名猜测系统已经具备的能力。

## 开始工作

1. 先读 `CONTEXT.md`，使用其中定义的领域术语；不要为同一概念创造近义词。
2. 根据“文档路由”只读取与任务相关的产品或技术规格，再查看对应接口、实现和相邻测试。
3. 先确认当前实现阶段和验收证据。规格中的目标能力不等于仓库已经实现或通过生产验证。
4. 在既有 seam 内完成最小改动，并同步修改测试和作为行为依据的文档。
5. 先跑目标包测试，再跑受影响层级的完整门禁。只有真实执行过的检查才能写入完成说明。

## 当前基线

- 仓库实现 Agent Workspace，目标行为以 `docs/product/agent-workspace-requirements.md` 为准；旧企业 Coding Agent 控制面已删除。
- 规格中的目标能力不等于已经实现或部署。只有真实执行过的测试与验收证据才能写入完成说明。
- Runtime 的可选 Capability 默认关闭；解析器能读出某字段，不代表该 Capability 已通过指定镜像 Digest 的 Conformance。

## 代码地图

| 路径 | 职责 |
|---|---|
| `frontend` | 独立 Vue 3 + TypeScript + Vite 产品界面 |
| `backend/cmd/api` | Wire 装配的 Kratos HTTP 控制面入口 |
| `backend/cmd/worker` | Wire 装配的 Kratos Worker 入口 |
| `backend/internal/biz/account` | User 与 Administrator 账号领域、OIDC 身份和用例 |
| `backend/internal/biz/workspace` | Session、Workflow、Run、Expert、Extension 与设置领域和用例 |
| `backend/internal/data/account` | Keycloak、Token Verifier 与账号 GORM Adapter |
| `backend/internal/data/workspace` | Agent Workspace GORM Repository 与 Runtime Executor |
| `backend/internal/infrastructure/gormdb` | GORM 连接池与不可变 PostgreSQL Migration 启动装配 |
| `backend/internal/platformconfig` | API/Worker 共用的严格 YAML 配置加载与校验 |
| `backend/internal/agentruntime` | Runtime 公共契约、四 Driver、Container Process 与脱敏事件 |
| `backend/internal/workspacefs` | Workflow Workspace 路径、配额和文件安全边界 |
| `backend/internal/skillstore` | Git/ZIP Skill 校验、规范化和对象存储 |
| `backend/internal/credentials` | 单次任务凭证物化、清理和精确字节脱敏 |
| `backend/internal/objectstore` | MinIO/阿里云 OSS 统一接口、校验与签名下载 |
| `deploy/runtimes` | 四个固定版本、非 root、单 Runtime 镜像及公共 Entrypoint |
| `deploy/sandbox` | Linux Worker 公网 Egress 网络策略配置 |
| `scripts/conformance` | Smoke、Linux Sandbox、存储和完整 Production Conformance 脚本 |
| `testdata/production-conformance` | 四种 Runtime 共用且彼此独立恢复的黑盒 Fixture |

## 文档路由

- **领域命名或模型关系**：读 `CONTEXT.md`。修改术语、实体边界或持久语义时同步更新它。
- **产品行为、账号、会话、工作流或界面边界**：读 `docs/product/agent-workspace-requirements.md`。
- **Runtime 契约、事件、错误或恢复**：读 `docs/technical/runtime-adapter.md`。
- **CLI 参数、镜像版本或 Capability**：读 `docs/technical/runtime-images.md`，同时检查对应 Driver、Dockerfile 和镜像测试。
- **Container 隔离、挂载、Egress 或 Reconcile**：读 `docs/technical/sandbox-runner.md`。
- **对象 Key、上传、签名 URL 或生命周期**：读 `docs/technical/object-storage.md`。
- **Runtime Conformance 或证据格式**：读 `docs/technical/production-conformance.md`；不存在新 Agent Workspace 验收证据时必须如实说明未验证。
- **架构决策**：读 `docs/adr`。改变已记录决策时新增 ADR 或明确取代旧 ADR，不要静默让实现与决策分叉。
- **服务端领域边界、分层或配置**：读 `docs/technical/service-architecture.md` 和 `docs/adr/0019-current-technology-stack.md`。

## 必须守住的边界

- Worker 只依赖 `agentruntime.Adapter` 的 `Describe` 和 `Execute`；Runtime 品牌差异留在各 Driver 内，共享行为留在 `cliadapter`、`processharness` 或公共契约层。
- 业务上下文按 Domain → Application 依赖；GORM Model、HTTP DTO 和 YAML Config 不能进入 Domain。聚合状态与其 Event 必须在一个 Repository 事务中提交。
- Runtime Adapter 不直接管理 Docker；Sandbox Runner 负责容器生命周期和隔离。Production Conformance 的 `containerprocess` 只是把公共进程 seam 接入容器。
- 外部命令以可执行文件和参数数组调用。用户输入通过受控 stdin、参数值或临时文件传递，不能拼接成 Shell 命令。
- Sandbox 必须 fail closed：只接受 Registry RepoDigest、`runsc`、非 root、只读 Rootfs、资源限制、受控挂载和明确的 Egress 策略；发现配置漂移就拒绝启动。
- Secret 只属于单个 Run 的临时环境。stdout、stderr、事件、Diff、Snapshot 和最终结果在持久化前必须经过同一组精确 Secret 值脱敏。
- `Event` 的 Run ID 必须匹配、Sequence 必须从 1 单调递增，并且恰有一个终态事件；取消或终态之后不得再发布事件。发布失败时停止执行。
- 对象存储上层只保存逻辑 Object Key，不保存 Provider URL 或签名参数。写入前必须验证 Size 和小写 SHA-256；Bucket 保持私有。
- Session 连续性依赖平台消息、Rolling Summary 和最近上下文。Runtime 原生 Resume 是经过验证后才能开启的优化，不是正确性的前提。
- Workflow Workspace 最多 Clone 一个公共 HTTPS 或私有 SSH Git Source；目标产品不包含 Commit、Push、Review Branch 或 PR/MR 流程。

## 修改模式

### Runtime 或 CLI 变更

保持 Driver 只含该 CLI 的构建与解析逻辑。更新固定版本时同时检查 Driver 的 `Version`、对应 Dockerfile、`docs/technical/runtime-images.md`、Driver 测试、共享 Adapter 契约测试和镜像 smoke test。新增或开启 Capability 必须附带对应 Digest 的黑盒验证证据。

### Runtime Contract 变更

从 `internal/agentruntime/contract.go` 开始，逐一检查 ContractSink、Runtime fake、Worker Runner、四个 Adapter、Conformance Runner 和技术规格。新增事件或错误分类时测试顺序、终态、取消和下游失败路径。

### Sandbox 或凭证变更

优先写拒绝不安全输入和配置漂移的测试。保持凭证不进入命令参数、镜像层或未经脱敏的 Artifact，并覆盖正常、取消、超时、失败和清理路径。macOS 测试不能替代 Linux + gVisor 证据。

### Object Storage 变更

上层代码依赖 `objectstore.Provider`，Provider 选择集中在 `providerfactory`。行为变化应先进入共享 Conformance Suite，再由 Memory、MinIO 和 Aliyun OSS 实现共同满足；远端集成测试缺少环境时会 Skip，不能把 Skip 记为通过。

### Web 变更

沿用 Vue Composition API 和 `<script setup lang="ts">`，保持桌面与移动端布局可用。组件行为放在 `.vue` 文件，当前全局视觉规则位于 `frontend/src/styles.css`；完成前至少运行 typecheck 和生产构建。

## 代码与测试约定

- Go 代码使用 `gofmt`；测试与实现相邻，优先使用表驱动或聚焦单一契约的测试，错误必须保留可用的 cause，并通过 `agentruntime.ErrorCode` 表达 Runtime 失败类别。
- 所有长操作接受并传播 `context.Context`；资源清理需要覆盖成功和失败路径，清理操作应尽量幂等。
- 在接口边界注入外部进程、存储和事件依赖，单元测试使用 fake/recording 实现，不调用真实外部服务。
- 安全逻辑采用 allowlist、路径校验和 fail-closed 默认值；相关测试同时覆盖合法路径与绕过尝试。
- TypeScript 保持严格类型检查，不提交 `dist`、`build`、`node_modules`、覆盖率或临时证据中的凭证内容。

## 验证

按改动范围从小到大执行：

```bash
gofmt -w <changed-go-files>
go -C backend test ./internal/<changed-package>/...
make test
make build
make web-typecheck
make web-build
```

- 纯 Go 改动至少运行目标包测试；公共契约、安全或跨模块改动运行 `make test` 和 `make build`。
- Web 改动运行 `make web-typecheck` 和 `make web-build`。首次使用前执行 `pnpm install`。
- Runtime 镜像改动在 Docker 可用时运行 `make runtime-image-smoke`。
- MinIO 本地集成使用 `make minio-conformance`；Aliyun OSS 测试依赖技术规格中列出的真实环境变量。
- `make sandbox-conformance` 和 `make production-conformance` 只在满足规格的 Linux + `runsc` Worker 上执行；执行前先跑 `make production-conformance-preflight`。

完成时检查 `git diff`，确保只包含任务内改动、相关测试与必要的权威文档更新；报告已运行命令、未运行的环境门禁和仍然缺失的证据。

## 提交与推送

- 完成用户要求的代码或文档改动并通过适用门禁后，自动创建 Git Commit 并推送当前分支，不等待用户再次提醒。
- 提交前检查 `git status` 和 `git diff`，只暂存本任务及当前连续工作中已经验证的改动；排除用户的无关改动、临时目录、凭证和禁止提交的生成产物。
- 使用准确概括改动的 Conventional Commit message，不修改或合并既有 Commit。
- 推送因鉴权、远端更新或分支保护失败时保留本地 Commit，不重写历史，并向用户报告具体阻塞与安全的后续操作。
