# Production Conformance Runner

状态：执行链与证据生成器已实现，等待 Linux + gVisor 外部环境执行

## 目的

`cmd/runtime-conformance` 把 Runtime Adapter 与 Docker + gVisor 接成同一条执行链。它不是宿主机 CLI smoke test：Adapter 仍负责生成参数和解析标准事件，但实际 CLI 进程由 `internal/agentruntime/containerprocess` 包装为受限的 `docker run --runtime runsc`。

容器执行固定以下策略：

- Runtime 镜像必须是 `repository@sha256:<digest>`；
- 非 root UID/GID、只读 Rootfs、`cap-drop ALL` 和 `no-new-privileges`；
- Code Workspace 为唯一可写业务挂载，凭证目录只读；
- `/tmp` 为 `noexec,nosuid,nodev` tmpfs；
- CPU、内存、PID、临时磁盘和公网 Egress Network 受限；
- Container 带受管 Label，正常、失败和取消路径都执行强制清理；
- Runtime 可执行文件必须与镜像类型一致，防止 Adapter/Image 串配。

## 输入

调用方准备：

1. 安装 `runsc` 的 Linux Docker Worker；
2. 已 Push 的 Runtime Registry RepoDigest；
3. 一个本次 Run 独占、绝对路径表示的 Git Workspace；
4. 由 `credentials.Materializer` 生成且归 UID/GID 65532 所有的凭证目录；
5. 位于 Workspace 外的 Evidence 目录；
6. 已配置模型名称和公网 Egress Network。

凭证目录沿用 Runtime EntryPoint 契约：模型环境变量位于 `env/<NAME>`，Git 凭证位于 `git/id_ed25519` 和 `git/known_hosts`。

## 示例

```bash
go run ./cmd/runtime-conformance \
  --runtime codex \
  --image registry.example/agent-platform/codex@sha256:<digest> \
  --model configured-model \
  --workspace /var/lib/agent-platform/workspaces/codex-p0 \
  --credentials /run/agent-platform/credentials/codex-p0 \
  --output /var/lib/agent-platform/evidence/codex-p0 \
  --run-id codex-p0 \
  --network agent-public-egress \
  --instruction "$(cat testdata/production-conformance/task.txt)"
```

同一个命令通过 `--runtime`、`--image` 和 `--model` 切换 Claude Code、Codex、Hermes 与 OpenClaw，不改变 Worker 调用契约。

## 证据

每次执行输出：

| 文件 | 内容 |
|---|---|
| `report.json` | Run ID、镜像 RepoDigest、实际 CLI 版本、Capability、耗时、结果、Usage 和错误分类 |
| `events.jsonl` | 已脱敏、可逐行读取的标准 Runtime Event |
| `stdout.log` / `stderr.log` | 已脱敏的原始 Runtime 输出 |
| `workspace.diff` | 相对于执行前 HEAD 的已脱敏 Git binary diff |

命令在成功前额外执行两个 fail-closed 检查：Repository 必须产生 Diff；Workspace 中不能出现任何已注入凭证的精确值。Evidence 目录禁止放在 Workspace 内，防止 Agent 修改验收证据。

## Fixture

`testdata/production-conformance` 是统一黑盒示例仓库。它包含一个会失败的 Go 测试、一个明确失败的命令和一个用于 Interrupt/强杀场景的长命令。四种 Runtime 必须从同一基线分别执行，不能共享前一个 Runtime 修改后的 Workspace。

当前命令覆盖真实 Runtime 修改、标准事件、Diff、超时/取消入口、容器清理与 Secret 持久化扫描。Clone/Push、强杀后重建、网络逃逸、OSS/MinIO Artifact 上传和汇总报告仍由 P0-08 外层套件编排；未完成前 Phase 0 保持 NO-GO。
