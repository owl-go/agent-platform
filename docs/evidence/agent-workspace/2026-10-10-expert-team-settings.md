# 专家团设置验证（2026-10-10）

基线：`05bcd86`；开发分支：`codex/expert-team-refactor`。本轮根据用户追加要求，将专家团新增和编辑统一为名称、描述、成员、领队四项设置；替代[上一轮目录验收](2026-10-10-expert-catalog-ui.md) AC-CATALOG-03 中的团队仅编辑绑定行为。单专家仍仅编辑技能和连接器。行为依据：[产品规格](../../product/expert-package-team-refactor.md)、[技术规格](../../technical/portable-experts.md)。

| 验收点 | 规则与门禁 | 实际证据 | 结果 |
|---|---|---|---|
| AC-TEAM-SETTINGS-01：管理员通过“添加专家团 → 创建专家团”打开四项设置弹窗；编辑和旧直达路由复用相同表单。 | REQ-001/004、UI-001/002、G-WEB | ExpertsPage、CatalogDetails、ExpertEditors、ExpertTeamSettings 组件测试；浏览器打开菜单及新建、编辑弹窗。 | 通过 |
| AC-TEAM-SETTINGS-02：成员为可搜索多选下拉，领队只能从所选成员中选择；成员限制 2–10 位，移除领队后必须重新指定。 | UI-002/004、TST-002、G-WEB | 表单回归覆盖新增、删除、领队清空、无领队、缺少成员、空名称及超字节长度。浏览器操作实际下拉选择两位成员并指定领队。 | 通过 |
| AC-TEAM-SETTINGS-03：原成员身份、独立指引、绑定和责任标签保留；新成员复制所选目录专家；保留既有核心能力并使用版本 CAS。 | COD-002/006、TST-002、G-WEB | 保存参数回归验证稳定 ID、原定义、标签、元数据和版本；来源专家已删除时仍能保留原成员。桌面浏览器验证编辑保存参数、新增成员和切换领队。 | 通过 |
| AC-TEAM-SETTINGS-04：仅维护权限的管理员可编辑；加载失败可重试，保存失败保留草稿，禁止重复提交与关闭后的迟到通知。 | UI-002/004、TST-002、G-WEB | 普通用户、不可变团队、非所有者只读；失败、重试、忙碌与关闭状态回归。服务端继续复用现有权限检查。 | 通过；本轮未重跑真实数据库权限验收 |
| AC-TEAM-SETTINGS-05：桌面和手机可操作，四项设置不展开全部技能或连接器列表。 | UI-003/004、G-WEB | Playwright CLI，1440×900 和 390×844，实际 Vue 组件及样式，隔离预览注入受控 API。截图位于本地忽略目录 `output/playwright/team-settings-desktop-20261010.png`、`team-create-mobile-20261010.png`、`team-catalog-mobile-20261010.png`。 | 通过；仅界面验证 |

已执行：

- `pnpm -C frontend exec vitest run --maxWorkers=2`：58 个文件、661 项通过。首次失败来自测试使用错误的中文字段名称、组件 wrapper 类型和 Vue proxy 身份比较，修正断言后完整回归通过。
- `make web-typecheck`、`make web-build`：通过。构建仍报告既有大 chunk 提示。
- `go -C backend test ./internal/systemskills ./internal/resourceaction ./internal/expertpackage`：通过；本轮 Go 改动仅更新创建 Skill 的完成说明。
- `git diff --check`、`cmp -s AGENTS.md CLAUDE.md`：通过。新增及修改的产品、技术和本证据文档经中性命名检查。

没有 API、持久化、迁移或 Runtime 修改。本轮未执行真实 PostgreSQL 集成、认证服务端浏览器验收、模型协作、Linux Sandbox 或 Production Conformance；上述隔离预览不证明这些能力或线上发布。本轮尚未部署。
