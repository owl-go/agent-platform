# Workflow Message Channel 应用图标 — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

全部 13 个渠道入口使用固定 32px 应用图标；已保存账号和配置弹窗标题使用对应的 24px 图标。图标为装饰性图片，保留现有按钮名称与文本。应用资源本地打包，来源及许可记录在 `frontend/src/assets/message-channels/SOURCES.md` 和相邻 LICENSE 文件；飞书、钉钉复用既有资源。

## 实际验证

- 目标组件和工作流设置页：2 个文件、64 项测试通过。
- 前端完整测试：52 个文件、549 项通过；集成到 `main_temp` 后再次全量通过。
- `make web-typecheck`、`make web-build`、`git diff --check` 通过；最终 PNG 资源替换后再次通过类型检查和生产构建。构建保留既有大 chunk 提示。
- Playwright 使用与正式入口共用 Vite 配置的临时生产构建检查。全部 13 张图片完成加载且 `naturalWidth > 0`，入口尺寸均为 32px；390×844 视口页面宽度为 390px，无横向溢出；Slack 配置窗口图标、字段和操作按钮正常。截图检查涵盖桌面与手机布局。临时入口和构建均已清理，没有进入正式发布。

## 发布

- 功能提交：`6de06fa`；发布集成源：`main_temp` 的 `2bd4f3d44db595180978ca3b19fd0aaf2b2ec98f`。
- 使用当前公开 OIDC 配置执行 `WEB_DEPLOY_HOST=agent-platform WEB_RELEASE_ID=workflow-channel-icons-20261003-1 make web-deploy`，发布成功。
- 当前 Web 为 `/srv/agent-workspace/web/releases/workflow-channel-icons-20261003-1`；前版为 `releases/workflow-channel-config-20261003-1`，目录保留，指针记录在 evidence 目录。
- 公网 `https://workspace.example.com` 下载的页面、主 JS、工作流页面 JS/CSS 和 BlueBubbles PNG 均与本地构建 SHA-256 相同。页面 SHA-256 为 `bc4fc8f0b166703af7cabac188935b0781ab9b2634afbebe0f93c00ea272c6fa`。其余小图标由 Vite 内联到工作流页面 JS。
- 公网 `/api/healthz` 返回 `ok`，`/api/readyz` 返回 `ready`；API、Worker、Egress Controller 均 healthy。本次仅发布 Web，没有修改后端或重新构建 Runtime 镜像。
- 构建、验证日志位于本机 `/tmp/agent-platform-wmc-icon-*.log`；资源 Hash、部署后检查、截图保存在服务器 `/srv/agent-workspace/evidence/workflow-channel-icons-20261003-1`。

浏览器检查使用只读 API fake，没有新增真实渠道账号或发送消息；本次视觉变更没有新增真实 IM 或 Runtime Conformance 验收证据。
