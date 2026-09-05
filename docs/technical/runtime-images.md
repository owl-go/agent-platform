# Runtime Images

状态：Runtime、隔离 CLI Builder、User Run 的已校验只读 bundle 挂载，以及公共 broker 协议与 Runtime 客户端已实现；受 Egress Gate 保护的 Sandbox 命令进程和 broker socket 容器契约已实现，细粒度 Egress 控制器、Runtime 绑定与 User Action Wait 尚未接入；部署前必须重新构建并记录 RepoDigest

| Runtime Engine | CLI | 固定版本 |
|---|---|---:|
| Claude Code | `@anthropic-ai/claude-code` | `2.1.233` |
| Codex | `@openai/codex` | `0.147.0` |
| Hermes | `hermes-agent[mcp,anthropic]` | `0.19.0` |
| OpenClaw | `openclaw` | `2026.7.1-2` |
| PI Agent | `@earendil-works/pi-coding-agent` | `0.84.4` |

五个 Dockerfile 位于 `deploy/runtimes/<runtime>/Dockerfile`。每个镜像只安装一种 Runtime Engine，并共同提供 Git、`npx`、`uvx` 与 MCP 运行依赖；进程固定以 UID/GID 65532 运行。生产配置只能引用 Registry `repository@sha256:<digest>`，不能使用 Tag 或本地 Image ID。

公共 Entrypoint 创建 tmpfs HOME，从只读 Credential Mount 导入模型与 Connector 环境变量，并复制 Runtime 配置到 HOME。SSH Git 仅在同时存在私钥与管理员预置 `known_hosts` 时启用，固定 `StrictHostKeyChecking=yes`。

Third-party CLI 不烘焙进 Runtime 镜像，也不在 User Run 中动态安装。管理员发布的固定版本 npm 包由隔离 Builder 生成不可变 bundle；Sandbox 只读挂载后由公共 CLI Connector Wrapper 调用。一个 Connector 组合只有在 exact bundle SHA-256 与 Runtime RepoDigest 的联合 Conformance 通过后才可标记 available。

CLI Builder 使用 `deploy/runtimes/cli-builder/Dockerfile`。Worker 仅在 `worker.cli_builder.enabled` 为 true，且 Builder 镜像为 RepoDigest、Egress Network 与超时均显式配置时装配它；验证集合只取当前可用 Runtime 的配置 RepoDigest。Builder 禁用或配置不完整时发布 fail closed。

Codex 调用会把本次 Run Scratch 中已校验的 `image/*` 只读附件逐个传给 `codex exec --image`；文件名和用户文本仍分别通过受控参数与 stdin 传递。其他 Runtime 当前仅通过公共 Instruction 中的只读路径读取附件，不声明图片输入已经通过固定镜像 Conformance。

镜像变更至少执行：独立构建、CLI `--version`、非 root/只读 Rootfs、五 Runtime 最小真实模型调用、适用 Runtime 的 MCP 配置加载、声明兼容的 CLI Connector bundle、取消、输出脱敏和 Workspace 写入 smoke。某镜像没有这些证据时，对应 `available` 必须为 `false`。
