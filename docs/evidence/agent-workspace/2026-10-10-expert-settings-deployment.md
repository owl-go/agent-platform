# 专家目录与专家团设置发布验证 — 2026-10-10

User 明确授权部署。功能分支 `codex/expert-team-refactor` 的 `05bcd86`、`cb518da` 经 `main_temp` 集成，固定发布源码为 `8d67c8d33ceb47edad96078755083eda4ab38038`，发布编号 `app-20261010T084447Z-73c63e15`；北京时间 2026-10-10 下午完成。

本次发布包括专家添加菜单、精简详情弹窗、技能和连接器下拉编辑，以及管理员专家团新增/编辑的名称、描述、成员、领队四项设置。行为与本地浏览器证据见[专家目录验收](2026-10-10-expert-catalog-ui.md)、[专家团设置验收](2026-10-10-expert-team-settings.md)。没有更新 Runtime、Sandbox、身份主题或基础设施配置。

## 实际检查

- `make deploy-check`：发布配置与后端、前端更新范围检查通过。
- `make deploy-test`：23 项部署测试、13 项安装测试及 shell 语法检查通过。
- `make deploy`：从 `origin/main_temp` 提取不可变源码，自动执行 `make test`、`make build`、冻结锁文件安装、完整前端测试（58 个文件、661 项）、`make web-typecheck` 和使用部署公开 OIDC 配置的 `make web-build`，全部通过。未配置专用 PostgreSQL 测试 DSN，相关 Skip 不记为数据库集成验收。
- 发布前未发现运行中或待处理任务。脚本校验源码与 Web 传输内容，在发布锁下备份业务库、身份库和受限配置；两个数据库 dump 经 `pg_restore -l` 验证可读。发布后再次核对备份 SHA-256 与文件权限通过；没有下载受限备份。
- 发布脚本及发布后只读复核通过 HTTPS 首页、Health、Readiness、OIDC Discovery、API / Worker 健康，以及公网 HTML 和入口资源逐字节一致检查。

## 运行版本

运行源码标记、后端内容摘要及容器镜像与固定发布一致。API / Worker 均为 healthy，RestartCount 为 0；启动时间范围内 ERROR / FATAL / PANIC 级别日志匹配计数均为 0，未保存日志正文。

| 服务 | 实际镜像 ID |
|---|---|
| API | `sha256:8d51ff09fb5821de6ce8b39ede54c759de1b2b533a88d308bdbfe9992ad8345e` |
| Worker | `sha256:349e00da796b912c5b2e4f49493fbab7577b4f37ae20edfb82927deb250a9b99` |

Migration 账本前后均为 102 项，完整账本与发布前恢复元数据一致，最新仍为 `000087_expert_team_creation_actions.sql`；本轮没有新增迁移。备份保留在服务器 `backups/pre-app-20261010T084447Z-73c63e15`，全部备份文件及目录无 group / other 访问位。

专家和专家团查询在未认证时均返回 401。以下公网资源与服务器当前 Web 发布逐字节一致；专家团设置资源包含当前描述、成员、领队字段。

| 资源 | SHA-256 |
|---|---|
| `ExpertTeamSettings-b9fGjx27.js` | `148e1e9424405a02ce675dfe37380277df019bb5f13db101b1fbdcb5133b0be9` |
| `ExpertTeamEditorPage-B1KV6Dw7.js` | `a96a8471a32c519b6fd42547f7a5ca704b2a1ad9bd809ae52e5202f5186f786b` |
| `ExpertEditorPage-BQTGtc7D.js` | `44fe1c260f7e76e444a448fc79f8bed84b21b12ce0c3c706a73cab9b3e308843` |

## 验证边界与恢复

本次证明集成应用上线、服务健康、线上静态资源一致、迁移账本未变及受限备份完整。线上浏览器打开和状态读取均因浏览器控制工具超时未完成；未认证登录或使用生产身份新增/修改专家、专家团，不能将本地受控预览称为线上表单验收。未调用真实模型执行专家团，未执行 Linux Sandbox 或 Production Conformance。

本次无 schema 变化，部署入口支持切换检查失败时恢复原应用；本次没有触发回滚，也未执行生产数据库恢复演练。恢复元数据、原镜像和数据库备份均保留在服务器受限备份目录。
