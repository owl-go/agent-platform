# 资源目录加载状态部署验证 — 2026-10-02

## 发布标识

- 集成分支：`main_temp`，Web 部署源 Commit：`5cd3ab09ae8e2e77b5aa91306fe9ee15cc7658bd`。
- 功能 Commit：`b46ee31`、`8776768`；合并无冲突。
- Web 发布 ID：`resource-loading-20261002T070745Z`。
- Web 目录：`/opt/agent-platform/web/releases/resource-loading-20261002T070745Z`。
- 前一 Web 版本：`legal-connectors-20261002-1.0.0`。
- 回滚记录目录：`/opt/agent-platform/backups/pre-resource-loading-20261002T070745Z`，保存前一 Web 指针与入口文件，二者通过 `SHA256SUMS` 校验；前一完整 Web 版本保留于不可变 releases 目录。
- 公网入口：`https://47-237-108-63.sslip.io`。

## 已执行的检查

- 在独立发布工作区从最新 `origin/main_temp` 集成，只引入本次七个前端、测试和产品规格文件的改动，并推送集成 Commit 后发布。
- `pnpm --dir frontend install --frozen-lockfile` 成功。
- `pnpm --dir frontend test`：46 个文件、491 个用例通过，包括四类资源目录首次请求未完成时显示骨架屏、不提前显示空状态，以及请求失败结束加载并显示错误。
- `make web-typecheck`、`git diff --check` 通过。
- 通过仓库 `make web-deploy` 执行带生产 OIDC 配置的 typecheck 和 Vite build、不可变 Web 上传、远端文件校验及 current 指针原子切换。
- 本地产物、远端 Web 当前目录和公网返回的 `index.html` SHA-256 一致：`b61d865a7e19b4abcadb54ba36542e54c0c898e24fbf1e1ce8d44c2e4edb7f6b`。
- 入口引用的全部静态资源，以及资源中心 JS/CSS，在公网均返回 HTTP 200，SHA-256 与本地产物一致。资源中心 JS 包含 `catalog-loading`，主样式包含骨架屏和减弱动效规则。
- 公网 `/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`；OIDC discovery 的 issuer 与生产 HTTPS 配置一致。
- API、Worker、Egress Controller 保持健康，Caddy 运行；本次静态 Web 发布未重启这些服务。服务源目录仍为 `/opt/agent-platform/src.release-legal-connectors-20261002-1.0.0`。

## 回滚

```bash
WEB_DEPLOY_HOST=agent-platform scripts/deploy-web.sh activate legal-connectors-20261002-1.0.0
```

## 验证边界

本记录证明前端集成门禁、静态 Web 发布、回滚记录、公网产物一致性和既有服务健康。开发阶段已使用实际 Vue 组件与注入的 API Fixture 验证 1440px/390px 下的加载状态、无横向溢出，以及减弱动效和英文加载标签。发布后未使用真实账号完成四类目录的浏览器加载链路；该交互由本地组件测试覆盖。

本次没有发布后端、数据库 Migration、Runtime 镜像或 Connector bundle，因此未执行完整平台 `make deploy`、后端测试/构建、Runtime smoke 或 Production Conformance，也没有新增这些层级的验收证据。
