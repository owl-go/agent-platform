# Runtime Images

状态：构建定义已实现，部署前必须重新构建并记录 RepoDigest

| Runtime Engine | CLI | 固定版本 |
|---|---|---:|
| Claude Code | `@anthropic-ai/claude-code` | `2.1.233` |
| Codex | `@openai/codex` | `0.147.0` |
| Hermes | `hermes-agent[mcp]` | `0.19.0` |
| OpenClaw | `openclaw` | `2026.7.1-2` |

四个 Dockerfile 位于 `deploy/runtimes/<runtime>/Dockerfile`。每个镜像只安装一种 Runtime Engine，并共同提供 Git、`npx`、`uvx` 与 MCP 运行依赖；进程固定以 UID/GID 65532 运行。生产配置只能引用 Registry `repository@sha256:<digest>`，不能使用 Tag 或本地 Image ID。

公共 Entrypoint 创建 tmpfs HOME，从只读 Credential Mount 导入模型/扩展环境变量，并复制 Runtime 配置到 HOME。SSH Git 仅在同时存在私钥与管理员预置 `known_hosts` 时启用，固定 `StrictHostKeyChecking=yes`。

镜像变更至少执行：独立构建、CLI `--version`、非 root/只读 Rootfs、四 Runtime 最小真实模型调用、MCP 配置加载、取消、输出脱敏和 Workspace 写入 smoke。某镜像没有这些证据时，对应 `available` 必须为 `false`。
