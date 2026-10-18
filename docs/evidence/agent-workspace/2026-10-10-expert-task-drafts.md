# 专家任务卡片与草稿入口（2026-10-10）

用户修正交互要求：常用任务点击后只放入输入框，不自动发送或启动响应；展示参考用户图 2 的浅灰卡片。该决定取代此前直接提交任务的行为；原发布与验证记录保留于 [此前证据](2026-10-10-expert-common-task-launch.md)。沿用 `codex/expert-team-refactor`。

## 当前行为

- AC-DRAFT-01：点击 Expert 或 Expert Team 任务卡，复用既有 Session 启动参数，选中该专家/专家团，把原任务文字填入可编辑 Composer 并聚焦。卡片上的引号仅用于展示，不进入草稿；没有自动消息提交或执行。
- AC-DRAFT-02：用户修改草稿并手动点击发送才提交；普通 Summon 只选择专家，不填任务。不可用资源禁用任务。等待导航时防止重复点击；导航异常或守卫取消后保留详情与本地化错误，可以重试。
- AC-DRAFT-03：任务为原生按钮卡片，无项目符号、蓝色链接或下划线。浅灰底、深灰文字、圆角、间距及悬停边框沿用设计 Token，长文本自动换行，保留键盘焦点样式。

对应 UI-001–005、COD-002/006、AC-UI-01/03/05/06/07，选择 G-WEB、G-DOC。没有后端、数据库、Runtime、Sandbox 或包契约变更。

## 验证

- `pnpm --dir frontend exec vitest run src/components/CatalogDetails.test.ts src/pages/SessionsPage.test.ts src/components/ConversationComposer.test.ts src/i18n/index.test.ts`：4 个文件、116 项测试通过。覆盖专家/专家团预填、手动编辑后发送、普通召唤、不可用状态、重复导航、异常重试与守卫取消。相邻测试中 Window.open 的 jsdom 提醒不代表真实弹窗验收。
- `make web-typecheck`、`make web-build` 通过；构建保留既有 500 kB chunk 提醒，没有新增依赖。
- Chromium + Playwright CLI 使用真实 CatalogDetails、SessionsPage、Composer 和合成 API Adapter。在 1440×900 点击卡片后，记录一次 Session 创建、零次消息提交，专家选择和预填原文正确且输入框已聚焦；编辑并点击发送后记录一次提交，内容为编辑后的文字。
- 390×844 中文卡片使用 Tab/Enter 打开后仍为零次消息提交，输入框聚焦且保留任务文字。英文长文本在相同视口检查无横向溢出；实际按钮文字颜色为 rgb(75,85,99)，背景为 rgb(245,247,250)，来自现有 Token。
- 已检查忽略目录中的截图：`output/playwright/expert-draft-cards-desktop.png`、`expert-draft-cards-mobile.png`、`expert-draft-cards-mobile-english.png`、`expert-prefilled-draft-desktop.png`。卡片和草稿分别核对视觉与操作，未把截图当作 API 验收。首次 CLI 选中空白标签页后重新选定 Fixture，最终截图已替换为实际受测页面；Fixture 只有 favicon 404 记录。
- 本地 Fixture、Vite 进程和专用浏览器已清理；合成数据不写入生产。`git diff --check`、`cmp -s AGENTS.md CLAUDE.md` 通过。

未执行认证 API E2E、真实账号手动发送或模型执行验收；未测读屏、真实软键盘或全站对比度。英文 Fixture 只用于长文案布局，不代表新增专家自动翻译。

## 发布

功能提交 `c48121f` 已推送并经 `main_temp` 集成。第一次 `make deploy` 在切换前被 2 GiB 可用空间门禁阻止，可用空间为 2,003,763,200 字节，原版本仍在运行。只执行 `docker builder prune --force` 清理未使用的构建缓存，Docker 报告回收 3.856 GB；随后可用空间为 5,859,905,536 字节，API 与 Worker 仍健康。没有删除备份、发布目录、镜像、容器或数据卷。

重试原发布命令成功，固定来源为 `c48121fc4373da4cecc66aa9ecdba2b2e28315c3`，发布编号 `app-20261010T113738Z-ff3ce29f`，只更新前端。只读脚本 `output/playwright/verify-expert-draft-release.py` 核实来源与发布指针，并检查目录组件包含 draft 参数、新卡片结构及本地化任务入口，同时已移除直接调用 sendSessionMessage 的逻辑。

公共 HTTPS 健康、Readiness、OIDC 和入口资源检查通过；从公网回读 `ResourceCenterPage-BRhcnRiX.js`、`index-CyMnwV8x.css` 并与实际发布逐字节比较一致。API 与 Worker 均为 healthy，后端镜像和来源摘要通过既有核验。上述公共资源检查不替代真实账号发送验收。
