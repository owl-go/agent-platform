# Production Conformance Runner

状态：自动化套件已实现，等待 Linux + gVisor 外部环境执行

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

`testdata/production-conformance` 是统一黑盒示例仓库。它包含一个会失败的 Shell 测试、一个明确失败的命令和一个用于 Interrupt/强杀场景的长命令。四种 Runtime 必须从同一基线分别执行，不能共享前一个 Runtime 修改后的 Workspace。

管理员使用 `scripts/conformance/prepare-production-repository.sh` 把 Fixture 推送到一个新的远端基线分支；脚本拒绝覆盖已有分支。

## 完整套件

`scripts/conformance/production-preflight.sh` 在任何模型调用前检查 Linux、`runsc`、公网 Egress Network、四个 RepoDigest、四组模型/Git 凭证、单行 Canary、Git 仓库、MinIO、阿里云 OSS，以及 Redirect、DNS Rebinding 和控制面拒绝测试地址。检查只输出缺失配置名称，不输出凭证值。

通过预检后执行：

```bash
scripts/conformance/production.sh
```

外层套件对四种 Runtime 分别自动执行：

1. 在 gVisor Sandbox 内通过固定 SSH Key 和 `known_hosts` Clone 同一 Fixture；
2. 运行失败命令、Secret Canary Probe、代码修改和成功测试；
3. 在测试通过后强杀 Container；
4. 把 Workspace Snapshot 分别上传到 MinIO 和阿里云 OSS并执行回读校验；
5. 从两个 Provider 分别恢复，比较归档 SHA-256，并从 MinIO 恢复的 Workspace 继续 Runtime；
6. 在 Sandbox 内复测、Commit 并 Push 独立 Review Branch；
7. 对长命令分别执行 Interrupt、Cancel、Timeout，验证错误分类和 Container 清理；
8. 扫描 Workspace 与全部 Evidence，确认 Canary 没有持久化；
9. 上传脱敏 Evidence Artifact 到两个 Provider；
10. 汇总镜像 RepoDigest、实际 CLI 版本、Capability、耗时、Usage、预期失败、Snapshot 和 Review Branch。

Sandbox 网络测试同时要求公网可达，并拒绝 Loopback、Docker Host Gateway、云元数据、RFC1918 私网、公共 URL 到私网的重定向、解析到私网的 DNS Rebinding 地址和平台控制面地址。

`cmd/conformance-artifact` 使用统一 Object Storage Provider 创建 tar Snapshot。归档上传后立即回读并核验大小和 SHA-256；恢复器拒绝绝对路径、`..` 穿越、重复路径、特殊文件和越界 Symlink。Evidence 和 Workspace 都通过相同路径验证，不使用 Provider 专有对象地址。

只有所有命令成功后，`summary.json` 才写入 `decision: "GO"`。任何 Runtime、网络、Git、Secret、Snapshot 或 Provider 场景失败都会让脚本非零退出，不生成成功决策。真实套件未执行前，仓库内 Phase 0 决策仍保持 NO-GO。
