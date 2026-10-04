# Workflow Message Channel 配置入口验证 — 2026-10-03

工作流设置的消息渠道区域在未保存账号时直接展示 Telegram、Discord、Slack、钉钉、飞书/Lark、Matrix、WhatsApp、Signal、企业微信、微信、QQ Bot、BlueBubbles 和元宝共 13 个配置入口。点击入口打开固定渠道的凭证与接收范围表单；飞书表单保留飞书/Lark 区域选择。已有账号仍单独展示，支持同一渠道配置多个账号。

表单显式引入 Element Plus Form/FormItem 及样式，凭证保持掩码输入，关闭后再次打开不会保留凭证草稿。平台关闭接入时入口仍可见但不可点击。补齐未启用状态翻译，集成最新 `main_temp` 已有的刷新翻译。

## 实际执行

- 目标组件和工作流页面测试：2 个文件、64 项通过。
- `pnpm --dir frontend test`：52 个文件、549 项通过。新增测试使用真实 Element Plus 表单，逐一检查 13 个渠道的必需字段和关闭后的凭证清除，并检查实际工作流页面打开 Slack 配置。
- `make web-typecheck`、`make web-build`、`git diff --check`：通过。构建保留既有大 chunk 提示。
- Playwright Chromium 检查生产构建：通过工作流设置折叠区原生点击展示 13 个入口，再通过 Slack 入口打开包含 Bot Token、Signing Secret 和接收范围的表单。390×844 手机视口无横向页面溢出，弹窗的取消/保存按钮在视口内可见，长表单可滚动。检查使用本地只读 API fake，没有访问真实账号或发送消息；临时检查入口未纳入正式构建或提交。

截图中的旧版本空白弹窗没有在独立生产构建中稳定复现，因此不将组件解析或样式问题记为已证实根因。本次交付依据是新的直接配置行为、真实表单回归测试和工作流设置页浏览器验证。

## 验证边界

本次仅修改 Web 配置入口和表单展示，没有修改 API、Worker、Migration 或 Runtime。未新增真实渠道凭证，未执行供应商消息收发闭环或新的 Linux Sandbox/Production Conformance。

## 发布与公网核对

- 功能提交：`b35d62c`；集成发布源：`main_temp` 的 `cac94dda53a49db125d20d1558da3ac576dd59b5`。
- 集成后再次运行 52 个前端文件、549 项测试及 `make web-typecheck`，全部通过。
- 使用服务器当前公开 OIDC 配置执行 `WEB_DEPLOY_HOST=agent-platform WEB_RELEASE_ID=workflow-channel-config-20261003-1 make web-deploy`，正式生产构建与发布通过，约 11:33 UTC 完成公网验证。
- 当前 Web：`/opt/agent-platform/web/releases/workflow-channel-config-20261003-1`；前版 `releases/workflow-settings-20261003-1` 指针记录在服务器 evidence 目录，原发布目录保留。
- 公开入口：`https://47-237-108-63.sslip.io`。公网下载的 `index.html` 和全部 80 个 JS/CSS 文件均与本地发布构建 SHA-256 一致。页面 SHA-256 为 `0f753aa90ca0e18464cd2cfce724999db0032e61552ce2a2ad81a8794a41bca2`；工作流页面 JS 为 `b72c1924a753231970c18841dee22de8426339b6056318711778d58ac395baa8`，CSS 为 `55fbe5d0af2feea5d160dd9c41edd8c1a58c3c8f1312738e7b96f9c76d402e53`。
- 公网 `/api/healthz` 返回 `ok`，`/api/readyz` 返回 `ready`；API、Worker、Egress Controller 均 healthy。本次没有重新创建后端容器或发布 Runtime 镜像。服务器磁盘为 71% 使用、12 GiB 可用。
- 本机日志：`/tmp/agent-platform-wmc-ui-deploy.log`、`/tmp/agent-platform-wmc-integrated-test.log`、`/tmp/agent-platform-wmc-ui-public-verification.log`、`/tmp/agent-platform-wmc-ui-server-verification.log`。资源 Hash 和截图保存在服务器 `/opt/agent-platform/evidence/workflow-channel-config-20261003-1`。

浏览器交互验收针对本地生产构建及只读 API fake；线上本次核对发布资源与服务健康，没有使用真实用户身份进行渠道配置，也没有将公网健康检查等同于真实 IM 验收。
