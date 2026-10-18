# 移除连接器后的 Composer 显示（2026-10-10）

用户反馈移除飞书后，Composer 仍显示图标及“未用于当前对话”。沿用 `codex/expert-team-refactor`，修改共享 Composer 的显示投影；Conversation Selection、授权及历史执行契约不变。

## 行为与原因

- AC-REMOVE-01：显式或专家/专家团继承的连接器被当前对话排除后，工具栏隐藏图标。此前合并继承及显式绑定的列表仅去重，没有过滤 `disabled_connectors`，因此执行已排除而显示仍保留。
- AC-REMOVE-02：排除记录继续保存和恢复，重新打开后保持移除状态；可从“＋ → 连接器”重新添加，授权不可用时仍走现有恢复流程。
- AC-REMOVE-03：移除失败保留原选中图标并显示重试提示；不同 Workflow Run Conversation 的选择互不影响，不修改可复用专家或账户启用状态。

对应 UI-002–005、COD-006、TST-001–003/005，选择 G-WEB/G-DOC。没有后端、持久模型或 Runtime 变更。

## 验证与恢复

- 修改实现前执行 `pnpm --dir frontend exec vitest run src/components/ConversationComposer.test.ts -t 'hides a removed Connector'`，最初 Session 与 Run 两项失败，扩展到继承/显式绑定后四项均失败：期望图标不存在，实际仍存在。空本地草稿同样复现，排除了必须由旧草稿触发的假设。
- 修复后 Composer 目标测试通过。补充失败保存用例首次断言使用了错误的提示文本，改为现有本地化重试文案。完整测试首次发现 Workflow 页面原用例仍断言保留关闭图标，按最新交互更新为隐藏，并检查返回另一个 Run 后仍保持原选择。
- 最终 `pnpm --dir frontend exec vitest run`：59 个文件、695 项通过。覆盖 CLI/MCP、显式/继承、移除/重选/草稿重开、授权恢复、保存失败与 Run 间隔离。jsdom 的 Window.open、导航与既有缺失 prop 提醒不代表真实供应商或浏览器验收。
- `make web-typecheck`、`make web-build` 通过；保留既有大于 500 kB chunk 提醒。`git diff --check` 和 `cmp -s AGENTS.md CLAUDE.md` 通过。
- Chromium + Playwright CLI 使用真实 Composer 和合成 API：1440×900 中文 Session 移除后图标消失，从列表重新添加后出现，再次移除并 reload 后保持隐藏；390×844 英文 Run 移除并 reload 后保持隐藏。截图 `output/playwright/connector-removed-desktop.png`、`connector-removed-mobile-english.png` 已检查，保留专家选择且输入区可用。首次点击 Element Plus 的隐藏 switch input 超时，改为可见 switch 外层后完成实际操作；两个 scope 各自草稿保留，分别移除后验收。
- 临时 Fixture、专用浏览器和 Vite 进程已清理，合成数据没有写入生产。

未执行认证 API E2E、真实供应商授权或模型执行验收；本次投影修复不声称后端、数据库或镜像 Conformance 通过。生产发布及公共资源检查见下节。

## 发布

功能提交 `0e333b62ad8caef2186fba31587f8e32fa1655f1` 已推送并经 `main_temp` 集成。`make deploy` 成功发布 `app-20261010T121813Z-de0a8ce3`，只更新前端，健康、Readiness 和 OIDC 检查通过。

只读脚本 `output/playwright/verify-connector-removal-release.py` 检查来源 Commit 与前端发布指针一致，部署的 Composer 代码实际包含 `disabled_connectors` 排除过滤。从公网回读 `index-Dh6dg4cF.js` 并与服务器发布文件逐字节一致，SHA-256 为 `a69f9dbbb6e9c825c28f45b6014f69895d22b8005e974515d85fa25fb8198a59`。API 和 Worker 均为 healthy，既有后端镜像与来源核验通过。公共文件验证不替代真实账号会话和 Runtime 验收。
