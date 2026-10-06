# 继承企业默认组合的首条 Session 消息：2026-10-06

问题：新 User 的 Personal Settings 已继承 Platform Execution Default，但空 Session 的发送按钮被禁用。管理员保存企业默认组合已不再要求验证 Run，SessionsPage 却仍对继承配置要求供应商连接和 Runtime/model pair 均为 `verified`。

线上只读检查确认：默认 Codex / Provider Model 已物化到继承账号；Runtime 和模型可用，API Key 及对应版本存在，供应商连接和 Codex pair 均为 `unverified`。新账号具有每日 Credits，未发现默认模型为空。没有读取或输出凭据及消息正文。

修复：删除仅针对继承配置的 `verified` 门槛，首条消息沿用个人选择的可用性检查。Runtime 不可用、未选择模型、缺少 API Key、模型不可用、pair 为 incompatible 或缺少 pair 信息时仍拦截发送。没有修改继承状态、默认组合或验证状态。

已执行的验证：

- 修复前运行 `pnpm --dir frontend exec vitest run src/pages/SessionsPage.test.ts -t 'sends the first inherited message'`：四种连接/pair 验证状态中，三种包含 `unverified` 的组合失败，双 `verified` 通过。失败点为预期不显示配置引导但实际显示。
- 修复后同页继承回归场景 10 个通过：四种验证状态可输入并点击发送，六种不可用配置禁用发送且不调用 API。完整 SessionsPage 测试 56 个通过。
- `pnpm --dir frontend test`：55 个文件、622 个测试通过。
- `make web-typecheck`、`make web-build`：通过；构建保留现有 chunk size 提示。
- `make test`、`make build`：通过。全量测试未配置 `WORKSPACE_TEST_POSTGRES_DSN`，其中跳过的数据库测试不记为通过。
- 使用一次性 `postgres:17-alpine` 容器和 tmpfs，设置测试进程的 `WORKSPACE_TEST_POSTGRES_DSN`，执行 `go -C backend test -count=1 ./internal/data/workspace/gormrepo -run '^TestPlatformExecutionDefaultSavesWithoutRunAndPropagates$' -v`：通过。分别覆盖 verified/unverified 连接；新 User 无需保存个人设置即可创建 Session、提交首条消息并进入 queued，Response Snapshot 保留继承的 Codex、模型和 unverified pair。测试使用隔离数据库并执行真实 Migration；容器和临时数据库已清理。
- `git diff --check`、`cmp -s AGENTS.md CLAUDE.md`：通过。

本次验证证明界面允许提交且 Repository 能持久化首条消息及执行快照；没有向真实模型供应商发送验收消息，也不新增 Runtime 镜像或 Linux + runsc 的 Production Conformance 证据。
