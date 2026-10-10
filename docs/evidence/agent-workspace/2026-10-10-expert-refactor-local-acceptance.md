# 专家与专家团本地验收 — 2026-10-10

范围为已批准的 22 张执行票和父规格的 68 条用户故事，审查基准为 `5269ea8`，连续开发分支为 `codex/expert-team-refactor`。行为合同见[重构规格](../../product/expert-package-team-refactor.md)、[技术合同](../../technical/portable-experts.md)和[ADR-0046](../../adr/0046-portable-experts-and-lead-coordinated-teams.md)。父 Issue #71 不作修改或关闭。

## 验收入口

认证 API 验证创建、编辑、预览/修订/原子确认、导入/导出、可见性及安全投影；已有 Executor.Execute 与可替换 Adapter 验证实际委派、Workspace、限制、审批和计费；PostgreSQL 17 的每个测试使用独立可删除数据库；Vue 组件和真实 Chromium/OIDC/API 验证界面。没有新增测试专用生产 API。

所有迁移为新文件 `000075`–`000087`；原迁移没有修改。旧 schema fixture 注入团队删除异常，证明失败迁移回滚删除与引用更新，解除异常后可继续并重复执行。恢复已成功删除的私人定义需要删除前数据库备份；应用回退不会自动重建数据。恢复边界见技术合同。

## 22 张票的实现核对

| 票 | 用户故事 | 本地证据 |
|---|---|---|
| T01 / [#72](https://github.com/owl-go/agent-platform/issues/72) | 1, 2, 3, 4, 14, 28, 34 | 认证 Markdown API、不可变修订 PostgreSQL、Executor 指令注入及编辑器 |
| T02 / [#73](https://github.com/owl-go/agent-platform/issues/73) | 11, 33, 68 | 退休标签 Worker 回归、目录组件与旧快照 |
| T03 / [#74](https://github.com/owl-go/agent-platform/issues/74) | 19, 20, 21, 22 | 认证权限 API、Team 所有权 PostgreSQL及浏览器 |
| T04 / [#75](https://github.com/owl-go/agent-platform/issues/75) | 24, 25, 26, 27 | 旧 schema 前向转换、注入删除失败后重跑、历史 Reader/Retry |
| T05 / [#76](https://github.com/owl-go/agent-platform/issues/76) | 23, 29, 30, 31, 32, 33 | 独立成员和修订 PostgreSQL、管理员编辑器 |
| T06 / [#77](https://github.com/owl-go/agent-platform/issues/77) | 6, 7, 12, 13, 14, 15, 18 | 认证包 API、恶意 ZIP/清单验证、导出往返 |
| T07 / [#78](https://github.com/owl-go/agent-platform/issues/78) | 6, 7, 12, 13, 15, 19, 21 | 管理员 Team 包往返、普通 User 拒绝导入/复制 |
| T08 / [#79](https://github.com/owl-go/agent-platform/issues/79) | 61, 62, 63, 64, 65, 66 | 目录冻结/路径测试、重复及并发默认初始化 PostgreSQL |
| T09 / [#80](https://github.com/owl-go/agent-platform/issues/80) | 61, 62, 63, 64, 65, 66, 23 | 同目录 Team 初始化/升级事务 PostgreSQL |
| T10 / [#81](https://github.com/owl-go/agent-platform/issues/81) | 8, 9, 10, 67 | 头像实际解码、认证头像 CAS/三提示、包往返、浏览器 |
| T11 / [#82](https://github.com/owl-go/agent-platform/issues/82) | 5, 10, 21 | 认证原子确认与预览 CAS、管理员 Team、预览组件 |
| T12 / [#83](https://github.com/owl-go/agent-platform/issues/83) | 34, 35, 36, 37, 38, 39, 44, 45, 48, 50, 59, 60 | Executor 领队/成员委派、正式回答分离、快照选择 PostgreSQL |
| T13 / [#84](https://github.com/owl-go/agent-platform/issues/84) | 41, 43, 45, 48, 57 | 冻结 Session→Workflow 模板 PostgreSQL、Executor 成功后提交/失败清理 |
| T14 / [#85](https://github.com/owl-go/agent-platform/issues/85) | 16, 17, 18, 37, 38 | 包内 Skill 不进入目录、实际临时技能权限与清理、执行 User 连接器授权 PostgreSQL |
| T15 / [#86](https://github.com/owl-go/agent-platform/issues/86) | 40, 41, 42, 43, 54 | Executor 三并行、独立 Workspace、文件冲突领队解决 |
| T16 / [#87](https://github.com/owl-go/agent-platform/issues/87) | 44, 49, 50 | Executor 一次 Repair、Required 失败、20 调用上限及控制合同 |
| T17 / [#88](https://github.com/owl-go/agent-platform/issues/88) | 46, 47, 48 | 响应预算/余额/部门实际 PostgreSQL、并发准入和幂等结算、Executor 超额阻止后续调用 |
| T18 / [#89](https://github.com/owl-go/agent-platform/issues/89) | 51, 52, 53 | 可控主动时钟、并行审批聚合状态 PostgreSQL、既有绑定/拒绝/过期审批测试 |
| T19 / [#90](https://github.com/owl-go/agent-platform/issues/90) | 48, 54, 55, 56, 57 | 新策略中断失败/不重放、旧恢复兼容 PostgreSQL、取消 queued/active 与清理 |
| T20 / [#91](https://github.com/owl-go/agent-platform/issues/91) | 58, 59, 60, 67 | 认证 HTTP/SSE 安全投影、Task Panel 组件与持久化浏览器 |
| T21 / [#92](https://github.com/owl-go/agent-platform/issues/92) | 3, 11, 28, 68 | 生成/兼容检查、旧写入拒绝/终态 Reader、目录中性清单、权威文档 |
| T22 / [#93](https://github.com/owl-go/agent-platform/issues/93) | 67, 68 | 完整门禁、跨入口验证、桌面/移动截图、逐故事核对及生产证据边界 |

## 逐用户故事核对

以下关联用于定位实现与验收，不表示 scripted Adapter 已验证模型质量。测试断言聚焦可观察边界；同一事务/执行 seam 的一项测试可覆盖多条相关故事。

| 用户故事 | 对应执行票 |
|---|---|
| US-01 | T01 |
| US-02 | T01 |
| US-03 | T01, T21 |
| US-04 | T01 |
| US-05 | T11 |
| US-06 | T06, T07 |
| US-07 | T06, T07 |
| US-08 | T10 |
| US-09 | T10 |
| US-10 | T10, T11 |
| US-11 | T02, T21 |
| US-12 | T06, T07 |
| US-13 | T06, T07 |
| US-14 | T01, T06 |
| US-15 | T06, T07 |
| US-16 | T14 |
| US-17 | T14 |
| US-18 | T06, T14 |
| US-19 | T03, T07 |
| US-20 | T03 |
| US-21 | T03, T07, T11 |
| US-22 | T03 |
| US-23 | T05, T09 |
| US-24 | T04 |
| US-25 | T04 |
| US-26 | T04 |
| US-27 | T04 |
| US-28 | T01, T21 |
| US-29 | T05 |
| US-30 | T05 |
| US-31 | T05 |
| US-32 | T05 |
| US-33 | T02, T05 |
| US-34 | T01, T12 |
| US-35 | T12 |
| US-36 | T12 |
| US-37 | T12, T14 |
| US-38 | T12, T14 |
| US-39 | T12 |
| US-40 | T15 |
| US-41 | T13, T15 |
| US-42 | T15 |
| US-43 | T13, T15 |
| US-44 | T12, T16 |
| US-45 | T12, T13 |
| US-46 | T17 |
| US-47 | T17 |
| US-48 | T12, T13, T17, T19 |
| US-49 | T16 |
| US-50 | T12, T16 |
| US-51 | T18 |
| US-52 | T18 |
| US-53 | T18 |
| US-54 | T15, T19 |
| US-55 | T19 |
| US-56 | T19 |
| US-57 | T13, T19 |
| US-58 | T20 |
| US-59 | T12, T20 |
| US-60 | T12, T20 |
| US-61 | T08, T09 |
| US-62 | T08, T09 |
| US-63 | T08, T09 |
| US-64 | T08, T09 |
| US-65 | T08, T09 |
| US-66 | T08, T09 |
| US-67 | T10, T20, T22 |
| US-68 | T02, T21, T22 |

## 十组验收与关键测试

| 验收 | 关键已执行证据 |
|---|---|
| AC-EX-01 | `expertpackage` 安全/内容/图像测试；`expert_package_integration_test.go` 的认证往返、幂等、冲突、升级和权限；编辑后 Bundle 导出往返 |
| AC-EX-02 | `directory_test.go` 的冻结/链接/清单验证；`expert_directory_integration_test.go` 和既有 default initialization 的全量并发/不可变/升级/自定义保护 |
| AC-EX-03 | `expert_team_permissions_integration_test.go`；普通用户直接/包/确认入口拒绝；管理员会话式 Team 确认；用户自己的授权解析及浏览器 |
| AC-EX-04 | `expert_migration_integration_test.go` 注入异常回滚、前向恢复、重复迁移、正文保留及绑定清理；历史执行/恢复 fixtures |
| AC-EX-05 | 独立 owned members、不可变 Expert/Team 修订与源编辑/删除隔离；Session/Workflow 原始模板和保留选择 |
| AC-EX-06 | `coordination_test.go` 的冻结 roster、必要成员、重复成员隔离、3 并行、20 上限、未知控制拒绝及独立调用 UUID |
| AC-EX-07 | 相邻 Executor tests 的文件冲突、所选来源、取消和成功后提交；`team_recovery_integration_test.go` 的不重放与已有事实/消费保留 |
| AC-EX-08 | `team_credit_integration_test.go`、Credits 生命周期/部门原子性；`team_clock_test.go`、`team_approval_integration_test.go`及现有 CLI 命令绑定/拒绝/过期测试 |
| AC-EX-09 | 认证 Markdown/Profile/Preview 测试；编辑器/目录/预览组件；真实桌面/移动浏览器创建、导入、键盘打开和管理员边界 |
| AC-EX-10 | `team_task_panel_integration_test.go` 的查询及 SSE；Task Panel 展开、修复/必需/调用数/冲突组件；真实数据库 fixture 浏览器重载与键盘结果展开 |

## 已执行门禁

- `WORKSPACE_TEST_POSTGRES_DSN=<local disposable DSN> make test`：完整 Go 测试通过，包含架构契约及真实 PostgreSQL 集成测试。
- 受影响 Executor、Workspace Repository/Service、Credits Repository、Domain、resourceaction 和 expertpackage 的完整 `go test -race`：七个包通过。审查修复后的三个包定向 race 回归通过；新目录 Discovery/Rejects/Discovers 合同和单独的 `TestDirectory` race 检查通过。额外执行的 defaultresources 全量 race 检查在重复解压既有 Connector CLI bundle 时达到 10 分钟测试超时，不能记为通过；该包常规完整 Go 测试已通过。
- `make build`、`go -C backend vet ./...`、`make generate`、`make breaking`、`make resources-check`：通过。目录结果为 20 Connector Packages、9 Skills、8 Experts、0 随仓分发 Team；纯目录 fixture 已证明任意合法 Team 可自动发现。
- `pnpm -C frontend test`：完整 56 文件/635 测试通过。审查修复后，`pnpm -C frontend exec vitest run --maxWorkers=2` 的完整 56 文件/637 测试通过，包含两个新的预览异步竞态回归。并行执行构建时的一轮完整测试遇到两个既有界面测试超时，同时包含尚未修复的确认竞态测试；修复后降低测试并发重跑通过，未延长断言超时或改动无关界面。
- `make web-typecheck`、`make web-build`、`pnpm -C frontend typecheck:e2e`：通过。
- `git diff --check`、`cmp -s AGENTS.md CLAUDE.md`、新增/修改文档和清单的中性用词及相对链接检查：通过。
- `make verify-generated`：通过，重新生成与已提交文件无差异；首次因远端生成服务不可用失败，重试成功。

浏览器执行命令为 `E2E_MINIO_BINARY=<task-owned source-built binary> node scripts/e2e/run-local.mjs --grep 'E2E-009|E2E-010|E2E-011|E2E-012|E2E-029|E2E-030|E2E-044|E2E-EXPERT|E2E-TASK'`。使用隔离的 PostgreSQL、真实 Keycloak、当前 API/Web 和 MinIO。上游容器镜像拉取失败后改用官方固定发布源码构建的临时 MinIO，不修改生产存储配置。九个不同流程已通过：基础七项及最终专家 Profile/包复制、Task Panel 两项。Task Panel 使用真实数据库写入的终态 fixture，未运行实际 Worker/模型；真实运行行为由 Executor 与 PostgreSQL 测试另行证明。截图已人工查看，移动端无横向溢出。

视觉截图保存于本地 `output/playwright/experts-desktop.png`、`experts-mobile.png`、`team-task-desktop.png`、`team-task-mobile.png`，不把凭据、私有服务日志或整个测试输出目录提交到仓库。此验收未进行屏幕阅读器测试。

## 环境证据边界

没有部署或生产数据库迁移；没有新的固定 Runtime RepoDigest 模型协作质量验收或 Linux + runsc sandbox/production conformance。macOS fake Adapter 只证明平台执行合同。远端 Aliyun OSS 与独立 MinIO conformance 所需环境未提供，完整 Go 的相关环境依赖测试存在 Skip；浏览器中的真实 MinIO 不等于完整存储 conformance。旧已发表生产证据也不扩张为本次新策略的证据。

## 审查及提交

固定比较命令为 `git diff 5269ea8...HEAD`；初审实现提交为 `7e21949`。工程规范与规格由两个只读审查分别执行，修复后再次复核。

### Standards

累计三项发现，最高 P1，均已解决：

- 预览保存等待期间继续编辑被错误标记为已保存。请求前冻结提交内容，后续编辑保持未保存；延迟响应回归先失败后通过。
- 确认请求返回前切换预览，旧响应可能触发新预览的确认。保存与确认均绑定原动作 ID 和对象身份，忽略陈旧响应；延迟确认后切换的回归先失败后通过。
- Connector 依赖将数据库故障和取消误报为配置错误。只将明确缺失、版本或授权错误转为不可用提示；保留系统 cause 并传播。真实 PostgreSQL 故障注入、取消、MCP/CLI 授权查询和 Service 回归通过。

依据为工程基线 UI-002 与 COD-002；复核没有未解决问题。

### Spec

累计两项发现，最高 P2，均已解决：

- 完整目录替换为文件时遗留空目录阻止合并。仅移除已验证的空目录树；文件与目录互换、竞争后代路径保留测试通过，满足非冲突合并合同。
- 领队上下文缺少成员文件增改删摘要。平台从独立 Workspace 基线生成有界、完整凭据脱敏的文件事实；实际 Adapter 下一次领队请求验证三种变化及秘密路径脱敏，截断和 UTF-8 路径边界测试通过，满足 US-39。

复核没有未解决问题或已证实的范围扩张。Standards：3 项已解决、0 项未解决；Spec：2 项已解决、0 项未解决。

实现与修复提交并推送连续开发分支；继续遵守 `main_temp` 集成与发布边界，不自动部署或改动保护分支。
