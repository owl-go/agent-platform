# 专家与专家团重构发布验证 — 2026-10-10

公开证据不保存部署账号、实际主机路径或私有配置。以下版本、摘要和计数来自实际发布及只读核对；本次部署由 User 明确授权。

## 发布版本

功能分支 `codex/expert-team-refactor` 的实现及审查修复已快进合入并推送 `main_temp`。发布固定源码为 `f797cbda351678d9fcc510f53ef1b3baa6e3c051`，发布编号为 `app-20261010T023906Z-67257daa`，北京时间 2026-10-10 上午完成。

执行入口为 `make deploy-check`、`make deploy-test` 和 `make deploy`。一键部署从 `origin/main_temp` 提取不可变源码，校验传输内容，复用服务器受限配置，在发布锁下备份并校验业务库与身份库，然后构建、切换 API / Worker / Web。没有从功能分支直接发布，没有改动 Runtime、Sandbox、身份主题或基础设施配置。

## 实际门禁

- `make deploy-test`：23 项部署测试、13 项安装测试及 shell 语法检查通过。
- 固定集成快照的自动门禁：`make test`、`make build`、冻结锁文件安装、完整前端测试、`make web-typecheck`、使用部署公开 OIDC 配置的 `make web-build` 全部通过。
- 发布前无运行或待处理任务。适用 PostgreSQL、权限、协作与界面回归以及 637 项前端测试的具体本地证据见[本地验收](2026-10-10-expert-refactor-local-acceptance.md)；部署脚本门禁未配置专用 PostgreSQL 测试 DSN，相关 Skip 不扩张为真实数据库集成验证。
- 自动发布核对 HTTPS 首页、Health、Readiness、OIDC Discovery、API / Worker 健康，以及公网 HTML 和入口资源逐字节一致；发布后再次执行核对通过。

## 服务与迁移核对

运行源码标记、容器镜像与固定发布版本一致。API / Worker 均 healthy，RestartCount 为 0，启动日志 ERROR / FATAL / PANIC 级别匹配计数为 0。

| 服务 | 实际镜像 ID |
|---|---|
| API | `sha256:7a353a80ecd353b7c6f41f1a26c024a455ce3b0b235086d7a00cbc89274fa035` |
| Worker | `sha256:81d59bf51739969964173bc4b503888a653039b70ce79f84c6d98fc63c4362f2` |

Migration 账本从 89 项增至 102 项，新增 `000075`–`000087` 全部执行，最新为 `000087_expert_team_creation_actions.sql`。部署脚本核对每个 SQL 的 SHA-256 和历史账本，未发现摘要变化。

发布前后普通 User 专家团定义和 Administrator 专家团定义均为 0，退休团队身份计数为 0；本次没有实际删除私人团队行。历史 Session Message 为 162 条、Run 为 92 条、Credit Ledger 为 444 条，前后计数一致。计数仅用于发布核对，没有读取私人正文或凭据。初次只读计数查询错误使用了不存在的列，纠正后核对通过，没有写入数据库。

目录初始化账本包含 8 个 Expert 项。它们已被原有初始化账本标记为自定义，依照自定义保护合同保留，未强制覆盖；Administrator 所有的 8 个专家均已有非空 Markdown Guidance。镜像构建同时验证了中性目录中的 8 个 Expert 包，随仓没有 Team 包。

## 备份与公网资源

服务器保留 `backups/pre-app-20261010T023906Z-67257daa` 的业务库及身份库 dump、受限配置、恢复元数据和摘要。两个 dump 已通过 `pg_restore -l` 可读性检查；发布后再次计算全部备份摘要一致，受保护文件均没有 group / other 访问位。未将备份下载或提交，不把可读性检查称为生产数据库恢复演练。

发布后的只读 HTTPS 检查确认：专家及专家团查询在无身份时返回 401；下面两个公网专家编辑器资源与当前服务器发布逐字节一致。

| 资源 | SHA-256 |
|---|---|
| `ExpertEditorPage-CUHmm94d.js` | `9dbd7183dd808a29bc10ac6b04a0606d751a6f44315082487c83926cfadfc158` |
| `ExpertTeamEditorPage-CcFITV9Q.js` | `1e16e5b9da844668daea7502c9df56f050a0fa22146c094146a56c4ada7ea22c` |

## 证据边界

本次证明应用及前端上线、迁移账本正确、受限备份可读且摘要一致、历史记录计数保留和服务健康。未使用生产 User / Administrator 身份创建、导入或修改资源，未调用真实模型执行专家团，没有新增 Runtime RepoDigest、Linux + runsc 隔离或 Production Conformance 声明。没有执行生产数据库备份恢复；新迁移后的故障恢复必须使用兼容 schema 的版本或明确恢复备份，不能直接回退旧二进制。
