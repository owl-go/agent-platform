# Runtime Adapter

状态：Agent Workspace 当前实现

Worker 只依赖 `agentruntime.Adapter` 的 `Describe` 和 `Execute`。Claude Code、Codex、Hermes 与 OpenClaw 的命令参数、版本探测和输出解析保留在各自 Driver，共享的进程、容器和事件行为位于 `cliadapter`、`processharness` 与 `containerprocess`。

一个 Execute 接收不可变的 Run/Session 输入、临时 Workspace、Model ID、凭证引用、可选原生 Resume Checkpoint 和 MCP 配置路径。Instruction 通过 stdin 或受控临时文件传递，绝不拼接 Shell 命令。

事件约束：

- Event 的 Run ID 必须与请求一致；Sequence 从 1 严格递增。
- stdout/stderr、结构化 delta、错误与终态在持久化前经过同一组 Secret 精确值脱敏。
- Event Sink 写入失败立即停止 Runtime，避免执行继续而审计记录丢失。
- 取消通过 Context 传播并终止完整进程组或 Container。
- 最终结果只有在 Runtime 成功、Workspace 安全检查和 Artifact 保存全部成功后才提交。

Session 连续性优先使用平台保存的最近消息与有界 Rolling Summary。只有某 Runtime 配置 `native_resume: true` 且该镜像通过验证时，才把原生 Checkpoint 作为优化；切换 Runtime 时自动放弃原生 Checkpoint。

当前部署固定的 Codex CLI `0.147.0` 已验证 `thread_id` 的保存与 `codex exec resume <thread_id>` 续接，允许开启 `native_resume`。每个 Run 只把用户与 Session 双重隔离的临时副本挂载到容器 `$CODEX_HOME`，成功后仅将经过精确 Secret 脱敏的 `sessions/` 原子写回；插件缓存、日志、认证文件和 MCP 配置均不持久化。MCP 配置从单 Run Credential 目录建立临时符号链接。API 删除 Session 时同步清理状态目录。其他 Runtime 保持关闭，直到各自固定镜像完成同等黑盒验证。

`ExecuteRequest.ModelEndpoint`、`ModelProvider` 与 `ModelProtocols` 来自发送消息或启动 Workflow Run 时冻结的 Response/Workflow Snapshot。Driver 必须把它们作为结构化参数或受控配置传给 CLI，禁止拼接 Shell；Endpoint 必须是无 Userinfo、Query 和 Fragment 的绝对 HTTP 或 HTTPS URL。HTTP 仅用于 User 明确配置的可信私有或自托管网关，API Key 与模型流量不会获得传输加密；内置官方 Endpoint 仍全部使用 HTTPS。API Key 通过对应 Model Provider Connection 版本的受保护凭证取得，并且只进入单 Run 临时环境。Codex Driver 使用固定的 `agent_workspace` Custom Provider，由 `OPENAI_API_KEY` 读取 Secret，并要求连接支持 OpenAI Responses；Claude Code 要求 Anthropic Messages。Hermes 与 OpenClaw 在完成指定镜像和协议组合的 Conformance 前保持 `unverified`。只有绕过产品 Model Provider Connection 的独立 Conformance 调用可以省略 Endpoint，此时 Driver 使用其官方 HTTPS Endpoint。

MCP 配置在每次执行中生成：Claude 使用 `--mcp-config`，Codex 使用 `$HOME/.codex/config.toml`，Hermes 使用 `$HOME/.hermes/config.yaml`，OpenClaw 使用受控配置文件。stdio MCP 只允许固定版本的 `npx`/`uvx` 包；Streamable HTTP 只允许 HTTPS 与可选写入型 Bearer Token。
