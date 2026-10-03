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
