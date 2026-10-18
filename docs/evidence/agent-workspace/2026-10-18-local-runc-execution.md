# 本地 runc 执行验收 — 2026-10-18

## 范围与环境

用户明确授权个人本地 `runc` 兼容模式，生产继续 `runsc`。实现边界由 [ADR-0046](../../adr/0046-explicit-local-runc-profile.md)规定。复用 `codex/local-deployment`，基于已有本地服务提交 `8003bc7`；没有连接、修改或发布云服务器，没有向 Docker Hub 推送镜像。

实际环境为 macOS Apple Silicon、Docker Desktop Linux/aarch64，Docker 只有可用的 `runc`，没有 `runsc`。API、Web、MinIO 为本机进程；Worker、PostgreSQL、Keycloak 为本地容器。页面 `http://127.0.0.1:14173`；API、Worker、数据库、认证、存储端口只发布在环回地址。本地状态和数据库沿用已有数据，没有重置供应商或会话。

本次实际运行的本地不可变 Image ID（不是生产 Registry RepoDigest）：

- Runtime：`sha256:4e517990bbc46b6568e38726be9f2f8c17510f4bfe8bedca2928f83902c2cb4d`。
- Worker：`sha256:3e1b19753d7bc7a1646c58bae603e692db4d46ced4753c5c763deb61376725d7`。

固定版本检查及镜像 smoke 通过：Claude Code `2.1.233`、Codex `0.147.0`、Hermes `0.19.0`、OpenClaw `2026.7.1-2`、PI Agent `0.84.4`。五个引擎在 Runtime API 中 `available=true`；本地均未开启 Native Resume。只有下面记录的 Codex/供应商组合做了真实模型调用，不能据此宣称其他组合已验证。

## 验收与门禁映射

| AC | 规则与门禁 | 实现/测试 | 实际结果 |
|---|---|---|---|
| LOCAL-01 显式本地模式，生产不自动降级 | ARC-003、COD-003/004；G-GO/G-SEC/G-RUNTIME | `sandbox/profile*`、`platformconfig/local_test.go`、`containerprocess/local_test.go` | 默认/production 拒绝 runc 和裸 Image ID；local 拒绝 Tag、root、host 网络及 CLI Connector 挂载。冷/暖启动保留只读 Rootfs、cap-drop、资源限制。测试通过。 |
| LOCAL-02 引擎可用、企业默认可保存 | REQ-004、UI-002；G-WEB/G-DEPLOY | `local-platform.mjs`、`SettingsPage.test.ts`；真实管理员浏览器 | 设置页选择 Codex + GPT-6.1 Sol 后保存，GET 企业默认 HTTP 200 / version 1；当前账号继承配置已同步。重启后仍保留。无需 validation Run，组合仍为 unverified。 |
| LOCAL-03 真实多轮会话及工具执行 | COD-002/004、TST-002；G-RUNTIME | `runtimeexecutor`、真实供应商会话 | 会话 `c0d9db35-bb97-4da8-9feb-ec4b763c4022`：消息 2 返回“本地会话运行成功。”；重启后的消息 8 正确回复口令“青竹七号”及 UID `65532`，状态 completed；记录 command.requested / command.completed。 |
| LOCAL-04 清理和取消 | COD-003/004、TST-002/003；G-SEC/G-RUNTIME | `local_filesystem*_test.go`、`local_docker_test.go`、真实取消调用 | 12 次真实只读嵌套挂载启动/执行/停止/删除全部通过；取消会话 `dd3a2136-a63f-44e2-966a-7af496a02b89` 的消息 10 时已进入 `sleep 30` 工具调用，最终 cancelled，执行容器停止，凭证目录没有残留文件。 |

浏览器在 1920×992 验证了真实设置保存、发送首条消息和流式回复。后续跨层检查使用同一登录页面的受认证 API；没有将 Token、Key、配置文件或凭证加入仓库。浏览器原始快照/截图在忽略目录 `.playwright-cli/`，不作为提交产物。未执行完整浏览器 E2E、移动端或完整无障碍验收。

## 失败、定位与恢复

第一次尝试通用 Worker 镜像时，可选 Notion CLI 下载出现 EOF；本地 Worker 不需要该 API 授权工具，新增独立本地 Worker Dockerfile，生产 Dockerfile 未修改。另一次构建遇到上游 Alpine metadata TLS timeout，重试成功。

首次真实工具会话（消息 4）已得到正确模型结果，但清理附件挂载点时返回 permission denied，最终被正确标记为 failed。无模型 Docker 最小复现中，停止容器后立即 `rmdir` 同样失败，稍后删除同一路径成功，目录仍为 UID/GID 65532、权限 0750。针对该行为先验证立即删除策略的回归测试失败，再仅在 local profile 加入最多 1 秒的特定系统错误重试；取消和永久错误仍返回失败，生产逻辑保持立即删除。修复后真实消息 8 与 12 轮 Docker 回归均通过。失败历史保留，没有修改历史结果或隐藏失败。

临时复现容器、临时目录及 Linux 测试二进制在验证后清理，持久数据保留。

## 实际执行的检查

以下命令均通过（工作目录为本会话 worktree）：

```bash
go -C backend test ./internal/sandbox/... ./internal/platformconfig/... ./internal/agentruntime/containerprocess/... ./internal/data/workspace/runtimeexecutor/... ./internal/wiring/workspaceworker/...
go -C backend test -race ./internal/sandbox/... ./internal/platformconfig/... ./internal/agentruntime/containerprocess/... ./internal/data/workspace/runtimeexecutor/... ./internal/wiring/workspaceworker/...
make test
make build
go -C backend vet ./...
pnpm --dir frontend test -- src/pages/SettingsPage.test.ts
pnpm --dir frontend exec vitest run src/pages/SettingsPage.test.ts src/i18n/index.test.ts
make web-typecheck
make web-build
make deploy-test
RUNTIME_IMAGE_REF=agent-workspace-runtime:local make runtime-image-smoke
make local-up
make local-status
node --check scripts/local-platform.mjs
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

第一次前端命令实际执行了完整 Vitest：55 文件、627 项通过；最后针对设置/i18n 的复测为 2 文件、15 项通过。typecheck 曾因新增测试的字符串字面量推断失败，收窄类型后重跑成功。生产构建仅有既有的大 Chunk 提示。部署脚本检查为 23 + 13 + 3 项测试通过。

真实 Docker 挂载测试不是把普通本机 Skip 算作通过：先以 `GOOS=linux GOARCH=arm64 go -C backend test -c` 生成该包测试二进制，放在本地共享 workspace，再在本地 Worker 内设置 `WORKSPACE_TEST_LOCAL_DOCKER_ROOT` 并运行 `-test.run '^TestLocalDockerReadOnlyMountCleanup$' -test.v -test.timeout 2m`，实际 12 轮在 4.11 秒内通过。普通 `make test` 未配置该变量时明确 Skip 此环境用例。

## 尚未验证的范围

- 没有 Linux + runsc 目标环境，本次没有运行 Sandbox / Production Conformance，也不宣称生产隔离或 Native Resume 已新增验收。
- 未配置 `WORKSPACE_TEST_POSTGRES_DSN`，完整 Go 测试中的相关 PostgreSQL 集成 Skip 不算通过；真实本地 PostgreSQL 的设置保存与会话持久化仅证明上述端到端子集。
- 未执行 Aliyun OSS、完整 MinIO Conformance、其他真实 Runtime/供应商组合、Workflow/Git/附件业务全流程。CLI Connector、CLI Builder、消息渠道在 local 模式中不支持；本地 bridge 不提供生产公网 Egress 隔离。
- 代码或 Schema 回退、持久数据恢复没有演练。本地重启已保留企业默认、供应商和会话；生产配置未变更。
