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
| EP-06 | Session 保存为 Workflow | 成功对话一键形成可再次运行的 Workflow | Workflow 创建预填 contract、来源关联 | 创建后验证 Run；失败不产生半成品；来源互链 | 待 EP-00/02 |
| EP-07 | Workflow 最小创建与概览 | 用户先用名称和目标验证，再配置 Schedule/API/Git | EP-06、现有 Workflow API | 首次创建不展示全部高级字段；验证 Run 成功后解锁建议 | 待 EP-06 |
| EP-08 | 企业默认黄金组合 | 新用户登录后无需理解 Runtime/Provider 即可开始 | 管理员 verified default、Personal Settings 继承 | 默认组合真实测试证据；不可用时明确阻断，不静默回退 | 待 EP-00 |
| EP-09 | 首页与统一待办 | 最近任务、常用 Workflow、审批与恢复入口集中呈现 | EP-00、Approval、授权和失败聚合 API | 待办完成后返回原任务；不读取用户内容 | 待 EP-06/08 |
| EP-10 | 企业额度治理 | 用户看到预计与实际消耗，管理员分配额度和预算 | Credits 现有账本、预算策略 | 企业部署隐藏 Redemption Code 主入口；Adjustment 不可变 | 待 EP-00 |
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

## 7. 发布与回滚

- EP-02 是展示层增强，不改变执行和持久语义，可按前端版本整体回滚。
- EP-03 起涉及公开协议和持久化，只允许追加字段和向后兼容读取。
- 每个 EP 独立 Commit；通过适用门禁后推送功能分支，再进入 `main_temp` 集成。
