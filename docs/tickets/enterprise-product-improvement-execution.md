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
| EP-00 | 建立基线 | 可以测量首次任务、失败阶段和 Workflow 二次运行 | 产品事件最小字段约束 | 不采集提示词、结果、文件名和 Secret；事件单测与隐私检查 | 待开始 |
| EP-01 | 持续状态与停止 | 长任务在任意滚动位置可见状态、耗时、模型、消耗和停止 | 现有 Session/Run 状态与取消 API | Session、Run Conversation、移动端和停止测试 | 已在 `main` 完成基础版，待补活动摘要与重连指标 |
| EP-02 | 本次执行证据 | 用户能确认实际调用了哪些工具、产生了哪些文件和阶段结果 | 现有 public Activity、Artifact、Expert Stage、Credits | 不把“可用资源”显示为“已使用”；Session/Run 共用组件；中英文测试 | 已完成展示层；来源证据转 EP-03 |
| EP-03 | 来源与 Citation 证据 | 回答能定位实际读取的文件、Knowledge Citation 和 Connector 数据源 | 新的公开 Evidence contract、检索和 Broker 事件 | owner scope、脱敏、失败/未采用状态、历史快照测试 | 待开始 |
| EP-04 | 条件式计划确认 | 复杂或有副作用的任务在执行前可确认范围和步骤 | Plan Snapshot、判定规则、计划确认 API | 普通问答不触发；写操作首次副作用前 100% 确认；Credits 可见 | 待 EP-00/02 |
| EP-05 | 自适应任务面板 | 宽屏集中查看计划、依据、文件和结果；无内容时保持单列 | EP-02/03/04 的统一 View Model | 1280/1440/1920/390px 浏览器测试，无水平页面滚动 | 待 EP-02/03/04 |
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

## 4. EP-03 后端协议预案

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

## 5. 发布与回滚

- EP-02 是展示层增强，不改变执行和持久语义，可按前端版本整体回滚。
- EP-03 起涉及公开协议和持久化，只允许追加字段和向后兼容读取。
- 每个 EP 独立 Commit；通过适用门禁后推送功能分支，再进入 `main_temp` 集成。
