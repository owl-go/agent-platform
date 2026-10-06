# 飞书消息渠道凭证简化发布验证 — 2026-10-04

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布标识

- 功能分支：`codex/feishu-channel-credentials`；功能提交：`c2fa518`。
- 集成分支：`main_temp`；发布构建源：`e8c441b4ffd5421f6c981fc74044ff31f27bcf05`。
- 发布 ID：`feishu-channel-20261004-1`。
- 公网入口：`https://workspace.example.com`。
- 源目录：`/srv/agent-workspace/src.release-feishu-channel-20261004-1`。
- Web 目录：`/srv/agent-workspace/web/releases/feishu-channel-20261004-1`。
- 发布前备份：`/srv/agent-workspace/backups/pre-feishu-channel-20261004-1`。

飞书/Lark 自建应用只要求 App ID、App Secret。API 通过应用 Token 查询企业信息，自动保存 Tenant Key；旧凭证中手填的 Tenant Key 不作为认证身份依据。查询失败、返回空值或接入验证时企业身份改变均拒绝。事件 Header 和 Sender 的企业归属校验保留。

集成时保留 `main_temp` 已有的渠道配置卡片和服务端账号授权流程，字段定义更新到 `messageChannelSetup.ts`，测试覆盖中英文“连接账号→保存授权”的实际表单路径。

## 实际执行的门禁

功能分支通过目标包和组件测试、`make test`、`make build`、`make web-typecheck`、`make web-build`、`git diff --check`。

集成分支安装固定前端依赖，实际通过：

```bash
go -C backend test ./internal/data/messagechannel/...
pnpm --dir frontend test src/components/WorkflowMessageChannels.test.ts
make test
make build
pnpm --dir frontend test
make web-typecheck
make web-build
git diff --check
```

目标组件为 24 项测试通过。后端覆盖飞书/Lark 自动查询、旧手填值无法覆盖身份、缺失凭证、供应商失败/非法响应、企业漂移及跨企业消息拒绝。完整 Go 测试未配置临时 PostgreSQL 或远端对象存储集成环境，不将依赖环境的 Skip 计为集成验收。

## 备份与服务切换

业务库及身份库 `pg_dump -Fc` 均通过 `pg_restore -l`；数据库、配置归档、旧源码/Web 指针、API/Worker 镜像记录及 Migration ledger 通过 SHA-256 检查。旧服务镜像另保留 `pre-feishu-channel-20261004-1` 标签。

只构建 API、Worker 和 Web。候选 API 在私有网络通过 `/readyz` 后清理；切换前非终态 Run 与活跃 Session Message 汇总为 0。停止旧 Worker 后替换服务，等待 healthy。配置文件 SHA-256 与全部 Migration name/checksum 在候选验证和切换后均相同。

核对时发现并行 QQ Gateway 发布重新切换了 Worker 和源码指针。已重新将 Worker 和源码指针统一到本发布目录；构建源包含此前 QQ Gateway 修复。最终核对 API 和 Worker 的镜像标签及 Compose 工作目录均属于本发布：

| 服务 | 最终运行镜像 ID | 状态 |
|---|---|---|
| API | `sha256:c8b2e266598a76e60b68bb1dddc7f50ef6445601261b704c147834c4ac5c9ec2` | healthy |
| Worker | `sha256:a78f29277f13343d33e407f3e088bebbd3c0e7116bd4e521ec7ff8425d5937b5` | healthy |

两项服务启动日志 ERROR/FATAL/PANIC 匹配数均为 0。消息渠道 connections、delivery、inbox、maintenance 四个循环均 started=1、fatal=0。Runtime、CLI Builder 与基础服务镜像没有重建，配置没有修改。

## 公网产物核对

公网首页、Health、Readiness、OIDC Discovery 均为 HTTP 200。公网 HTML、工作流详情、i18n 与入口 JavaScript 字节和本地生产构建一致。

- HTML SHA-256：`9e0e2d5b9ed59225577a33014dbc2f6c1ea79b9c532644f8df6651f5cf155168`。
- `WorkflowDetailPage-tQfeHqEg.js`：`28c3d7bd2f09d6156d8637c490e7f6090be741547cac5315334f795426b37034`。
- 公网工作流详情产物中的飞书凭证字段只有 `app_id` 和 `app_secret`，i18n 产物已移除 `Tenant Key` 标签。
- 远端发布源码的 `feishu.go` 与 `messageChannelSetup.ts` SHA-256 和本地集成源码一致。

本机发布日志：`/tmp/agent-platform-feishu-channel-release.log`。公网产物核对：`/tmp/agent-platform-feishu-channel-public-evidence.json`。临时发布及核对脚本不进入仓库。

## 验证边界与回退

目标 Linux 实际执行 `make production-conformance-preflight`，因缺少专用 Git Fixture、Work/Evidence Root、Sandbox 探测配置、五个 Runtime 的模型测试凭证及专用存储配置而失败；未执行完整 Production/Sandbox Conformance。

本记录证明集成测试、构建、前后端发布、服务健康和公网产物一致性。未使用真实飞书/Lark 凭证执行企业查询或收发，也未进行认证浏览器的桌面/移动端操作验收。没有新增 Runtime Capability 或供应商收发验收结论。

回退使用备份中的源码/Web 指针和已保留的旧 API/Worker 镜像。Web 可通过 `scripts/deploy-web.sh activate qq-gateway-20261004-1` 恢复前一版；没有新增 Migration。
