# 专家常用任务直接发送（2026-10-10）

用户要求点击专家详情中的常用任务，直接向会话发送该文本并使用该专家。沿用 `codex/expert-team-refactor`，实现于 CatalogDetails，专家团共享相同交互。普通“召唤”仍只选择资源；用户原会话草稿不参与此新会话。

## 行为与验收

- AC-TASK-01：点击常用任务，新建选中该专家或专家团的 Session，读取服务端 Conversation Selection，以其修订 ID 提交文本。服务端接受后打开该 Session，沿用消息加载与流式展示。
- AC-TASK-02：发送期间展示本地化状态、禁用任务及底部操作，并保留弹窗。创建或发送失败展示可读错误；显式重试复用已创建的 Session。已接受的消息在导航失败后不重发。不可用专家和专家团不能发送。
- AC-TASK-03：任务使用原生 button，中文与英文均有直接发送的可访问名称。沿用全局焦点样式、设计 Token 和弹窗响应式宽度。
- AC-TASK-04：会话入口避免刷新后再次打开同一 Session，保留专家名称并避免重复开始读取响应。打开已接受的任务不再次创建或提交消息。

AC-TASK-01/02/04 对应 UI-002、COD-002/006、AC-UI-03/05，AC-TASK-03 对应 UI-001/003/004/005、AC-UI-01/06/07。G-WEB 的组件回归及实际视口操作结果如下。

## 本地验证

- 初次 `pnpm --dir frontend test -- ...` 实际执行完整前端套件，59 个文件、684 项测试通过。新增会话入口回归后，目标检查首次失败：同一 Session 在 refresh 和 open 查询入口重复打开，清空专家名称；修复重复打开并补齐测试附件 Adapter 后，`pnpm --dir frontend exec vitest run src/components/CatalogDetails.test.ts src/pages/SessionsPage.test.ts` 的 2 个文件、69 项测试通过。
- 最终 `make web-typecheck` 与 `make web-build` 通过。构建保留既有大于 500 kB chunk 提醒，无新增依赖。
- Chromium + Playwright CLI 使用实际 CatalogDetails 和合成 API Adapter 做本地视觉/交互验收：1440×900 点击首条任务，回执为一次创建、一次提交，文本、expert_id 与 selection_id 对应；390×844 用 Tab 到达首条任务、Enter 提交，注入网络失败后弹窗保留文本与错误，点击重试的回执为一次创建、两次提交尝试、最终接受。
- 390×844 中文与英文长文案无横向溢出；检查原生按钮焦点轮廓，Esc 关闭后焦点返回打开弹窗的按钮。截图位于忽略目录：`output/playwright/expert-tasks-desktop.png`、`expert-tasks-mobile-keyboard.png`、`expert-tasks-mobile-failure.png`、`expert-tasks-mobile-english.png`，已逐项检查布局。
- 本地 Fixture 只有 favicon 404 控制台记录。Fixture 文件和 Vite 进程已清理，合成数据没有写入生产。英文 Fixture 的专家文案仅用于长文案布局，不代表产品新增了自动翻译或语言切换。
- G-DOC：`git diff --check` 与 `cmp -s AGENTS.md CLAUDE.md` 通过。

以上浏览器验收使用合成 API，不是认证 API E2E、真实模型执行或生产账号验收；未运行完整 Docker E2E。未进行读屏、真实软键盘或全量对比度测量。

## 发布

代码与证据将经 `main_temp` 集成，发布及线上静态资源核验结果在实际执行后补记。
