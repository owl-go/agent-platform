# 专家目录交互优化验证

日期：2026-10-10。分支：`codex/expert-team-refactor`；本轮起点：`912f07d`。产品依据：[专家资源规格](../../product/expert-package-team-refactor.md)；包格式依据：[技术契约](../../technical/portable-experts.md)。本轮未发布到线上。

## 验收与实现

| 验收 | 规则与门禁 | 实现及证据 | 结果 |
|---|---|---|---|
| AC-CATALOG-01：专家顶栏仅保留“我的专家”“添加专家”；添加菜单提供创建和 ZIP 导入，创建进入 Create Expert Skill 对话。 | REQ-001/004、UI-001/002、G-WEB | ExpertsPage 及相邻测试；浏览器实际打开添加菜单。 | 通过 |
| AC-CATALOG-02：专家与专家团详情使用弹窗，仅展示名称、能力描述和已有常见任务；不提供另存副本。 | UI-001/003、G-WEB | CatalogDetails 测试确认无抽屉、指引正文或副本按钮；Skill 原有抽屉保持。 | 通过 |
| AC-CATALOG-03：编辑仅使用紧凑技能/连接器下拉框；保存保留名称、指引、头像、常见任务、依赖及团队领队/成员定义；只读权限和重复提交受控。 | COD-002/006、UI-002/004、G-WEB | ExpertBindingsEditor 测试覆盖资料保留、连接器类型区分、未就绪选项、只读、加载/保存失败、重复提交、关闭后的迟到响应。直达旧编辑路由同样限制绑定。 | 通过 |
| AC-CATALOG-04：创建 Skill 生成能力简介、具体常见任务和完整 Markdown；ZIP 复用现有严格 profile 校验。 | COD-002/006、TST-002、G-GO | systemskills 示例逐个通过 resourceaction.Parse；expertpackage 回归包含合法三条任务、缺少简介/指引、空指引、旧字段、分类/标签、凭据、执行设置及非法任务数量/内容。 | 通过 |
| AC-CATALOG-05：桌面与窄屏菜单、弹窗和筛选保存可用，选择列表不撑大页面。 | UI-003/004、G-WEB | Playwright CLI，1440×900 与 390×844，临时隔离预览注入示例 API；30 项技能中搜索“27”、选择并保存，弹窗关闭且目录显示 1 个技能。窄屏页面宽 390px，编辑弹窗宽 366px、两个选择框。 | 通过；仅界面验证 |

## 实际门禁

- `go -C backend test ./internal/systemskills ./internal/expertpackage ./internal/resourceaction`：通过。
- `pnpm -C frontend exec vitest run --maxWorkers=2`：57 文件、647 项通过；随后精简已有团队编辑页的重复加载，增加直达编辑路由回归，`src/pages/ExpertEditors.test.ts` 四项通过。
- 受影响组件与目录测试 `ExpertBindingsEditor.test.ts`、`CatalogDetails.test.ts`、`ExpertsPage.test.ts`：20 项通过。
- `make web-typecheck`、`make web-build`：通过；构建保留既有 bundle 超过 500 kB 的提示。
- `make build`：通过；资源前置校验为 20 Connector Packages、9 Skills、8 Experts、0 Expert Teams 和 3 个内置创建 Skills。
- 首轮类型检查发现测试 fixture 缺少字段，以及示例测试引用了不存在的 Kind 常量；修正后上述检查通过。浏览器预览首次因手写优化模块路径产生重复路由模块，改为标准模块导入；直接点击选择框内部输入被占位文本遮挡，改用实际可点击的选择框区域后筛选与保存通过。未把这些首次失败记为通过。

浏览器截图保存在忽略目录 `output/playwright/expert-modal-desktop-20261010.png`、`expert-edit-mobile-20261010.png` 和 `expert-catalog-mobile-20261010.png`。临时预览页面和服务在验证后清理，不引入生产测试 API。创建预览仍允许确认前完善生成定义；已保存资源的目录编辑只修改绑定。已有定义没有 starter_prompts 时不生成虚构常见任务。

本轮没有修改 API、数据库迁移或 Runtime，未执行新的 PostgreSQL、真实模型协作、真实账号端到端、Linux/runsc Conformance 或线上部署验收；界面示例 API 不替代这些证据。
