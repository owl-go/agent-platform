---
status: active
baseline-version: 2.0.0
last-reviewed: 2026-10-07
---

# 工程 Harness 采用记录

## 来源与权威边界

本次依据用户在 2026-10-07 明确执行 `ai-project-conventions init` 的指令，采用该 skill 的通用基线 **2.0.0**。旧规范未记录基线版本；[2026-10-04 同步记录](../evidence/agent-workspace/2026-10-04-ai-conventions-sync.md)只证明当时的消息渠道文档同步范围。本次从最新 `origin/main` 的 `e61591f6663e4968bf629112a618ed477feddb3e` 开始，不追认历史实现或生产验收。

| 层 | 项目内权威位置 | 维护方式 |
|---|---|---|
| 通用基线 | [工程规则基线](engineering-baseline.md) | 保存六类规则表及八项字段；个人 skill 升级后先比较规则 ID 和正文，再决定项目升级。 |
| 技术栈补充与验证选择 | 本文的技术栈映射、[质量门禁](quality-gates.md) | 版本与命令以清单、配置、Makefile、CI 为事实源；映射只说明规则如何验证。 |
| 项目约定与当前产物 | [AGENTS.md](../../AGENTS.md)、[CONTEXT.md](../../CONTEXT.md)、产品规格、技术规格及已接受 ADR | 保留领域、安全、权限与 `main_temp` 发布约定；目标、实现和日期化验收分别判断。 |

初始化补充验证与交接规则，不改变产品行为、权限或生产流程。代码与批准规格冲突时记录实现差异；基线不能授权静默修改业务要求。维护者与固定复审周期没有明确仓库依据，本次不指定；接口、数据、技术栈、已批准决策变化，重复失败、事故、例外失效或基线升级时复审。

## 六类适用性与文档影响

六类均适用。规则正文集中在工程规则基线的对应章节；按以下触发读取当前产物并选择门禁，详细规则不复制进工具入口。

| 类别与规则 ID | 范围及适用依据 | 当前权威产物 | 门禁 |
|---|---|---|---|
| 架构 ARC-001–005 | API/Worker、Domain/Application、Data Adapter 与外部系统边界 | [服务架构](../technical/service-architecture.md)、[ADR 0019](../adr/0019-current-technology-stack.md)、[Runtime 契约](../technical/runtime-adapter.md) | G-ARCH、G-API；保持现有模块地图与 ADR，不新建重复架构说明。 |
| 代码 COD-001–006 | Go/TypeScript、输入校验、错误、取消、资源、Secret 与依赖 | [AGENTS.md](../../AGENTS.md)、[领域词汇](../../CONTEXT.md)、语言清单及相邻实现/测试 | G-GO、G-SEC、G-WEB；沿用项目约定，不新增全仓规模或覆盖率阈值。 |
| 数据库 DB-001–006 | PostgreSQL 实体、所有权、事务、并发和不可变追加迁移 | [服务架构的数据库与事务说明](../technical/service-architecture.md)、[Migration](../../backend/internal/infrastructure/gormdb/migrations)、各 Data Model/Repository | G-DB；当前语义继续在技术规格说明，具体迁移再补兼容、恢复与规模证据，不生成空模型/迁移计划。 |
| 需求 REQ-001–005 | User/Administrator 的产品行为、权限、输入边界和验收 | [产品规格](../product/agent-workspace-requirements.md)、相关产品专项及 [Issue 使用约定](../agents/issue-tracker.md) | G-DOC 与实际行为门禁；保留已有需求与 tickets，以具体任务 AC 连接实现和结果。 |
| 测试 TST-001–005 | 单元、集成、契约、回归、浏览器和目标环境验证 | [E2E 策略](../testing/e2e.md)、[Production Conformance](../technical/production-conformance.md)、相邻测试、[CI](../../.github/workflows/ci.yml) | 所有适用 G-*；复用现有测试策略，结果区分通过、失败、未验证和不适用。 |
| UI UI-001–005 | Vue 页面、状态、响应式、双语、键盘与视觉验收 | [UI 标准](frontend-ui.md)、[UI 规格与验收](../specs/frontend-ui-acceptance.md)、[前端入口](../../frontend/AGENTS.md)、[Design Tokens](../../frontend/src/design-tokens.css)、相关产品规格 | G-WEB、G-E2E；沿用已有标准和组件，UI 清单不扩大已批准产品范围。 |

## 技术栈与规则落地

| 规则 | 事实源与实现约定 | 验证方式及盲区 |
|---|---|---|
| ARC-001/003/005 | [架构测试](../../backend/internal/architecture/architecture_test.go)检查 Biz 导入、部分上下文边界和部署配置；[服务架构](../technical/service-architecture.md)说明端口和事务所有权。 | G-ARCH 加受影响导入/调用链人工检查。测试仍含旧上下文表清单，缺失目录会跳过扫描，不能据此宣称当前所有表的所有权已自动覆盖。 |
| ARC-002、COD-002/006 | [workspace.proto](../../backend/api/workspace/v1/workspace.proto)是普通 HTTP API 契约；手写 SSE 和 Runtime 事件分别遵守对应技术规格。 | G-API 与调用方、错误/取消/终态检查；生成一致性不代替流式行为测试。 |
| COD-001/005 | [go.mod](../../backend/go.mod)/go.sum 定义 Go 1.25.0 与 Kratos、Wire、GORM 等依赖；[package.json](../../frontend/package.json)/pnpm-lock.yaml 定义 Vue 3、TypeScript、Vite、Element Plus 与测试工具。 | 格式检查、G-GO/G-WEB；版本不复制为长期清单。CI 的 Node 24、pnpm 11.19.0、PostgreSQL 17.6 来源于 [CI 配置](../../.github/workflows/ci.yml)，不代表本机或生产版本。 |
| COD-003/004、DB-004 | context 传播、单 Run 凭证物化与精确脱敏、安全路径和所有权校验使用现有 seam。 | G-SEC；日志/Artifact/取消清理人工检查及相关测试，未执行目标环境时不能宣称无泄露或隔离已验收。 |
| DB-001/003/005/006 | [迁移器](../../backend/internal/infrastructure/gormdb/migrate.go)按文件名排序，在事务和 advisory lock 内执行 SQL 并记录 SHA-256；已应用文件摘要变化拒绝启动。GORM Model 与 Migration 为实现事实，业务语义由专项技术文档说明。 | G-DB；CI 提供 PostgreSQL。空库/旧数据升级、恢复和真实负载仍需任务级证据，迁移器代码不能证明已演练恢复。 |
| REQ-001–005、TST-001–005 | [产品验收边界](../product/agent-workspace-requirements.md)、tickets、相邻测试和日期化 evidence 分开保存。 | 采用质量门禁中的 AC→规则→检查→结果映射；已有 E2E 编号保留，其他验收用任务内稳定 ID。 |
| UI-001–005 | [根入口](../../AGENTS.md)约定 script setup，[前端入口](../../frontend/AGENTS.md)补充 i18n 与 Element Plus；视觉和响应式来源于 [UI 标准](frontend-ui.md)、Design Tokens 和全局 styles.css。 | G-WEB/G-E2E；类型和构建不能证明键盘、对比度、辅助技术、移动布局或截图已验收。 |

技术栈补充列出的命令已按配置核对；本次仅执行文档检查，详见[初始化证据](../evidence/agent-workspace/2026-10-07-harness-init.md)。未发现独立的安全扫描、重复度或覆盖率门禁配置；不添加臆测命令或统一数值要求，相关任务采用现有检查和人工证据补足。

## 初次采用差异、待确认与升级

| 规则范围 | 旧状态与本次变化 | 处置与验证状态 |
|---|---|---|
| ARC-001–005、COD-001–006 | 已有领域、分层、安全和实现约定，没有版本化规则 ID。 | 保存规则表及项目映射；仅检查快照完整性、路由和路径，未运行架构/代码门禁。 |
| DB-001–006 | 已有 Migration 与事务说明，没有统一规则到任务恢复证据的映射。 | 补齐 G-DB 的兼容/恢复检查要求；历史迁移规模、锁时间和恢复证据未逐项审计，后续数据库任务按受影响范围补证。 |
| REQ-001–005、TST-001–005 | 已有产品验收、E2E、Conformance 和日期化记录，没有统一交接格式。 | 新增门禁与 AC/规则映射、失败恢复和结果状态约定；不批量重编号旧需求或把历史记录升级为当前验收。 |
| UI-001–005 | 已有视觉标准与前端入口。 | 关联现有状态/视口/键盘与视觉验收；未执行浏览器或读屏验证，UI 愿景清单仍受产品契约约束。 |

本次没有申请或批准规则例外。上述历史证据缺口不是豁免；对应未来变更或发布触发必须门禁时，保持待完成，或关联项目已批准且有效的例外。初始化文档检查的完成不证明全仓已经符合所有 MUST。

工具入口保留根目录 `AGENTS.md` → `CLAUDE.md` 的逐字节镜像；`frontend/AGENTS.md` 保持独立作用域。当前会话收到用户提供的根入口指令，并读取磁盘文件；没有验证其他客户端自动发现或加载这些入口。Claude 客户端加载未验证。

升级时按旧/新版本逐项记录新增、变更、移除 ID、要求差异、受影响模块和门禁、例外处置及验证结果。相同版本正文摘要不同视为漂移；完成项目采用与必要验证后才更新版本。以下摘要针对每个源文件从 `| ID |` 开始的首个连续规则表，按 UTF-8、LF、保留末尾换行计算 SHA-256。快照只收录规则表，不携带个人安装路径或模板提示；COD-006 的包内测试链接改为项目快照测试章节，比较时先还原该路径适配。

| 来源（ai-project-conventions 2.0.0 包内） | 类别 | 规则表 SHA-256 |
|---|---|---|
| `references/architecture.md` | 架构 | `40a95985b7ca0d2a02fa151141e9f8c285944b0bac8e8edfefa2ae8d10777c4f` |
| `references/code.md` | 代码 | `e062ae5d1de27055e32a600a9411b2d740e1425bb330653fb64714301c7e5a51` |
| `references/database.md` | 数据库 | `c764e61130b149fbca85a617a139e9afaca18a9da64ce1ddbe32fd36f190fc17` |
| `references/requirements.md` | 需求 | `3cc034fc1af7850bf32993b079863670c342d4e164df91812dee5e0d4dbe5d2b` |
| `references/testing.md` | 测试 | `8e16e4eb66207031b7ad7ae08ddc8c3c051df6f63dccb524aaf96786ecf95803` |
| `references/ui.md` | UI | `a18b53ecba885b9c14fe88d4c93c8b938049b2aa713af1db04079ca1f2803a81` |
