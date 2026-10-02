# 自动登录入口部署验证 — 2026-10-02

## 发布标识

- 功能分支：`codex/login-style-refresh`，功能 Commit：`10c832c`、`bd2a65f`。
- 集成分支：`main_temp`，部署源 Commit：`05d44769c8a4`，合并无冲突。
- Web 发布 ID：`login-entry-05d44769c8a4`。
- Web 目录：`/opt/agent-platform/web/releases/login-entry-05d44769c8a4`。
- 发布前检查所见 Web 版本：`tianyancha-region-20261002T1335`，保留供回滚。
- 公网入口：`https://47-237-108-63.sslip.io`。

## 已执行的检查

- 功能分支的认证与 App 目标测试通过；完整前端测试 46 个文件、422 个用例通过。
- 合并后的 `main_temp` 执行 `pnpm --dir frontend exec vitest run --maxWorkers=2`：46 个文件、471 个用例通过。
- 集成工作区的 `make web-typecheck`、`pnpm --dir frontend typecheck:e2e`、`git diff --check` 通过。
- `scripts/deploy-web.sh` 使用线上公开 OIDC 配置完成生产构建、不可变目录上传、文件检查和 `current` 原子切换；没有跳过构建。
- 本地产物、远端当前 Web 目录和公网 `index.html` SHA-256 一致：`f9dd73d45e691b721b7baad3485e31340a8713354759e49f7d5d29254b372c3a`。
- 公网入口主 JavaScript 和样式文件均正常返回，SHA-256 与本地生产构建产物一致。
- 线上 `/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`；OIDC discovery 的 issuer 与配置一致。
- Playwright 使用新浏览器访问公网入口，未点击任何产品登录按钮即到达 Keycloak `/protocol/openid-connect/auth`。页面包含 `#username` 和 `#password`，不存在产品内 `.auth-card` 登录入口。

## 验证边界

本次仅发布静态 Web。未修改或重启 API、Worker、Keycloak、Runtime、数据库或连接器包。

有效会话直接恢复、过期会话自动跳转、缺失会话自动跳转、静默续期成功与失败、回调失败不循环跳转和退出失败不暴露受保护状态，均由实际认证代码的单元测试覆盖。本轮线上浏览器只验证未登录入口的自动跳转，未输入真实账号密码，未完成真实账号的回调、刷新恢复或失效重登验收。

本地 `node scripts/e2e/run-local.mjs --grep 'E2E-001'` 在环境启动阶段因 MinIO 镜像拉取被拒绝而终止，新增的真实 Keycloak 会话恢复及失效跳转 E2E 未执行，不能记为通过。完整 Runtime Production Conformance 与本次静态 Web 改动无关，未执行。

发布日志和前端测试日志保存在集成工作区的忽略目录 `output/playwright/login-release/`；浏览器检查没有保存账号凭证。
