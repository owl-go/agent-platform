# Sandbox Runner

状态：Linux + gVisor 生产边界；CLI Connector bundle 的二次校验与物化、公共 broker 协议、Runtime 客户端、专用短生命周期 Connector Container、只读 broker socket、Runtime/broker 绑定、窄协议宿主 Egress Controller，以及存活 Worker 内持久化 User Action Wait 与一次性命令绑定已实现；模型 Runtime 不挂载 CLI bundle，Worker 重启恢复、普通执行 deadline 暂停和 Linux + gVisor 端到端证据尚未完成

Runtime 执行使用 Docker CLI 参数数组创建 `runsc` Container。启动前必须验证：镜像是 RepoDigest、Runtime 为 `runsc`、UID/GID 非 root、Rootfs 只读、Capabilities 全部移除、`no-new-privileges`、CPU/内存/PID/tmpfs 限制有效、Credential Mount 与 CLI Connector bundle Mount 只读、Workspace 是唯一可写业务挂载、附件在 Workspace 文件访问边界内使用独立只读挂载、Egress 使用明确网络。Connector bundle 的 SHA-256 与该 Runtime RepoDigest 的可用性组合必须匹配冻结快照。

Session 与 Run Conversation 的每个 Execution Stage 按 Owner、资源 ID、冻结 Team Member 身份（无成员时使用 Expert 或匿名 Stage）、Runtime Engine 和镜像 Digest 使用稳定且彼此隔离的 Warm Container。一次 Stage 中的版本探测与正式调用通过 `docker exec` 进入同一 Container；Expert Team 的不同成员不共享执行上下文，即使引用同一 Expert，也只按顺序挂载同一轮 Workflow 临时 Workspace。Stage 结束后立即停止 Container 以终止所有子进程并清理单次 Secret，但保留不可变 Container 定义。30 分钟内同一 Stage 身份再次使用时直接启动，连续空闲 30 分钟后由 Worker Reaper 销毁。复用前必须校验 Container 配置指纹，漂移时 fail closed。

进入 `waiting_for_user` 时不结束 Stage：保留 Container、临时 Workspace、Workflow 串行锁与 Credit lease，暂停普通执行超时，仅运行最长十五分钟的 User Action Wait deadline，并持续接受取消。Worker 重启后的 Reconcile 只能从已持久化的批准状态恢复；过期、拒绝、Definition disabled 或 Authorization revoked 均关闭尚未启动的命令。批准后由公共 CLI Connector Wrapper 在真实进程启动前再次校验不可变 argv 摘要和全部当前授权。

Workspace Run 在临时副本工作；挂载前整棵临时文件树必须归固定 Runtime 用户 `65532:65532` 独占，目录为 `0700`、普通文件为 `0600`，已有所有者执行位的文件保持可执行。Runtime 不依赖宿主机用户手工修改权限。仅成功执行才安全合并到 Workflow 的持久 Workspace。路径穿越、符号链接、特殊文件和超过 1 GiB 配额均 fail closed。失败或取消不会污染持久 Workspace。

模型、MCP、Workflow 环境与 Git SSH Secret 只存在于单次任务的 Credential 目录，权限为 0700/0600；每次 Container 停止后立即幂等清理，不等待 Warm Container 到期。Secret 不进入镜像层、Docker 参数、对象 Key、日志或结果。

生产可访问公网，但容器仍通过受管 public-egress Network 和固定 Resolver。Runtime 默认不能回连 Worker；若同机模型网关必须经平台 TLS 入口访问，只能通过 `AGENT_ALLOWED_HOST_HTTPS_IPS` 显式允许公开 IPv4 的 TCP 443，其他 Worker 端口继续拒绝。MCP 测试也在相同隔离边界中执行；CLI Connector 的 Egress 是其结构化 capability policy 与网络策略的交集。不允许 API 进程直接运行用户配置的包或 CLI bundle。

CLI capability Egress Gate 必须先确认 Connector Container 位于显式配置的私有 IPv4 子网，再将每个审核域名解析为公网 IPv4；任何私网、回环、链路本地、保留或 IPv6 结果均拒绝整次命令。命令期间临时 `DOCKER-USER` chain 只允许访问固定 Resolver 的 TCP/UDP 53 与解析结果的 TCP 443，其余流量拒绝；完成、失败或取消均在独立清理 deadline 内移除 jump 和 chain。策略安装前不得启动 CLI。

Worker 配置必须显式给出 `sandbox.egress_subnet`、`sandbox.resolver_addresses` 和 `sandbox.egress_controller_socket`，并与 `configure-public-egress.sh` 创建的网络及 Resolver 文件一致；缺失、私网 Resolver 或配置漂移均拒绝 CLI Connector 执行。容器化 Worker 不直接修改宿主网络命名空间：专用 Egress Controller 以宿主 Network Namespace、`NET_ADMIN`、只读 Rootfs 和 Unix Socket 运行。Worker 只能通过 acquire/release 协议提交 Container 名称和审核域名，不能提交原始 `iptables` 参数；连接中断按 Container 可能仍活跃处理并保留限制性规则。

每条 CLI 命令使用一个专用 Connector Container：在受管 Egress Network 上启动固定镜像内的 `/bin/sleep 900` 等待进程，以取得 Docker 分配的真实 IP；此时不运行 Connector 代码、不注入凭证。安装 capability policy 后，才通过 `docker exec` 注入单次命令环境并调用公共 Entrypoint 与审核后的 CLI。凭证值不进入 Docker 参数或 Container 创建配置。Docker 的 `none` 网络不能直接与其他网络并存，未启动的 Container 也没有可供策略使用的 IP，因此不能以“先 none、再 connect”的方式准备策略。Container 使用与当前 Stage 相同的固定 Runtime 镜像 RepoDigest 和 `runsc`，只读 Rootfs、移除全部 Capability、启用 `no-new-privileges` 和资源限制；仅挂载当前 Connector bundle（只读）、Workspace（可写）及固定 Resolver。模型 Runtime 不挂载 bundle。命令结束、失败或超时后必须在 Egress Gate 撤销规则前强制删除 Connector Container；无法证明 Container 已停止时保留限制性规则并返回失败，不能退化为放开网络。bundle、凭证值和 Egress host 均由 broker 的冻结服务端配置决定。

CLI broker 的真实 socket 保留在当前 Stage 的 `scratch/broker/cli-broker.sock`。Worker 通过 `/tmp` 下随机且仅自身可访问的短目录别名完成 Unix socket 绑定，绑定后立即移除别名，不改变进程工作目录；关闭时幂等清理真实 socket。模型 Runtime 的冷启动与 Warm Container 均保留原始 broker 目录的只读覆盖，并额外只读挂载到 `/run/agent-cli`，`AGENT_PLATFORM_CLI_SOCKET` 固定指向 `/run/agent-cli/cli-broker.sock`，避免较长 Workspace 根路径超过 Unix socket 地址限制。该容器内目标路径进入 Warm Container 配置指纹；旧配置不得被静默复用。

部署 CLI broker 时，Docker 的 `runsc` 配置必须使用 `--host-uds=open` 以连接已挂载的宿主 Unix socket；不使用允许创建宿主 socket 的 `create` 或 `all`。模型 Runtime 仅显式挂载本 Stage 的 broker socket 目录，不挂载 Docker socket 或其他宿主控制 socket。指定 `CLI_BROKER_TEST_RUNTIME_IMAGE=<RepoDigest>` 后，在 Linux Worker 执行 `go -C backend test ./internal/cliconnector -run TestUnixBrokerRuntimeClientLongPath -v` 验证镜像内 `agent-cli`、长路径、冷启动及 Warm Container 复用；未提供环境变量时此集成测试会 Skip，不代表通过。

额外指定 `CLI_CONNECTOR_TEST_NETWORK=<受管测试网络>` 后，`TestDockerConnectorWaitsForPolicyBeforeCommand` 在真实 Docker + `runsc` 上验证 IP 分配、等待进程不含凭证、策略回调前不执行命令、拒绝时不执行命令，以及成功和拒绝后的 Container 清理。该测试使用记录式 Gate，不替代真实出网规则的端到端验收。
