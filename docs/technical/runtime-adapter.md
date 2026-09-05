# Runtime Adapter

状态：当前 Runtime 契约；Expert 与 Connector 简化为已接受但尚未实现的目标修订

Worker 只依赖 `agentruntime.Adapter` 的 `Describe` 和 `Execute`。Claude Code、Codex、Hermes、OpenClaw 与 PI Agent 的命令参数、版本探测和输出解析保留在各自 Driver，共享的进程、容器和事件行为位于 `cliadapter`、`processharness` 与 `containerprocess`。

一个 Execute 接收不可变的 Run/Session 输入、临时 Workspace、Model ID、凭证引用、可选原生 Resume Checkpoint、Skill 与 Connector 快照、MCP 配置路径和已校验的只读附件路径/Content-Type。Expert 的 Core Capability、Operating Procedure、Output Standard、Cautions 按固定可见标题组装；Introduction 和 Derived Expertise Tags 不进入 Instruction。Team Member 名称和标签位于该 Expert 指令之前。Instruction 通过 stdin、独立参数值或受控临时文件传递，绝不拼接 Shell 命令。附件路径位于 Runtime Workspace 文件访问边界内的 `/workspace/.agent-platform-attachments`，但由独立只读挂载提供，不属于可持久化 Workflow Workspace。所有 Runtime 都可按 Instruction 中的路径读取普通文件；Codex Driver 还会把 `image/*` 附件作为重复的 `--image <path>` 参数传入新会话与 Resume 调用，使多模态模型实际接收图片内容。

Runtime JSONL 的单个结构化事件允许增长到本次执行的总输出上限，因为 Codex 等 CLI 会在一行 `command_execution` 事件中携带完整工具输出。进程输出仍受 64 MB 总量硬限制；超过总量时立即终止，不能用较小的通用单行限制误杀合法事件。

事件约束：

- Event 的 Run ID 必须与请求一致；Sequence 从 1 严格递增。
- stdout/stderr、结构化 delta、错误与终态在持久化前经过同一组 Secret 精确值脱敏。
- Runtime 可发布 `reasoning.summary` 作为面向 User 的公开推理摘要；它不是原始 chain-of-thought。Session 只持久化经过脱敏和长度限制的活动摘要、命令及状态，不持久化工具输出作为活动明细。
- Event Sink 写入失败立即停止 Runtime，避免执行继续而审计记录丢失。
- 取消通过 Context 传播并终止完整进程组或 Container。
- 最终结果只有在 Runtime 成功、Workspace 安全检查和 Artifact 保存全部成功后才提交。

Session 连续性优先使用平台保存的最近消息与有界 Rolling Summary。只有某 Runtime 配置 `native_resume: true` 且该镜像通过验证时，才把原生 Checkpoint 作为优化；切换 Runtime 时自动放弃原生 Checkpoint。Expert Team 为每个冻结成员维护独立的临时 Checkpoint 状态，只有整轮成功才共同晋升，任一成员失败或取消都丢弃本轮全部临时状态。

Workflow 的持续对话由 Run Conversation 提供。每次追问创建新的 Run，Worker 将同一 Conversation 的既有 User/Assistant 轮次和当前输入通过公共 Instruction seam 交给 Runtime；原生 Resume 只作为优化。Codex 的匿名 Workflow Conversation 与 Expert Conversation 都按 Conversation 隔离并持久化脱敏后的 `sessions/`；如果 Worker 或容器重建后只有数据库 Checkpoint 而本地原生状态缺失，Worker 在执行前放弃该 Checkpoint，并依靠完整公共 Instruction 启动新原生会话。该恢复不会重开或改写已经终态的 Run。

当前部署固定的 Codex CLI `0.147.0` 已验证 `thread_id` 的保存与 `codex exec resume <thread_id>` 续接，允许开启 `native_resume`。每个 Run 只把用户与 Session 双重隔离的临时副本挂载到容器 `$CODEX_HOME`，成功后仅将经过精确 Secret 脱敏的 `sessions/` 原子写回；插件缓存、日志、认证文件和 MCP 配置均不持久化。MCP 配置从单 Run Credential 目录建立临时符号链接。API 删除 Session 时同步清理状态目录。其他 Runtime 保持关闭，直到各自固定镜像完成同等黑盒验证。

`ExecuteRequest.ModelEndpoint`、`ModelProvider` 与 `ModelProtocols` 来自当前 Execution Stage Snapshot；Session 首次发送消息或 Run Conversation 启动时从 Personal Settings 解析一次，无 Expert、单 Expert 和 Expert Team 的全部 Stage 共用该冻结配置。Driver 必须把它们作为结构化参数或受控配置传给 CLI，禁止拼接 Shell；Endpoint 必须是无 Userinfo、Query 和 Fragment 的绝对 HTTP 或 HTTPS URL。HTTP 仅用于 User 明确配置的可信私有或自托管网关，API Key 与模型流量不会获得传输加密；内置官方 Endpoint 仍全部使用 HTTPS。API Key 通过对应 Stage 的 Model Provider Connection 版本受保护凭证取得，并且只进入单次执行临时环境。Codex Driver 使用固定的 `agent_workspace` Custom Provider，由 `OPENAI_API_KEY` 读取 Secret，并要求连接支持 OpenAI Responses；Claude Code 要求 Anthropic Messages。DeepSeek 的连接 Endpoint 保持为同时服务 OpenAI 协议的 `https://api.deepseek.com`，Worker 只在物化 `ANTHROPIC_BASE_URL` 时派生其官方 `/anthropic` 路径。PI Agent Driver 为当前 Stage 生成隔离的 `models.json`，由单次执行的 `OPENAI_API_KEY` 读取同一受保护 Secret，并把平台协议映射到 PI 支持的 OpenAI Responses、OpenAI Chat、Anthropic Messages 或 Google Generative AI API。OpenClaw Driver 同样为当前 Stage 生成隔离的 `openclaw.json`，使用固定的 `agent-workspace` Provider 注册平台 Model ID、Endpoint 和所选协议，并显式保持 OpenClaw 内置 Agent Runtime；OpenAI 与 Anthropic Secret 分别通过配置中的环境变量引用读取。Hermes、OpenClaw 与 PI Agent 在完成指定镜像和协议组合的 Conformance 前保持 `unverified`。只有绕过产品 Model Provider Connection 的独立 Conformance 调用可以省略 Endpoint，此时 Driver 使用其官方 HTTPS Endpoint。

MCP Connector 配置在每次执行中生成：Claude 使用 `--mcp-config`，Codex 使用 `$HOME/.codex/config.toml`，Hermes 使用 `$HOME/.hermes/config.yaml`，OpenClaw 使用受控配置文件。stdio MCP 只允许固定版本的 `npx`/`uvx` 包；Streamable HTTP 只允许 HTTPS 与可选写入型 Bearer Token。

Third-party CLI Connector 不进入各 Runtime Driver。公共 CLI Connector Wrapper 根据冻结 Definition 和当前 User-private Authorization 生成真实可执行文件与参数数组，强制检查 bundle Digest、Runtime RepoDigest、能力、身份、argv、scope、Egress、Workspace、输出和超时，并在进程启动前再次检查 Definition、Enablement、Authorization 与批准状态。当前 Secret 只为这一次命令物化、加入精确值脱敏集合并幂等清理。

高风险命令先持久化绑定 nonce 与完整命令摘要的一次性批准请求，然后令 Session response 或 Run 进入 `waiting_for_user`。每个 Execution Stage 同时只暴露一个请求；等待期间保留 Runtime 和临时 Workspace、暂停普通执行超时并继续响应取消。只有认证的 owning User 可决定，拒绝或超时作为结构化 CLI 错误返回 Runtime；批准消费后才可启动进程，且整个执行仍遵守单调 Event Sequence 与唯一终态。

PI Agent 固定使用非交互 JSONL 模式，并关闭隐式 Extension、Skill、Prompt Template 和 Context File 发现；平台冻结的 Skill 仍通过公共 Instruction seam 暴露。PI Agent 本身不内置 MCP，因此带 MCP Connector 的执行会 fail closed，直到平台提供并验证明确的 PI Extension 适配。
