# Workflow 产品闭环部署验证 — 2026-09-29

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布标识

- 集成分支：`main_temp`，部署源 Commit：`8d89c96a647e28850d327ae7c4fc9de678812ef3`。
- 本轮 Workflow 实现 Commit：`e7d6f3f47dcdf4ec2b9373ecf67126a961b0d934`。
- 发布 ID：`platform-20260929T043925Z`。
- 源目录：`/srv/agent-workspace/src.release-platform-20260929T043925Z`。
- Web 目录：`/srv/agent-workspace/web/releases/platform-20260929T043925Z`。
- 发布前备份：`/srv/agent-workspace/backups/pre-platform-20260929T043925Z`。
- 公网入口：`https://workspace.example.com`。

## 已验证

- `main_temp` 合并无冲突；`make test`、`make build`、`pnpm --dir frontend test`（46 个文件、351 条测试）、`make web-typecheck` 和 `make web-build` 均通过。正式 `make deploy` 再次执行其内置门禁，未设置 `SKIP_DEPLOY_GATES`。
- 正式发布脚本创建业务库、身份库和配置备份；两个 `pg_restore -l` 检查及 `SHA256SUMS` 校验通过。
- Runtime 与 CLI Builder 镜像 smoke test 通过；API、Worker、Egress Controller 和 Caddy 已从新源目录启动，健康检查通过；最新 Migration `000065_smart_assistant_controlled_publication.sql` 在账本中恰有一条记录。
- 公开 `/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`，Web 返回 HTTP 200；发布脚本还验证了 OIDC discovery、HTTP 到 HTTPS 跳转、容器健康和所检查服务日志中无 panic/fatal/error 级记录。
- 未认证请求 `POST /api/v1/workflows/schedule-preview` 与 `DELETE /api/v1/workflows/{workflow_id}/api-credential` 均返回 HTTP 401，确认新路由在公网可达且受身份保护。
- 部署源中的 Workflow Proto 和列表页面源码与集成工作区 SHA-256 一致；线上 `index.html` 与本地产物 SHA-256 均为 `d3fb61fa1acee356ae097116f59061b862bccf28a864ccddb386be7f0c18b249`。

## 运维与证据边界

发布前删除了无容器引用的退休 AnythingLLM 镜像，并清理未使用的 Docker 构建缓存；发布后磁盘剩余约 5.8 GB。未删除数据库、对象存储或现役服务数据。

本记录证明发布源、备份、迁移、服务与 Web 健康、静态资源一致性及新路由保护。尚未用真实企业账号完成 Workflow 创建、二次运行、Schedule 触发、Credential 轮换/撤销或浏览器端到端验收；Runtime 与 CLI Builder smoke test 也不等于完整 Linux + gVisor Production Conformance。
