# 2026-10-06 日常一键部署验收

## 发布结果

- 从已集成的 `main_temp` 执行 `make deploy`，完整发布成功。
- 代码 Revision：`a51bbe36f009b9d4f54e9fe94fd82815b64866db`。
- 源码发布：`/opt/agent-platform/src.release-app-20261006T095835Z-c9c06eb0`。
- API Image ID：`sha256:be501f4fa8a5c08278c7c9893843917e97a16d1af84ce59d3d23d229504e32ef`。
- Worker Image ID：`sha256:def57b8d45f55ef98d8b2852989b918ad1b3f61e2b92d03d4827eb120bcb5536`。
- 原 Web 发布继续使用 `/opt/agent-platform/web/releases/execution-default-direct-save-20261006-1`；公网 HTML 与入口引用的静态资源逐字节匹配服务器当前发布。
- HTTPS、API Health、API Readiness、Worker health 和 OIDC discovery 均通过。
- 发布前后 `platform.env` 与 `platform.https.yaml` 字节摘要相同。Identity、Caddy、Egress Controller 的镜像和启动时间相同；日常入口没有构建、推送或修改 Runtime / CLI Builder。

## 已执行检查

- `make deploy-test`：22 个测试通过；两个部署 Shell 入口语法检查通过。
- `go -C backend test ./internal/architecture ./internal/conformancepreflight` 通过。
- 一键入口对集成快照执行 `make test`、`make build`，均通过；本轮未设置 `WORKSPACE_TEST_POSTGRES_DSN`，数据库集成测试会 Skip，不记为新的数据库集成验收。
- 业务与身份 PostgreSQL 均完成部署前 `pg_dump -Fc`，通过 `pg_restore -l` 检查可读，配置/恢复记录和摘要保存在服务器受限的 `backups/pre-app-20261006T095835Z-c9c06eb0`。
- `git diff --check`、部署文档本地链接检查、`cmp -s AGENTS.md CLAUDE.md` 通过。
- 同一版本再次运行 `make deploy`，输出“应用无改动，无需重启”；之后核对所有五个受检查容器的 Image ID 与 StartedAt 完全相同。

## 发布中修正的问题

旧架构测试改为检查独立的基础设施升级入口，保留 Runtime Digest、Smoke 和 CLI Connector 切换前验证约束。旧 Conformance 拒绝测试使用模拟 Docker 命令与 30 秒 Context，避免访问工作站真实 Docker / CLI 插件。

工作站构建曾因磁盘不足中止，清理三天前的可再生 Go 构建缓存后恢复；入口现在在构建前检查空间。Git HTTP/2 曾停滞；集成拉取使用 HTTP/1.1 与低速超时，本轮推送使用相同协议覆盖。

目标数据库保留四个不在现行源码中的历史 Migration。入口现在检查现行源码的每个 Migration SHA-256，并保证原有历史记录没有变化；不删除历史账本。失败时、账本未变化的自动应用恢复已经在真实服务器执行。

首次恢复实现误把 Web current 写为宿主绝对路径，使 Caddy 的 `/srv/web` bind mount 无法解析，页面曾返回 404。已恢复原相对链接，并将切换/恢复统一改为相对链接；新增测试把完整 Web 目录复制到另一容器挂载路径验证解析，最终发布与重复发布的公网页面和资源均验证通过。

## 范围

覆盖现有单服务器安装的日常应用更新、一次保存的部署配置和短版文档。Web 单独发布通过模拟远端文件系统与命令验证，本轮线上没有重建 Web。全新服务器供应、Linux + gVisor / Runtime Capability 的完整 Conformance、真实模型执行与手机扫码登录不是本轮新增验收；能力保持现有受控配置。
