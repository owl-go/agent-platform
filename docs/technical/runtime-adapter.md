# Runtime Adapter 技术规格

状态：Phase 0 实施基线

## 目标

Runtime Adapter 是 Go Worker 与 Agent Runtime CLI 之间唯一的 seam。Worker 只理解平台的 Run、Event、Result 和 Capability，不理解 Claude Code、Hermes Agent、OpenClaw 或 Codex CLI 的参数、状态目录和输出格式。

完成标准：四种 Runtime 在不修改 Worker 调用代码的前提下，通过同一套 Conformance Suite。

## Interface

权威 Go Interface 位于 `internal/agentruntime/contract.go`：

```go
type Adapter interface {
    Describe(ctx context.Context) (Descriptor, error)
    Execute(ctx context.Context, request ExecuteRequest, events EventSink) (Result, error)
}
```

### Describe

返回 Runtime 名称、版本和经过验证的 Capability。描述来自实际镜像中的 CLI，不信任控制面提交的字符串。

### Execute

负责一个 Runtime 执行的完整生命周期：准备 Runtime 状态、启动非交互 CLI、标准化事件、响应取消、保存可用 Checkpoint、收集最终结果并回收子进程。

调用约束：

- 一个 `Execute` 只处理一个 Run Attempt。
- `RunID`、Workspace 和 Environment 引用必须在启动前校验。
- `context` 取消后，Adapter 先请求优雅退出；超过 Deadline 后终止整个进程组。
- `EventSink.Publish` 失败时停止执行，防止产生无法审计的后续行为。
- Adapter 返回前必须等待所有子进程退出并关闭输出管道。
- Runtime 原生 Session ID 只能作为 Checkpoint 元数据，不能替代平台 Session。

## Request

`ExecuteRequest` 只携带平台已经冻结的执行输入：

- `RunID`
- `WorkspacePath`
- `Instruction`
- `Model`
- 可选 `CheckpointRef`
- `EnvironmentRef`

`EnvironmentRef` 指向 Worker 准备的临时执行环境。它不是 Secret 内容，Adapter 不从数据库或 Secret Manager 自行读取凭证。

## Result

`Result` 包含：

- 用户可见最终消息
- Runtime 退出码
- Diff Artifact 引用
- 可用 Checkpoint 引用
- Runtime 能准确提供的 Usage

进程退出成功不等于 Coding Task 完成。Worker 根据结果、质量门禁和用户决策更新产品状态。

## 标准 Event

所有事件具有 `RunID`、单调递增 `Sequence`、`OccurredAt`、`Kind` 和 JSON Payload。

必需事件：

- `runtime.started`
- `message.completed`
- `command.requested`
- `command.completed`
- `file.changed`
- `runtime.completed` 或 `runtime.failed`

按 Capability 产生：

- `message.delta`
- `approval.requested`
- `usage.updated`
- `checkpoint.saved`

Adapter 必须保留 Runtime 原始 stdout/stderr 作为脱敏 Artifact，标准事件只保存平台需要查询的结构化字段。

## Capability

首批 Capability：

| Capability | 含义 |
|---|---|
| `streaming` | 执行期间能产生稳定文本增量 |
| `native_resume` | 能从 Runtime 原生 Session 恢复 |
| `structured_final` | 能约束并验证最终结构化结果 |
| `subagents` | Runtime 可以在父 Run 内使用原生 Subagent |
| `usage` | 能准确报告 Token 或模型成本 |

Capability 必须由镜像 Conformance Test 证明。未验证能力保持关闭。

## 进程约束

每种 Adapter 定义固定的可执行文件和参数模板。用户输入只通过受控 stdin、参数值或临时文件传递，不能拼接为 Shell 命令。

Adapter 必须：

- 使用独立进程组；
- 分开读取 stdout 和 stderr；
- 限制单行和总输出大小；
- 对输出执行 Secret Redaction；
- 将非 UTF-8 输出作为二进制 Artifact；
- 记录 CLI 版本和最终参数的脱敏形式；
- 拒绝 Runtime 在 Run 中自更新。

## Sandbox Interface

Adapter 不直接调用 Docker。Worker 通过 Sandbox Runner 创建 gVisor Container，并在 Container 内启动 Runtime Adapter Entrypoint。

Sandbox Runner 负责：

- 镜像 Digest
- 非 root UID/GID
- 只读根文件系统
- Workspace 挂载
- CPU、内存、PID、磁盘和时长限制
- Egress Policy
- 临时凭证环境
- Container Stop/Destroy

这样可在不修改 Runtime Adapter 的情况下替换 Docker 执行实现。

## 恢复

平台恢复始终可用，原生恢复是优化：

1. Worker 读取 Agent Release、Session Memory 和 Workspace Snapshot。
2. 若 Runtime 声明 `native_resume` 且 Checkpoint 有效，将引用传入 Adapter。
3. 否则创建新 Runtime Session，并把已确认摘要和当前 Workspace 作为输入。
4. 恢复使用原 Agent Release、Runtime 镜像、Model Binding 和权限。

平台不承诺恢复后模型内部上下文逐 Token 相同。

## Adapter 目录

目标目录：

```text
internal/agentruntime/
  contract.go
  conformance/
  claude/
  codex/
  hermes/
  openclaw/
```

每个 Adapter 包只包含该 CLI 的参数、输出解析、状态目录和错误映射。共享进程、事件和脱敏逻辑放在 `internal/agentruntime` 的内部 Module 中，不复制到四个 Adapter。

## Conformance Suite

每个 Runtime 镜像必须执行同一组黑盒场景：

1. 描述实际 Runtime 版本和 Capability。
2. 在示例仓库完成一次小修改并产生 Diff。
3. 运行成功和失败的测试命令。
4. 在输出中生成已知假 Secret，证明持久结果已脱敏。
5. Interrupt 长命令并证明进程组退出。
6. 强杀 Container 后从 Snapshot 重建。
7. 使用临时 SSH Key Push Review Branch。
8. 尝试访问宿主机、元数据和私网，证明 Egress Policy 阻止访问。
9. 验证声明的每个可选 Capability。

macOS 只运行单元测试和普通 Docker smoke test。Production Conformance 必须在安装 `runsc` 的 Linux Worker 执行。

## 错误分类

Adapter 将错误归一化为：

- `invalid_configuration`
- `runtime_unavailable`
- `authentication_failed`
- `model_failed`
- `command_failed`
- `budget_exhausted`
- `interrupted`
- `timed_out`
- `event_delivery_failed`
- `internal_adapter_error`

错误分类决定 Run 状态和重试资格；原始 CLI 错误只作为脱敏诊断信息保存。

