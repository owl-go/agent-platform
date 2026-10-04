# 资源目录接口 500：查询状态隔离 — 2026-10-03

## 复现与原因

启用消息渠道装配后，`ConfigureChannelProtection` 使用 GORM `Set` 保存加密盒、启用标记和限额，返回了已初始化、会原地修改 Statement 的查询对象。随后 `ListCLIConnectorEnablements` 从共享入口建立 Definition 子查询，将 `Model`、`Select("id")` 和软删除 Schema 留在 Repository 上。其他查询继承了这些状态，错误访问 `users.deleted_at`、`installation.deleted_at`、`c.deleted_at` 或从 `cli_connector_definitions` 查询仅属于其他表的字段。

线上 PostgreSQL 日志与本地复现生成的 SQL 一致。这些字段不存在是正确的现有 schema，故障不应通过向多个无关表追加字段来掩盖。

修复在保存 Settings 后恢复可复用 GORM Session，保留渠道事务需要的加密盒与限额，同时使后续查询各自克隆 Statement。新测试验证渠道装配后先调用 CLI Enablement，再依次查询不同目录；真实 PostgreSQL 集成测试还验证平台资源、个人资源与其他用户私有资源的可见性。

## 实际检查

- 修复前 SQL 回归测试在八个子案例失败，包含与线上一致的错误字段/表引用。
- 修复前真实 PostgreSQL 集成测试返回 `column "owner_user_id" does not exist (SQLSTATE 42703)`。
- 正常 OIDC Authorization Code + PKCE 登录，使用部署配置中与 Bootstrap Administrator 匹配的既有账号，没有打开 Direct Access Grant、创建账号或绕过认证。Token 与凭证只在内存使用，响应正文不保存。
- 修复前登录后的九个 GET 接口均为 500：CLI Enablement、Expert、Expert Team、Skill、MCP、CLI Definition、Connector Catalog、Connector Installation、Command Approval。
- 修复后目标 GORM Repository 的完整测试通过，连接临时 PostgreSQL 并执行完整 Migration 链；全量 `make test` 和 `make build` 通过。合并 `main_temp` 后再次通过全量测试和构建。
- `go test -race` 的查询隔离和资源可见性回归测试通过。
- 技术规格追加了渠道装配不能污染共享资源查询的约束。此前仅验证未装配渠道 Settings 的 Repository，未覆盖实际装配后的交叉接口访问；本次回归测试补齐该场景。

本次没有修改 Migration、Runtime 或前端。真实 IM 收发与新的 Linux Sandbox/Production Conformance 不属于本次故障验收。

## 线上发布与验收

修复提交 `8af29cd` 经 `main_temp` 合并为 `a9bc0e93a4faea27b81bcc47e078a4a6b20b31c0`，在集成分支完成 `make test`、`make build` 后，从该版本构建并发布 `resource-query-isolation-20261003-1`。API、Worker 镜像分别为 `sha256:611cd1307c14e5c15117193a0aa2266810243672c33735a08f6f830ff1a84b0c`、`sha256:c8d1f87cc8d40d5d818e34656e56641890c6ffd4424415e190bca0310c7a09a0`。

发布前完成业务 PostgreSQL 备份、`pg_restore -l` 与备份 SHA-256 校验，保留原配置、源码指针与 API/Worker 镜像，路径为服务器 `/opt/agent-platform/backups/pre-resource-query-isolation-20261003-1`。候选 API 仅绑定服务器回环地址，使用线上数据库及正常 OIDC 登录，九个资源接口全部返回 200，随后删除临时候选容器。

切换前活动 Run 数为 0，先停止 Worker，重建 API 并等待 healthy，再重建 Worker 并等待 healthy，最后切换源码指针。公网 `https://47-237-108-63.sslip.io` 的正常登录探测执行两次，每次均先查询 CLI Enablement，再查询其余八个接口；十八次请求均返回 200。这证明此前会污染共享查询状态的访问顺序已恢复正常。响应正文、Token 与账号凭证没有写入证据。

公网 `/api/healthz`、`/api/readyz` 分别返回 `ok`、`ready`。API、Worker 均 healthy，启动日志 ERROR/FATAL/PANIC 级记录为 0。Worker 指标中 `message-channel-inbox`、`message-channel-delivery`、`message-channel-connections`、`message-channel-maintenance` 四个循环均 started=1、fatal=0。

发布前后 Migration 名称与 checksum 清单一致，配置文件 SHA-256 一致，Web 发布指针一致。资源记录数一致：Expert 8、Expert Team 0、Skill 13、MCP 0、CLI Definition 16、Connector Installation 20、Connector Publication 20。无需添加字段或恢复资源数据。服务器证据保存在 `/opt/agent-platform/evidence/resource-query-isolation-20261003-1`，包含脱敏的接口状态、资源数量、Migration 清单、镜像和渠道循环指标。

本次验收覆盖资源加载、查询隔离与资源可见性，不代表重新验收真实第三方 Connector 操作或 IM 收发。
