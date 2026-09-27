# Runtime Images

状态：Runtime、隔离 CLI Builder、User Run 的已校验 bundle 物化，以及公共 broker 协议与 Runtime 客户端已实现；模型 Runtime 不挂载 CLI bundle，专用短生命周期 Connector Container、只读 broker socket、Runtime/broker 绑定、窄协议宿主 Egress Controller、存活 Worker 内持久化 User Action Wait 与一次性命令绑定，以及飞书 CLI Application 设备流注册和加密绑定已实现；飞书 User 设备授权、加密 Token、断开即时阻断、能力 Scope 复验和单次 Connector Container 环境注入已实现；Token 刷新、Bot 权限恢复、Worker 重启恢复和 Linux + gVisor 端到端证据尚未完成；部署前必须重新构建并记录 RepoDigest

| Runtime Engine | CLI | 固定版本 |
|---|---|---:|
| Claude Code | `@anthropic-ai/claude-code` | `2.1.233` |
| Codex | `@openai/codex` | `0.147.0` |
| Hermes | `hermes-agent[mcp,anthropic]` | `0.19.0` |
| OpenClaw | `openclaw` | `2026.7.1-2` |
| PI Agent | `@earendil-works/pi-coding-agent` | `0.84.4` |

五个 Runtime Engine 统一安装在 `deploy/runtimes/unified/Dockerfile` 构建的一个镜像中。镜像以固定 Python 3.13 基础镜像提供 Hermes，复制固定 Node 24 镜像的 Node/npm 工具链，提供 Git、`npx`、`uvx` 与 MCP 运行依赖；进程固定以 UID/GID 65532 运行。`scripts/build-runtime-images.sh` 只构建一个 Runtime 镜像及独立 CLI Builder。平台的五个 Runtime Engine 配置引用同一个 `RUNTIME_IMAGE` RepoDigest，但各自保持 CLI 版本、Capability 和可用状态。冷启动与 Warm Container 均由平台明确传入引擎命令，不依赖镜像默认 Entrypoint 选择引擎。生产配置只能引用 Registry `repository@sha256:<digest>`，不能使用 Tag 或本地 Image ID。

公共 Entrypoint 创建 tmpfs HOME，从只读 Credential Mount 导入模型与 Connector 环境变量，并复制 Runtime 配置到 HOME。公共 `agent-cli` 客户端使用 CommonJS，可由镜像内 Node 24 直接执行无扩展名的 `/usr/local/bin/agent-cli`。SSH Git 仅在同时存在私钥与管理员预置 `known_hosts` 时启用，固定 `StrictHostKeyChecking=yes`。

Third-party CLI 不烘焙进 Runtime 镜像，也不在 User Run 中动态安装。管理员提交的固定版本 npm 包或已校验 ZIP 包由隔离 Builder 生成不可变 bundle；可执行文件和执行策略从内置 profile 或包内 `agentWorkspace` 元数据解析并再次校验。Sandbox 只读挂载后由公共 CLI Connector Wrapper 调用。一个 Connector 组合只有在 exact bundle SHA-256 与 Runtime RepoDigest 的联合 Conformance 通过后才可标记 available。

CLI Builder 使用 `deploy/runtimes/cli-builder/Dockerfile`。Worker 仅在 `worker.cli_builder.enabled` 为 true，且 Builder 镜像为 RepoDigest、Egress Network 与超时均显式配置时装配它；构建和 Conformance 输出分别创建在 Worker 已映射到宿主同路径的 `credential_temp_root/cli-build` 与 `credential_temp_root/cli-conformance` 下，Docker 只把本次任务目录挂入无凭证容器。Builder 镜像和 `/work` tmpfs 都把工作目录直接归固定非 root 用户 `65532:65532`，无需容器内提权。npm lifecycle script 只允许在这个无凭证、受限资源的构建容器内执行，以支持安装阶段物化固定版本二进制的 CLI；User Run 和 Conformance 均不执行安装脚本。验证集合只取当前可用 Runtime 的配置 RepoDigest。Builder 禁用或配置不完整时发布 fail closed；这些均为平台部署配置，不增加管理员发布 CLI Definition 的操作步骤。

完整平台部署会以同一 Release ID 构建并推送统一 Runtime 镜像与 CLI Builder，在五个 Runtime CLI、非 root、只读 Rootfs 和 broker 客户端 smoke 通过后，将远端 `RUNTIME_IMAGE` 和 CLI Builder 配置原子更新为各自的 Registry RepoDigest，再重建 Worker；因此 Runtime 内容、协议读取逻辑与 Builder 输出不会跨版本漂移。首次从五镜像部署升级时，统一 Runtime Repository 从原 Codex 镜像所在 Registry 命名空间推导，后续发布复用已经固定的 `RUNTIME_IMAGE` Repository。

Codex 调用会把本次 Run Scratch 中已校验的 `image/*` 只读附件逐个传给 `codex exec --image`；文件名和用户文本仍分别通过受控参数与 stdin 传递。其他 Runtime 当前仅通过公共 Instruction 中的只读路径读取附件，不声明图片输入已经通过固定镜像 Conformance。

镜像变更至少执行：统一镜像构建、五个 CLI 的 `--version`、非 root/只读 Rootfs、`agent-cli` 与当前 Worker broker 的 `-- <argv>` 协议 smoke、五 Runtime 最小真实模型调用、适用 Runtime 的 MCP 配置加载、声明兼容的 CLI Connector bundle、取消、输出脱敏和 Workspace 写入 smoke。仅检查 `agent-cli` 在没有 broker socket 时退出非零不足以证明协议兼容，必须确认它已解析命令并尝试连接 socket。一个共享 Digest 不代表五个 Runtime Engine 均已通过 Conformance；某引擎没有这些证据时，仅该引擎的 `available` 必须为 `false`。新的 Digest 还需重验受影响的 CLI Connector bundle。
