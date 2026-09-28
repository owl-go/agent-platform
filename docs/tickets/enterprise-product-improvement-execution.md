# 企业产品改进实施计划

状态：执行中

依据：`docs/product/enterprise-product-improvement-plan.md`

## 1. 实施原则

- 每个批次只交付一个可独立验收的用户结果，不按页面或技术层随意拆分。
- 先复用当前 Domain、Application 和 Runtime seam；新增协议时先固定公开状态和失败语义。
- 不把模型生成的描述当作执行证据。工具、文件、来源、Credits 和状态只能来自平台记录。
- 不把未执行的测试、真实 Provider 或 Linux + `runsc` 验证写成完成。
- P0 期间不扩展新的 Runtime、模型协议、Connector、Digital Human 或图片能力。

## 2. 执行顺序

| ID | 垂直切片 | 用户结果 | 依赖 | 验收与门禁 | 状态 |
|---|---|---|---|---|---|
| EP-00 | 建立基线 | 可以测量首次任务、失败阶段和 Workflow 二次运行 | 产品事件最小字段约束 | 不采集提示词、结果、文件名和 Secret；事件单测与隐私检查 | 已完成代码与本地门禁；待真实部署形成基线 |
| EP-01 | 持续状态与停止 | 长任务在任意滚动位置可见状态、耗时、模型、消耗和停止 | 现有 Session/Run 状态与取消 API | Session、Run Conversation、移动端和停止测试 | 已完成代码与本地门禁；生产重连率待部署后形成基线 |
| EP-02 | 本次执行证据 | 用户能确认实际调用了哪些工具、产生了哪些文件和阶段结果 | 现有 public Activity、Artifact、Expert Stage、Credits | 不把“可用资源”显示为“已使用”；Session/Run 共用组件；中英文测试 | 已完成展示层；来源证据转 EP-03 |
| EP-03 | 来源与 Citation 证据 | 回答能定位实际读取的 Knowledge Citation 和 Connector 数据源 | 新的公开 Evidence contract、检索和 Broker 事件 | owner scope、脱敏、失败/未采用状态、历史快照测试 | 已完成代码与无数据库本地门禁；PostgreSQL Integration、真实 Provider 与生产历史证据待验证 |
| EP-04 | 条件式计划确认 | 复杂或有副作用的任务在执行前可确认范围和步骤 | Plan Snapshot、判定规则、计划确认 API | 普通问答不触发；写操作首次副作用前 100% 确认；Credits 可见 | 已完成代码与本地门禁；真实 Provider、浏览器断点与 PostgreSQL Integration 待验证 |
| EP-05 | 自适应任务面板 | 宽屏集中查看计划、依据、文件和结果；无内容时保持单列 | EP-02/03/04 的统一 View Model | 1280/1440/1920/390px 浏览器测试，无水平页面滚动 | 已完成代码与本地组件/页面门禁；浏览器布局验收见 EP-05.3，真实部署待验证 |
| EP-06 | Session 保存为 Workflow | 成功对话一键形成可再次运行的 Workflow | Workflow 创建预填 contract、来源关联 | 创建后验证 Run；失败不产生半成品；来源互链 | 已完成代码、本地门禁与临时 PostgreSQL Integration；真实部署闭环待验证 |
| EP-07 | Workflow 最小创建与概览 | 用户先用名称和目标验证，再配置 Schedule/API/Git | EP-06、现有 Workflow API | 首次创建不展示全部高级字段；验证 Run 成功后解锁建议 | 已完成代码与本地门禁 |
| EP-08 | 企业默认黄金组合 | 新用户登录后无需理解 Runtime/Provider 即可开始 | 管理员 verified default、Personal Settings 继承 | 默认组合真实测试证据；不可用时明确阻断，不静默回退 | 已完成代码、本地门禁与 PostgreSQL Integration；生产 Provider 证据待验证 |
| EP-09 | 首页与统一待办 | 最近任务、常用 Workflow、审批与恢复入口集中呈现 | EP-00、Approval、授权和失败聚合 API | 待办定位回原任务；不读取用户内容 | 已完成代码、本地门禁与 PostgreSQL owner-scope Integration；真实部署待验证 |
| EP-10 | 企业额度治理 | 用户看到预计与实际消耗，管理员分配额度和预算 | Credits 现有账本、预算策略 | 企业部署隐藏 Redemption Code 主入口；Adjustment 不可变 | 已完成代码、本地门禁与 PostgreSQL Integration；生产用量分布与部署浏览器闭环待验证 |
| EP-11 | 资源库收敛 | Expert、Skill、Connector、Knowledge Base 按任务发现 | 现有 catalog API | 所有资源显示来源、可用性和权限；未验证资源不推荐 | 待核心闭环 |
| EP-12 | 企业治理 | 多管理员、用户组、部门资源和离职转移 | 新授权模型和身份源同步 | 跨范围访问 fail closed；管理员不可读私有内容 | 待设计伙伴验证 |
| EP-13 | Smart Assistant 受控发布 | 将已验证问答发布给内部或受控访客 | EP-03/08/10/12 | FAQ/Knowledge 来源、安全、额度和 iframe 审计闭环 | 待核心指标连续四周达标 |

## 3. EP-02 详细任务

### EP-02.1 统一前端证据 View Model

- 给共享 Conversation Activity 增加公开类别：Runtime、Reasoning、Tool、File、Activity。
- Session 从持久化 `ExecutionActivity` 映射；Run Conversation 从持久化/流式 `RunEvent` 映射。
- Feishu 等品牌差异只影响本地化动作名称，不进入共享组件分支。

### EP-02.2 改造执行详情

- 将泛化的“查看执行过程”改为“本次执行”。
- 折叠标题显示平台可证明的数量：工具调用、文件变化、Expert Stage、Artifact。
- 每一组显示类别与完成状态；运行中仍保留当前 Activity。
- 没有外部工具、文件和 Artifact 时显示“未记录外部工具或文件变化”，不宣称没有使用知识来源。

### EP-02.3 测试与视觉

- ConversationThread 组件测试覆盖工具、文件、Stage、Artifact 和无外部证据。
- SessionsPage 测试覆盖 `ExecutionActivitySummaryKind` 到公开类别的映射。
- WorkflowDetailPage 测试覆盖 `RunEvent` 类别映射。
- 使用现有 Design Token；桌面和移动端不新增未经需求支持的常驻面板。
- 运行相关 Vitest、`make web-typecheck`、`make web-build` 和 `git diff --check`。

## 4. EP-03 来源证据

EP-02 不解决 Knowledge Citation 和“结果是否采用某次调用”的完整证据。EP-03 需要新增最小公开 Evidence：

```text
Evidence
├── kind: file | knowledge | connector | artifact
├── source identity and safe display name
├── state: requested | succeeded | failed | not_used
├── action summary
├── owning message/run and stage position
└── optional bounded citation/location
```

Evidence 与 Assistant Message 或 Run terminal state 一起持久化；Secret、原始 Tool Output、完整敏感输入、Provider response 和 Chain-of-Thought 不进入协议。

### EP-03.1 公开 Evidence 与持久化

- Assistant Message 和 Run 追加同一组 `Evidence`；旧记录按空数组读取。
- 单次执行最多保留 64 条，安全名称、动作和位置均有长度上限；协议不存在原始 Tool Output、参数、凭证、Prompt 或 Provider Response 字段。
- Worker 只在终态事务中随 Message/Run 一起写入 Evidence；中断恢复重新执行时清空未完成 Evidence，避免把旧尝试误当成本次结果。

### EP-03.2 平台证据来源

- Knowledge Retrieval 命中由权限校验后的 Retrieval seam 生成 `succeeded` Citation；无命中为 `not_used`，索引或 Provider 不可用为 `failed`。
- CLI Connector 只在服务端 Broker 通过 Definition 和 Capability 校验后记录调用；成功、失败和已选择但未调用分别显示，参数、Target、输出和外部账号标识不进入 Evidence。
- 附件、可用 Skill/MCP 和仅出现在快照中的资源不等于“已使用”，本批次不为它们伪造来源。

### EP-03.3 来源打开与界面

- Session 与 Run Conversation 复用同一来源列表，区分“已使用”“调用失败”“未采用”；折叠摘要只统计 `succeeded` Evidence。
- 只有成功的 Knowledge Citation 提供“打开来源”。下载请求携带执行时的 Document Revision ID，服务端在同一次查询中重新校验当前用户对 Knowledge Base、Document 和该不可变 Revision 的权限；删除、私有化或权限撤销后显示不可用，不改写历史 Evidence，也不会把更新后的 Revision 冒充旧来源。
- 当前代码可证明 bounded contract、owner-scoped download seam、状态展示和精确 Revision 拒绝行为；AnythingLLM、真实 CLI Connector、生产迁移及历史记录仍需部署环境验收，不能记作已验证。

## 5. EP-04 条件式计划确认

### EP-04.1 判定与冻结

- Session 仅在用户显式选择、多个 Execution Stage、两个以上不同外部资源或可能产生副作用的 Connector 存在时生成 Execution Plan；普通单阶段问答不增加等待。
- 手动 Workflow Run 和交互式 follow-up 必须先生成 Plan，因为 Runtime 可能修改持久 Workspace；Scheduled/API Run 保持非交互执行，不伪造人工确认。
- Plan 与 Message/Run 在一个事务中持久化，冻结目标、步骤、资源、安全副作用类别、模型调用数和 fallback Credit 估算。当前生成器为确定性平台规则，生成消耗明确为 0 Credits。

### EP-04.2 决策与执行边界

- `开始执行`、`直接回答`、`取消` 使用 owner-scoped、versioned 决策 API；并发或重复决策返回 conflict。
- `直接回答` 只对无副作用的 Session Plan 开放，Worker 同时移除 MCP/CLI Connector 配置，不能只在界面上跳过 Plan 后继续外部调用。
- `修改要求` 先取消当前冻结 Plan，再把原请求作为新的可编辑输入；不原地修改隐藏状态。Workflow、MCP 或高风险 CLI 的副作用在执行前可见，高风险命令仍经过独立的一次性审批。
- pending Workflow Plan 占用有限队列容量；取消形成终态记录。Worker 中断恢复沿用已确认的冻结输入，但重置本次尝试的可见步骤进度。

### EP-04.3 状态与界面

- Session 与 Run Conversation 复用计划卡，显示目标、步骤、资源、副作用、模型调用和 Credits 估算；仅允许的动作才出现。
- Plan Step 状态由 Worker claim、Expert Stage 和终态事务回写为 pending/running/completed/skipped/failed，不从模型文字推断。
- 本地可验证内容包括领域判定、去重资源、side-effect direct-answer 拒绝、API contract、共享组件和完整前端测试。真实 Provider 的首次副作用时序、数据库迁移、1280/1440/1920/390px 浏览器布局仍需目标环境验证，不能记作已通过。

## 6. EP-05 自适应任务面板

### EP-05.1 内容与选择

- Session 和 Run Conversation 复用同一个 Task Panel，只展示当前 Assistant Message 已持久化的 Execution Plan、Evidence、Execution Activity、Expert Stage、输入附件、Artifact、状态、耗时和 Credits；不解析回答文字补造步骤或来源。
- 只有历史回答包含上述任一任务内容时才显示“查看任务详情”，打开后可切换到该回答。首次进入一段 Conversation 时默认选中最新的可检查回答；纯问答没有 Task Panel，仍保持单列。
- 输入附件取自生成该回答的 User Message 或 Run，不混入其他轮次。Knowledge 来源仍通过 EP-03 的精确 Revision 权限检查打开；过期 Artifact 只显示元数据且不可下载。

### EP-05.2 响应式布局与状态

- Assistant 正文最大阅读宽度为 `76ch`。Workflow 在视口宽度至少 1360px 时将 Task Panel 集成到右侧，1360–1439px 使用 320px 面板，更宽视口使用 360px；1280px 使用覆盖抽屉，避免正文缩到 68ch 以下。
- Session 还有产品主导航和 240px Session 列表，因此只在至少 1600px 时集成 320px 右栏；较窄桌面使用右侧覆盖抽屉，避免把正文压缩到目标阅读宽度以下。
- 700px 及以下使用从移动端 Header 下方展开的全宽面板。关闭状态按 Session/Run Conversation ID 保存在当前浏览器设备；再次点击历史回答可重新打开。

### EP-05.3 验收证据边界

- Vitest 覆盖任务内容判定、最新历史回答选择、四类内容分组、来源/Artifact 动作、Session 和 Run Conversation 的关闭与重新打开行为。
- 本地浏览器布局检查使用当前生产 CSS 与等价组件 DOM，在 1280、1440、1920 和 390px 检查 Task Panel 尺寸、正文区域和 `document.documentElement.scrollWidth <= clientWidth`；该检查不等于真实 OIDC、API 或 Provider 部署验收。
- 2026-09-28 的本地 Chromium 检查结果：四个视口的 `scrollWidth` 均等于 `clientWidth`；Workflow 在 1280px 使用 358px 覆盖抽屉、1440px 使用 360px 集成面板、1920px 使用 360px 集成面板，390px 使用从 `y=56` 开始的 390px 全宽面板；Session 在 1440px 使用 380px 覆盖抽屉、1920px 使用 320px 集成面板。截图保存在本地 `output/playwright/`，不作为生产证据提交。
- `make web-typecheck`、`make web-build` 和 `git diff --check` 是提交门禁。真实部署的浏览器到 API 闭环仍按第 14 节执行，不能由静态布局检查替代。

## 7. EP-06 Session 保存为 Workflow

### EP-06.1 服务端原子边界

- 只有 owner-scoped、`completed` 的 Assistant Message 可以转换；名称和目标可编辑，Expert、Skill、MCP 与 CLI 资源必须来自该回复的服务端 Response Snapshot，客户端不能覆盖。
- `workflows.execution_template` 只保留该回复的专家与资源阶段；每个新 Run 仍解析当前 Personal Settings 的 Runtime Engine、Provider Model、协议和 Credit Rate。用户在 Workflow 设置中切换 Specialist 时清除转换模板，避免旧专家继续执行。
- Workflow、Session Workflow Origin 与首个 `session_conversion` 验证 Run 在同一个数据库事务中创建。验证 Run 带 pending Execution Plan 并进入 `waiting_for_user`；同一 owner/Session/Message 的并发或重复请求返回原对象。
- 来源附件与 Artifact 在提交前逐个选择 `Workspace` 或 `不带入`。选择 Workspace 时先做 Size/SHA-256 校验并放入预分配 Workflow Workspace；数据库事务失败或幂等回放时清理本次预放内容。过期或缺少对象的文件只能排除。Knowledge Base 转换不在本批次伪造，留给资源库动作。

### EP-06.2 界面与来源导航

- 每个成功回复和对应 Task Panel 结果区显示“保存为工作流”；已保存的回复改为“打开已保存工作流”。
- 确认层预填 Session 标题和原 User Message，显示实际带入的 Specialist/Skills/Connectors，并要求每个文件有明确去向。
- 创建成功直接打开首个验证 Run 的 Plan；Workflow 标题区提供“来自会话”，Session 通过 Message Link 回到该 Workflow。两边后续历史独立。

### EP-06.3 验收证据边界

- Vitest 覆盖预填、文件默认去向、API contract、验证 Run 自动打开和来源展示；完整前端结果为 44 个文件、319 个测试通过。
- `make test`、`make build`、`make web-typecheck`、`make web-build` 与 `git diff --check` 通过。
- 使用临时 PostgreSQL 17 容器实际运行完整 migration chain 和 `TestSessionWorkflowConversionIsAtomicAndIdempotent`，验证成功转换、pending 验证 Run、幂等回放及失败回复不产生 Workflow。该结果不等于真实 OIDC、对象存储、Provider 或生产部署验收。

### EP-07 Workflow 最小创建与概览

- 新建入口只收集名称和目标，不提前展示专家、知识库、环境变量、Schedule、API 或 Git 配置。
- 创建成功后立即使用现有手动 Run API 生成强制 Plan 的验证 Run，并直接打开该 Plan；若 Run 启动失败，保留已创建 Workflow、直接进入其运行记录并给出可恢复提示，避免用户重试后重复创建。
- 只有至少一个 Run 成功后，详情页才展示 Schedule、API Credential 和 Git Source 的后续配置建议；完整设置仍留在 Settings。
- Vitest 覆盖最小提交、自动验证 Run、直接打开 Plan，以及成功前后建议的显隐。该批次不声称真实 Provider 验证成功。

### EP-08 企业默认黄金组合

- 新增唯一 Platform Execution Default。管理员只能从可用 Runtime、已验证 Provider Connection、非 incompatible 的 Provider Model，以及所有执行阶段均精确匹配该组合的成功 Run 建立默认；保留 Run ID 作为审计证据，并在同一事务把 unverified 组合晋升为 verified，incompatible 组合不能晋升。
- Personal Settings 默认继承企业组合。更新企业默认时原子同步所有仍在继承的账号，新账号通过数据库触发器继承；用户保存个性、语言和时区不会隐式退出继承，也可显式切换为个人执行设置。
- 未配置企业默认或组合不可用时沿用现有 setup blocking，绝不从目录中静默挑选另一个 Runtime 或模型。管理员界面允许用成功 Run 验证 unverified 组合，但不展示 incompatible 组合。
- `make test`、`make build`、前端目标测试、typecheck、build 和临时 PostgreSQL 17 的完整 migration/integration 是本地证据。它们不等于真实 Provider 或生产镜像执行证据；生产设默认前仍必须先获得真实成功 Run。

### EP-09 首页与统一待办

- 登录和未知路径进入 Home。Home 只聚合 owner-scoped 的 Session 标题、Workflow 名称、执行状态、时间和 Run 次数，不查询消息正文、生成结果、Runtime 错误、执行参数、文件名或外部账号。
- 待办按命令审批、计划确认、失败恢复排序；同一执行同时有 Plan 和命令审批时只保留审批。过期审批、归档或 external Session、已删除 Workflow 和其他账号的数据不出现。
- 待办不复制决策逻辑。点击后打开所属 Session 或精确 Workflow Run，由已有 Plan、命令审批和恢复控件处理；最近任务和常用 Workflow 也只提供原对象入口。
- 前端覆盖聚合区、空状态、原 Run 定位和 API repeated-field 兼容；PostgreSQL Integration 运行完整 migration chain，验证 owner 隔离、待办去重与内容/错误不进入返回 contract。真实 OIDC、生产数据分布和部署浏览器闭环仍待验证。

### EP-10 企业额度治理

- 新增唯一且 versioned 的 Enterprise Credit Policy：新建额度账户继承平台每日默认额度，已有账号继续使用自己的额度；管理员可设置 1–99% 用量提醒线。单用户额度仍在下一个 Credit Day 生效，Adjustment 继续要求原因、request ID 和不可变 Ledger。
- 用户入口统一显示 Available Credit、执行中预留、今日使用和恢复时间。达到提醒线或耗尽时明确联系管理员；多阶段结果只展示阶段序号、额度和 measured/fallback 结算类别，不泄露 Token、模型、倍率、供应商成本或费率修订。
- Redemption Code 默认关闭；普通用户输入、管理员 Tab 及相关 API 一起隐藏或 fail closed。只有管理员在 Enterprise Credit Policy 显式启用后才恢复原渠道能力。
- 文本 Admission 和图片 Reservation 改为使用跨 daily/persistent bucket 的净 Available Credit。一个正余额 bucket 不能再掩盖另一个 bucket 的债务；真实 PostgreSQL 回归覆盖过量结算形成负余额、后续拒绝、新账号继承策略、策略 CAS 和兑换渠道开关。
- 本地门禁不等于生产用量分布或部署浏览器闭环证据；本批仅提供产品内阈值提示，不提供站外通知。部门预算和跨用户组策略依赖 EP-12 的组织授权模型，不在本批次伪造。

## 8. 发布与回滚

- EP-02 是展示层增强，不改变执行和持久语义，可按前端版本整体回滚。
- EP-03 起涉及公开协议和持久化，只允许追加字段和向后兼容读取。
- 每个 EP 独立 Commit；通过适用门禁后推送功能分支，再进入 `main_temp` 集成。
