---
status: active
last-reviewed: 2026-10-07
---

# 质量门禁与交付证据

本文件将[工程规则基线](engineering-baseline.md)绑定到仓库已有命令和人工检查。当前架构、产品语义与技术栈证据由[采用记录](harness-adoption.md)路由。选择门禁依据实际变更及风险，不要求每次执行所有门禁；命令存在、退出码为零和目标环境验收是三个不同结论。

UI 变化按 [UI 规格与验收](../specs/frontend-ui-acceptance.md)的状态矩阵和 AC-UI-* 选择 G-WEB/G-E2E，具体页面场景和结果保存在任务或日期化证据中。

## 门禁选择

命令默认在仓库根目录执行。先检查改动范围及格式，再跑目标包测试，最后扩展到受影响层级。把本次任务验收编号（AC）和适用规则 ID 记到任务/Issue 中；已有 E2E/Conformance 编号继续使用，不替旧产品条目编造验收结果。

| 门禁 / 触发 | 规则 ID | 实际命令或人工检查 | 成功判据、环境与证据边界 |
|---|---|---|---|
| G-DOC：规范、技术或产品文档 | REQ-001–005、TST-005 | 核对受影响路径、相对链接、领域术语、命令和文档权威位置；`git diff --check`；`cmp -s AGENTS.md CLAUDE.md` | 路由可达、镜像一致、补丁无空白错误；远端 URL 和锚点如未检查须单列。文档检查不证明产品行为。 |
| G-GO：Go 实现或回归 | COD-001–006、TST-001–003/005 | `gofmt -w <changed-go-files>`；`go -C backend test ./internal/<changed-package>/...`；公共契约、安全或跨模块变化再跑 `make test`、`make build` | 目标包和适用完整门禁通过，正常/错误/取消/清理路径有证据；检查输出中的 Skip，不把零退出码写成数据库验证通过。两个 make 目标还执行 `resources-check`。 |
| G-ARCH：分层或跨模块边界 | ARC-001–005 | `go -C backend test ./internal/architecture/...`；人工检查受影响导入、公开端口、数据所有权及主/失败调用链；`make test`、`make build` | 约定的 Domain → Application 依赖和 Adapter 边界成立；架构测试的扫描盲区见采用记录，必须补人工检查。 |
| G-API：Proto、公共 API 或 Runtime Event | ARC-002/003、COD-002/006、TST-001/002/005 | Proto 变化执行 `make verify-generated`、`make breaking`；公共契约与流式端点执行相邻契约/调用方测试，再跑 `make test`、`make build` | 生成物一致、无未说明的兼容破坏；手写流、Runtime fake、Sink、Worker、Adapter 和 Conformance 按受影响范围检查顺序、终态、取消和下游失败。 |
| G-DB：实体、查询、事务或 Migration | DB-001–006、TST-002/003/005 | `go -C backend test ./internal/infrastructure/gormdb/...` 与受影响 Repository 测试；按专项技术文档在隔离 PostgreSQL 上检查空库/代表性旧数据升级、重跑/中断、并发与恢复；`make test`、`make build` | 设置 `WORKSPACE_TEST_POSTGRES_DSN` 并确认相关集成用例真实运行；索引收益、锁时间和数据规模引用实测。恢复需区分应用回退、Schema 回退、数据恢复与前向修复；未演练必须恢复项则保持未完成。初始化不执行迁移。 |
| G-SEC：权限、Secret、Sandbox、路径或外部命令 | COD-002–004、DB-004、ARC-002/003、TST-002/003 | 相邻 allowlist、绕过、越权、脱敏与清理测试；人工审查受影响参数数组、日志、Snapshot、Diff、Artifact 和凭证生命周期；`make test`、`make build`；涉及隔离再执行 G-RUNTIME | 正反路径和取消/失败路径满足既有 fail-closed 红线；禁止例外的泄密/越权缺陷阻断。人工检查不能替代 Linux 隔离证据。 |
| G-WEB：页面、组件或交互 | UI-001–005、COD-001/006、TST-001/002/005 | 首次安装依赖用 `pnpm --dir frontend install --frozen-lockfile`；相关测试用 `pnpm --dir frontend test -- <test-path>`；`make web-typecheck`、`make web-build`；人工核对相关状态、双语、键盘/焦点、目标视口与视觉证据 | 与当前产品契约和 Token/组件一致，所有适用关键状态可操作；记录实际尺寸和状态，截图不替代操作。响应式/无障碍目标见现有 UI 标准，类型/构建通过不等同浏览器验收。 |
| G-E2E：跨层产品流程 | REQ-004、TST-001–003/005、UI-002–005 | 按 [E2E 文档](../testing/e2e.md)执行 `pnpm --dir frontend run typecheck:e2e`、`pnpm --dir frontend run test:e2e`；完整运行后 `node scripts/e2e/report.mjs` | Docker、Go、Node、pnpm、Chromium 与隔离服务可用；原始报告位于忽略目录。本地 E2E 不启动真实 Worker/Runtime，环境门禁 Skip 必须单列。 |
| G-RUNTIME：镜像、Capability、Container、Egress 或存储 | ARC-002/003、COD-003/004、TST-001–003/005 | Docker 可用时镜像改动跑 `make runtime-image-smoke`；MinIO 用 `make minio-conformance`；Linux + runsc 环境先 `make production-conformance-preflight`，再按范围 `make sandbox-conformance`、`make production-conformance` | 各命令前提、Digest 与 Fixture 依 [Conformance 规格](../technical/production-conformance.md)。Aliyun OSS/真实供应商按专项规格使用受保护环境；缺环境、Skip 或 macOS 检查不能替代目标环境通过。 |
| G-DEPLOY：安装或发布脚本/配置 | ARC-003、COD-003/004、TST-001–003/005 | `make deploy-test`；按 [CI](../../.github/workflows/ci.yml)核对 Compose 配置、镜像构建及受影响 shell 语法 | 本地脚本测试不授权生产操作。集成与发布继续经过 `main_temp`，发布前检查和恢复依 [部署手册](../../deploy/platform/README.md)及任务授权。 |

## CI 与命令限制

根 [Makefile](../../Makefile)、[backend/Makefile](../../backend/Makefile)、[frontend/package.json](../../frontend/package.json)和 [CI](../../.github/workflows/ci.yml)是命令来源；它们变化时同步本表。CI 还执行 `go -C backend vet ./...`，以及：

```bash
go -C backend test -race ./internal/service/workspace ./internal/server/... ./internal/architecture ./internal/biz/workspace/... ./internal/data/workspace/...
```

CI 目前在 Pull Request 和 `main` push 触发；功能分支单独 push 不证明 CI 已运行。CI 的后端 Job 直接执行 `go test` / `go build`，不调用根 make 的资源前置检查；资源目录变化还需 `make resources-check`。前端 CI 配置类型/构建，未配置 Vitest/E2E Job，按任务选定的相关测试仍需执行。

`make verify-generated` 会调用 generate 并重写生成物，再检查差异；需 Go 工具下载与前端依赖，不是只读命令。`make breaking` 默认对照 `origin/main`，实际基线与回退到 lint 的情形须记入结果。数据库测试应使用隔离测试 DSN；生产数据不作为默认测试输入。覆盖率没有产物时报告未验证，不估算百分比。

## 任务、失败恢复与交接

短任务在 Issue 或完成报告保存记录；长期或跨阶段任务在已有 `docs/tickets/` 或任务账本持续更新，不给每次小改动新增永久规范。最少保存以下内容：

1. 目标、改动范围、风险，以及任务验收 ID 的前置、触发和可观察结果。
2. `AC → 适用规则 ID → G-* → 实现/测试位置 → 通过/失败/未验证`；人工验收提供步骤、状态、视口和结果，关联可定位且无秘密的日志/截图/报告。
3. 已完成、进行中、受阻和剩余范围，未决问题及对依赖工作的影响；不适用检查说明依据。
4. 命令、工作目录、基线/版本、目标环境、退出结果及 Skip；失败保留首次结果与原因，修复后只重跑受影响检查。重试未产生新证据时停止并记录阻塞，不用重试隐藏失败。
5. 工作区/分支/Commit 状态、已验证恢复点、下一步及安全恢复方式；代码、Schema、数据回退分别说明，不能声称未演练的恢复已经可用。

适用 MUST 门禁失败或未验证时，该交付保持待完成；仅已批准且有效的规则例外可以改变其处置，禁止例外的红线不能豁免。任务应区分文档初始化完成、代码测试完成、目标环境验收完成，历史证据保留其原始日期和范围。
