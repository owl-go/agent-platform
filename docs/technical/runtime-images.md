# Runtime Images 与 CLI Adapter

状态：本地镜像构建与 smoke test 完成，等待 Production Conformance

## 版本基线

| Runtime | CLI 包 | 固定版本 | 基础运行时 |
|---|---|---:|---|
| Claude Code | `@anthropic-ai/claude-code` | `2.1.233` | Node 22.22 |
| Codex CLI | `@openai/codex` | `0.147.0` | Node 22.22 |
| Hermes Agent | PyPI `hermes-agent` | `0.19.0` | Python 3.13.11 |
| OpenClaw | npm `openclaw` | `2026.7.1-2` | Node 24.15 |

四个 Dockerfile 位于 `deploy/runtimes/<runtime>/Dockerfile`，独立安装且只包含一种 Agent Runtime。镜像以 UID/GID 65532 运行，CLI 全局安装目录归 root；结合 Sandbox 只读 Rootfs，Runtime 无法在 Run 中覆盖自身版本。

Dockerfile 使用 public ECR 的 Docker Official Images 镜像并固定基础 Manifest Digest，同时固定 CLI 包版本。`scripts/build-runtime-images.sh` 只为本机构建记录 Image ID；Production CI 必须 Push 后记录 Registry 返回的 `repository@sha256:<digest>`。Agent Release 不接受 Tag 或本地 Image ID 作为不可变引用。

## Credential Entrypoint

公共 `runtime-entrypoint` 在启动 CLI 前：

1. 创建位于受限 tmpfs 的 `$HOME`；
2. 从本次 Run 的只读 `/run/agent-credentials/env/<NAME>` 文件导出模型与构建环境变量；
3. 若存在 `git/id_ed25519`，要求同时存在固定的 `git/known_hosts`，并启用 `StrictHostKeyChecking=yes`；
4. 通过 `exec` 启动镜像内固定 CLI。

Instruction 不作为 Secret 处理，但 Claude/Codex 使用 stdin，OpenClaw 使用 0600 scratch 文件。Hermes 0.19.0 的 one-shot 接口只接受参数，因此命令诊断必须经过同一 Redactor。模型 ID 是平台配置的 opaque 字符串；Adapter 不验证模型提供商，也不维护提供商白名单。

## Invocation

### Claude Code

使用 `--print --output-format stream-json --include-partial-messages`。`--bare`、strict MCP 和禁用 Slash Command 防止容器读取未审核的用户级插件、Hook 与 Keychain。CLI 的权限确认在外层 gVisor Sandbox 内自动放行。Parser 提取文本增量、工具请求、最终消息、Session ID 和 Usage。

### Codex CLI

新会话使用 `codex exec --json ... -`，恢复使用 `codex exec resume ... <checkpoint> -`。Instruction 走 stdin；`--dangerously-bypass-approvals-and-sandbox` 只在平台外层 Sandbox 已验证后使用。Parser 提取 Thread ID、命令开始/完成、文件变化、最终消息和 Usage。

### Hermes Agent

使用 `hermes --oneshot`、`--safe-mode` 与 built-in toolsets；one-shot 本身会自动批准工具，只允许在外层 Sandbox 中运行。最终文本来自 stdout，Usage/成本来自本次 scratch 下的 `usage.json`。原生 Resume 尚未验证，Checkpoint 输入会被拒绝。

### OpenClaw

固定使用 `openclaw agent --local`，不连接 Gateway，不交付到聊天渠道。Instruction 通过 `--message-file`；Adapter 拒绝 `claude-cli`、`codex-cli` 和 `cli-backend` 风格 Model ID，防止 OpenClaw 再路由另一个 Coding CLI。原生 Resume、稳定流式事件和 Usage 尚未验证，保持关闭。

## Capability Gate

Driver 能解析某字段不等于 Release 已验证该 Capability。`cliadapter.Config.VerifiedCapabilities` 默认空；只有 P0-08 对某个镜像 Digest 和 CLI 版本通过场景后，发布配置才可以启用对应 Capability。

当前默认矩阵全部为关闭：

| Runtime | streaming | native_resume | structured_final | subagents | usage |
|---|---:|---:|---:|---:|---:|
| Claude Code | off | off | off | off | off |
| Codex CLI | off | off | off | off | off |
| Hermes Agent | off | off | off | off | off |
| OpenClaw | off | off | off | off | off |

P0-08 报告记录从 off 调整为 on 的逐项证据，不按品牌或文档推断能力。

## Tests

- 各 Driver 单元测试固定参数、版本解析和代表性输出解析。
- `TestRuntimeAdaptersShareContract` 用同一场景覆盖四种 Adapter 的 Describe、Execute、事件终态和默认 Capability。
- `TestRuntimeDockerfilesPinOneCLIAndNonRootUser` 检查精确版本、单 CLI EntryPoint 和非 root 用户。
- `scripts/conformance/runtime-image-smoke.sh` 在 Docker 可用时验证镜像内真实版本和 UID。
- 真实代码修改、测试、Interrupt、Snapshot 重建、临时 SSH Key Push 与模型 Usage 属于 P0-08；必须使用镜像 Digest 和真实 gVisor Worker。
