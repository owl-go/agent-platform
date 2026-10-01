# 连接器分类与弹窗布局部署验证 — 2026-10-01

## 发布标识

- 集成分支：`main_temp`，部署源 Commit：`dfa9f01593320074be3fb155ad4597e4e99e8b53`。
- 功能 Commit：`8cd9bc8`；合并无冲突。
- 发布 ID：`connector-layout-20261001T073440Z`。
- 源目录：`/opt/agent-platform/src.release-connector-layout-20261001T073440Z`。
- Web 目录：`/opt/agent-platform/web/releases/connector-layout-20261001T073440Z`。
- 发布前备份：`/opt/agent-platform/backups/pre-connector-layout-20261001T073440Z`。
- 前一版本：`connector-guides-20261001T062000Z`。
- 公网入口：`https://47-237-108-63.sslip.io`。

## 已执行的检查

- 正式 `make deploy` 成功，未设置跳过门禁选项；执行 `make test`、`make build`、完整前端 Vitest、`make web-typecheck`、生产 Web 构建及 `git diff --check`。
- 完整前端 Vitest：46 个文件、413 个用例通过。分类、安装标记、居中弹窗、断开保留安装、按当前 Version 卸载、既有授权路径和使用指引草稿行为的本地验证见同日 `connector-catalog-layout-local-validation` 记录。
- 业务库、身份库备份通过 `pg_restore -l`；业务库、身份库、配置、前一版源/Web 指针通过 `SHA256SUMS` 校验。
- 远端统一 Runtime 和 CLI Builder 镜像 smoke 检查通过。候选 Runtime 的连接器复验入口正常退出，日志为 `verified 0 active CLI Connector bundles for configured Runtime`；没有新增 bundle 复验证据。
- API、Worker、Egress Controller 均运行且健康，Caddy 运行；四个服务的 Compose 工作目录均属于本次源目录。
- 发布脚本验证最新 Migration 恰有一条记录、OIDC discovery、HTTP 到 HTTPS 跳转，以及所检查服务日志没有 panic/fatal/error 级记录。
- 从本机访问公网 `/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`。
- 线上五个相关源文件（详情、连接器目录、页面、i18n、样式）SHA-256 与本地集成工作区一致。
- 本地产物、远端 Web 当前目录和公网下载的 `index.html` SHA-256 一致：`a056ffb9f675c5d259931cfa832fda6762250831c4ae919549ad51648eabcc80`。
- 公网资源中心 JavaScript 和主样式文件正常返回，其 SHA-256 与生产构建产物一致。

## 验证边界

本记录证明发布门禁、备份、部署源、服务健康和公网产物一致性。桌面/手机布局以及弹窗键盘行为使用实际 Vue 组件和本地 API Fixture 验证；未在发布后使用真实账号完成连接器浏览器操作或外部调用，也未运行完整 Production Conformance。本轮没有改变后端、Runtime Capability、Connector bundle 或账号授权契约。
