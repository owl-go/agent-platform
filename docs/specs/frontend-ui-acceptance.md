---
status: active
last-reviewed: 2026-10-07
---

# Agent Workspace UI 规格与验收

## 来源与范围

本规格依据 2026-10-07 的 `ai-project-conventions add ui` 请求，覆盖 Vue 产品界面的公共交互和验收方式：应用导航、资源目录、编辑/确认表单，以及 Session 和 Workflow Run Conversation。Smart Assistant、Image Creation、Knowledge Base 和消息渠道的专属流程仍以各产品/技术规格为准，沿用这里适用的公共验收项。Keycloak 托管的登录页面不属于 Vue 组件范围；跨系统登录结果按 [E2E 策略](../testing/e2e.md)验证。

| 来源 | 权威内容与使用方式 |
|---|---|
| [产品规格](../product/agent-workspace-requirements.md)与相关专项 | 用户可见数据、操作、角色/所有权、状态、附件边界、双语和验收目标；视觉清单不新增产品能力。 |
| [视觉标准](../standards/frontend-ui.md) v1.0 | 布局、尺寸、颜色、动效、响应式和无障碍目标；没有单独设计稿时以此及已批准产品交互为设计来源。 |
| [UI 规则](../standards/engineering-baseline.md)、[前端入口](../../frontend/AGENTS.md) | UI-001–005 与当前实施约定；本文件只具体化状态、验收场景和证据，不复制规则正文。 |
| [质量门禁](../standards/quality-gates.md) | G-WEB、G-E2E 的真实命令、环境前提与交付状态；文档维护走 G-DOC。 |

本文件的 AC-UI-* 是公共验收索引。每次页面变更补充任务内的具体前置数据、触发、可观察结果和实际证据，引用相关产品条目及适用 AC；不表示全站所有页面已通过。没有适用状态时在任务中说明数据流依据。

## 设计系统与实现来源

| 层/场景 | 当前可定位来源 | 复用与验收关注点 |
|---|---|---|
| 基础视觉与语言 | [Design Tokens](../../frontend/src/design-tokens.css)、[全局样式](../../frontend/src/styles.css)、[i18n](../../frontend/src/i18n/index.ts)、[入口装配](../../frontend/src/main.ts) | Token 是样式实现来源，视觉目标仍由标准定义；检查实际计算样式和中文/英文文案。Element Plus 映射使用 --el-*，平台 Token 使用 --aw-*；兼容别名不能当成第二套设计系统。 |
| 应用框架/导航 | [App.vue](../../frontend/src/App.vue)、[Router](../../frontend/src/router.ts)、[App 测试](../../frontend/src/App.test.ts) | 当前页面与所属导航关系、Resource Library 二级入口、移动导航、身份检查与失败反馈。 |
| 资源目录 | [ResourceCenterPage](../../frontend/src/pages/ResourceCenterPage.vue)、[CatalogLoading](../../frontend/src/components/CatalogLoading.vue)、[目录测试](../../frontend/src/pages/ResourceCenterPage.test.ts) | 复用四类目录和骨架；首次请求未返回时不提前显示空结果。加载、空数据和失败分别验证。 |
| 编辑与瞬时反馈 | [ExpertEditorPage](../../frontend/src/pages/ExpertEditorPage.vue)、[ConfirmDialog](../../frontend/src/components/ConfirmDialog.vue)、[ToastMessage](../../frontend/src/components/ToastMessage.vue) | 输入标签、保存/删除状态与反馈；Toast 的 alert/status、关闭按钮和定时清理由组件实现，焦点和弹层遮挡仍需浏览器验证。 |
| 对话/执行 | [ConversationComposer](../../frontend/src/components/ConversationComposer.vue)、[ConversationThread](../../frontend/src/components/ConversationThread.vue)、[ExecutionStatusBar](../../frontend/src/components/ExecutionStatusBar.vue)、[ConversationAttachments](../../frontend/src/components/ConversationAttachments.vue)、[TaskWorkspacePanel](../../frontend/src/components/TaskWorkspacePanel.vue) | Session 与 Run Conversation 复用角色、消息、输入、附件和执行状态；平台状态来自真实数据，过程内容不当作成功结果。 |
| 专项例：知识检索 | [KnowledgeBasesPage](../../frontend/src/pages/KnowledgeBasesPage.vue)、[相关测试](../../frontend/src/pages/KnowledgeBasesPage.test.ts) | 检索弹窗、未就绪/无匹配/失败与来源引用；读者和维护者操作分别验证。 |

新增组件/样式先检查上表与相邻页面是否能表达需求，再在任务记录说明扩展的产品依据、差异和影响。复用不能省略加载、失败、无权限或输入边界检查。组件源码和测试存在只证明可以定位实现与断言，不证明当前浏览器行为已经通过。

## 状态与表单交互

下表给出验收场景与检查要求；“检查”列描述需要观察的结果，不宣称各页面已实现。

| 状态 | 触发与可观察反馈 | 操作/校验/重复提交检查 | 恢复与焦点检查 | 验收 ID |
|---|---|---|---|---|
| 初始 | 登录身份检查或首次进入页面；应用框架与当前导航有明确位置。新对话尚无消息与数据加载分开。 | 身份未确认时不呈现私有业务内容；标题和必要操作符合当前产品契约。 | 身份失败在同一入口给出可读反馈；首次可操作控件可由键盘到达。 | AC-UI-01/02/04/07 |
| 交互 | 编辑表单、展开选择菜单、打开详情/弹窗；选中项与可操作范围可辨认。 | 表单有可识别标签；Enter/Shift+Enter、输入法组合、菜单确认与发送分别检查；选择资源本身不提交消息。 | 检查焦点进入目标输入/弹层，关闭后回到有效触发点；返回详情后所属导航保持选中。 | AC-UI-01/03/05/07 |
| 加载 | 延迟目录/API、提交、上传或等待流；未完成请求显示相应加载反馈。 | 目录骨架不与空结果混淆；提交/发送中的 loading 与有效禁用状态检查重复点击和键盘提交。 | 网络慢不丢失草稿；结束加载后能继续操作，不留下永久 busy。 | AC-UI-02/03/05 |
| 空 | 请求成功且零记录、对话无消息、检索无匹配；空反馈说明当前情况。 | 引导操作依角色、当前能力及产品依据提供；不使用样例记录填充，不把未就绪或失败显示为无结果。 | 检查有效创建/上传/返回入口及键盘可达性；确无操作的只读场景记录不适用依据。 | AC-UI-02/04/07 |
| 成功 | 保存被服务端接受、发送被接受、执行终态成功；反馈与真实状态一致。 | 瞬时成功使用统一 Toast；持续的配置指导和一次性信息按产品契约留在页面。执行成功与消息提交成功分别判定。 | 根据实际流程检查列表/详情/草稿更新；Toast 关闭不打断主要操作，自动消失不移除业务内容。 | AC-UI-03/05/07 |
| 失败 | API 拒绝、加载/上传/发送失败、检索失败、执行终态失败。 | 错误可读，下一步来自实际支持的恢复操作；重复提交不掩盖首次失败；可重试的执行按 retryable 等权威状态控制。 | 发送失败保留可恢复草稿；适用重试能回到操作路径。检查错误播报、关闭/恢复控件与焦点，截图不替代操作。 | AC-UI-02/03/05/07 |
| 无权限/只读 | 普通 User 打开管理入口、非维护者打开共享资源、访问另一 User 的对象、已归档内容。 | 依据角色和资源 scope 显示只读或可维护操作；客户端隐藏控件不代替 API 所有权验证。不存在与越权按产品公开错误语义处理。 | 不为恢复操作扩大权限；可用的返回入口和错误反馈不暴露私有内容。 | AC-UI-04/07 |
| 边界/执行中 | 长文本/文件名、附件上限、窄视口、排队、等待 User 操作、取消或终态。 | 附件数量/大小遵守产品边界；停止、等待与重试操作匹配真实执行状态，取消不渲染为失败或成功；长内容仅在允许区域滚动。 | 检查等待/终态后的操作恢复、草稿保留、弹层滚动、输入可达与移动端焦点；不渲染未验证 Capability 的占位结果。 | AC-UI-05/06/07 |

表单字段、必填、长度、版本冲突与文件限制以对应 API/产品契约和页面实现为证据，不统一发明字段阈值。验收覆盖有效输入、空白/非法/超界输入、服务端错误、慢请求、连续点击和取消后的恢复。页面没有某项校验或恢复证据时标为未验证或实现差异，不能因使用 Element Plus 就记为通过。

## 视口、输入与无障碍

响应式目标沿用视觉标准的 1440px/1024px 布局边界。已有历史视觉记录使用 1440×900、1024×768、390×844；当前 [E2E-030](../../frontend/e2e/workspace.spec.ts)指定 390×844。后续任务按受影响布局选择这些有来源的尺寸，并在涉及断点变化时补充边界两侧；它们是检查尺寸来源，不是本轮实测或新的支持设备清单。

桌面验证鼠标与键盘，移动验证触摸和输入可达；单纯缩窄浏览器不能证明真实软键盘下可用。检查主页面横向溢出、标题/返回入口、导航、表单按钮和弹层；需要横向滚动的文档表格限制在其容器。长名称、双语、长答案和打开弹层后的尺寸变化作为相关页面的边界数据。

无障碍目标仍以视觉标准第 9 节和产品验收边界为准：语义名称、可见焦点、Tab 顺序、输入错误反馈、弹层进入/关闭的焦点恢复、正文与大字对比度、减弱动效，以及适用的辅助技术检查。当前全局样式有 focus-visible 和 prefers-reduced-motion 规则，Composer、Toast、ConfirmDialog、状态条具有部分语义实现；这不能直接证明所有控件或读屏行为合格。新增图标按钮使用现有本地化可访问名称。

键盘验证按实际控件区分：Composer 的 Enter 发送、Shift+Enter 换行、菜单确认和输入法组合；Esc 关闭适用浮层，忙碌确认框按实际 busy 语义处理。视觉标准里的快捷键或播报目标若没有产品流程/实现/验收证据，记录具体差异，不将其描述为已可用的能力。未执行读屏或对比度测量时不得宣称满足完整可访问性标准。

## 验收与证据映射

| AC（前置、触发、可观察结果） | 规则 / 门禁 | 可定位实现或已有断言 | 当前版本实际验收 |
|---|---|---|---|
| AC-UI-01：进入/编辑受影响页面，中文与英文的标题、导航、组件和样式符合产品契约与视觉来源，文案和装饰信息均有需求依据。 | UI-001/005；G-WEB | App、i18n、Token、相邻页面；[i18n 测试](../../frontend/src/i18n/index.test.ts)、[Expert 编辑测试](../../frontend/src/pages/ExpertEditors.test.ts)。 | 未验证：本次仅读取来源。 |
| AC-UI-02：分别延迟、成功返回空结果、拒绝数据请求后，加载/空/失败可区分且提供适用操作。 | UI-002；G-WEB | CatalogLoading、目录页面；[目录测试](../../frontend/src/pages/ResourceCenterPage.test.ts)。 | 未验证：未执行组件或浏览器测试。 |
| AC-UI-03：有效/非法表单提交、慢请求和失败后，反馈对应结果，重复动作受控，输入可恢复。 | UI-002/004；G-WEB | ExpertEditorPage、ConfirmDialog、Toast；[编辑测试](../../frontend/src/pages/ExpertEditors.test.ts)、[Toast 测试](../../frontend/src/components/ToastMessage.test.ts)。 | 未验证：既有断言不视为覆盖全部字段/重复提交。 |
| AC-UI-04：不同 User/Administrator/Resource Publisher 和 scope 打开相同对象，只能观察/执行有权操作，私有内容不被 UI 或 API 暴露。 | UI-002、COD-002；G-WEB/G-E2E/G-SEC | App 与相关页面/API；[Knowledge Base 权限测试](../../frontend/src/pages/KnowledgeBasesPage.test.ts)、[浏览器/API E2E](../../frontend/e2e/workspace.spec.ts)。 | 未验证：未进行真实角色/API 闭环。 |
| AC-UI-05：在 Session/Run Conversation 选择资源、输入/粘贴文件、发送、失败恢复或取消，草稿与状态符合产品语义，不重复发送或把过程当作完成。 | UI-002；G-WEB/G-E2E | Composer/Thread/状态条/附件；[Composer 测试](../../frontend/src/components/ConversationComposer.test.ts)、[附件测试](../../frontend/src/components/ConversationAttachments.test.ts)、[Thread 测试](../../frontend/src/components/ConversationThread.test.ts)。 | 未验证：未执行本轮组件、流式或真实 Runtime 验收。 |
| AC-UI-06：在受影响目标视口和适用输入方式打开页面/弹层，核心操作可达，长内容与局部滚动不拉宽主页面。 | UI-003；G-WEB/G-E2E | App、styles.css、TaskWorkspacePanel；E2E-030 和下面的历史视觉记录。 | 未验证：本轮没有截图、尺寸测量或触摸/软键盘操作。 |
| AC-UI-07：使用键盘和适用辅助技术操作关键流程，控件名称、焦点、错误/状态播报、对比度及减弱动效满足既有目标。 | UI-004；G-WEB/G-E2E | Composer、Toast、ConfirmDialog、ExecutionStatusBar 和全局样式；相邻语义/键盘断言。 | 未验证：浏览器焦点、读屏和实际对比度均未复验。 |
| AC-UI-08：交付本次页面变更时，每个适用 AC 关联产品条目、代码版本、状态、实际尺寸、视觉对照及单独的操作结果。 | UI-005、TST-005；G-WEB/G-E2E | 任务/Issue 或日期化 evidence，门禁与完成报告。 | 未验证：本次没有页面变更验收；仅建立映射。 |

当前版本以实际受测 Commit/资源版本记录，不使用本文件更新日期替代代码版本。任务证据至少含：AC/规则 ID、测试账号角色（无凭证）、状态与输入、浏览器/视口尺寸、设计来源、可定位截图或录屏、操作与键盘结果、适用读屏/对比度结果、通过/失败/未验证及未覆盖原因。图片只证明视觉状态；组件测试不能代替真实布局，静态资源发布不能代替认证后的浏览器闭环。

测试命令、依赖安装与 E2E 环境沿用 G-WEB/G-E2E，不复制另一套执行流程。截图/报告按 [E2E 策略](../testing/e2e.md)放入忽略目录或获授权的证据位置，避开登录凭证、API Key、私有内容和敏感日志；不为截图引入线上样例记录。

## 历史证据与未验证项

| 历史记录 | 已记录的范围 | 当前使用限制 |
|---|---|---|
| [2026-10-01 Connector 目录布局](../evidence/agent-workspace/2026-10-01-connector-catalog-layout-local-validation.md) | 当时组件/类型/构建与 1280×960、390×844 Fixture 视觉、弹层滚动和 Esc。 | 本轮未重跑；不证明当前目录、真实账号授权或生产 Runtime。 |
| [2026-10-02 资源目录加载](../evidence/agent-workspace/2026-10-02-resource-catalog-loading-deployment.md) | 当时骨架/空/失败组件断言、局部视口与双语/减弱动效检查、发布静态产物一致性。 | 记录明确没有发布后真实账号四类目录浏览器闭环；保留这一限制。 |
| [2026-10-02 Knowledge Base 布局及后续修改](../evidence/agent-workspace/2026-10-02-knowledge-layout-validation.md) | 当时 1440×900、1024×768、390×844 Fixture；后续检索弹窗焦点/Esc、移除目录搜索与简化上传的独立检查。 | 根据对应后续段落和版本判断范围，不使用较早描述恢复已移除控件；未重做当前认证环境与 RAGFlow 验收。 |

本轮没有新增或批准设计差异、规则例外、业务行为或前端代码。公共矩阵不能替代各专项页面的状态和权限场景，未来任务应按受影响范围补齐。全站快捷键、所有弹层焦点、辅助技术、暗色主题与真实移动设备没有本轮完整验收证据；不从 Token/源码存在推断支持完成。

本次 `add ui` 的交付是规格、来源和路由的文档检查；代码测试、类型/构建、浏览器/视觉/读屏与生产验收未运行。适用页面验收缺少 MUST 证据时按质量门禁保持待完成，本次文档完成不豁免后续 UI 门禁。
