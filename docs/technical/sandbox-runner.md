# Sandbox Runner

状态：Linux + gVisor 生产边界

Runtime 执行使用 Docker CLI 参数数组创建 `runsc` Container。启动前必须验证：镜像是 RepoDigest、Runtime 为 `runsc`、UID/GID 非 root、Rootfs 只读、Capabilities 全部移除、`no-new-privileges`、CPU/内存/PID/tmpfs 限制有效、Credential Mount 只读、Workspace 是唯一可写业务挂载、附件在 Workspace 文件访问边界内使用独立只读挂载、Egress 使用明确网络。

Session 与 Run Conversation 的每个 Execution Stage 按 Owner、资源 ID、冻结 Expert 身份（无 Expert 时使用匿名 Stage）、Runtime Engine 和镜像 Digest 使用稳定且彼此隔离的 Warm Container。一次 Stage 中的版本探测与正式调用通过 `docker exec` 进入同一 Container；Expert Team 的不同成员不共享执行上下文，只按顺序挂载同一轮 Workflow 临时 Workspace。Stage 结束后立即停止 Container 以终止所有子进程并清理单次 Secret，但保留不可变 Container 定义。30 分钟内同一 Stage 身份再次使用时直接启动，连续空闲 30 分钟后由 Worker Reaper 销毁。复用前必须校验 Container 配置指纹，漂移时 fail closed。

Workspace Run 在临时副本工作；仅成功执行才安全合并到 Workflow 的持久 Workspace。路径穿越、符号链接、特殊文件和超过 1 GiB 配额均 fail closed。失败或取消不会污染持久 Workspace。

模型、MCP、Workflow 环境与 Git SSH Secret 只存在于单次任务的 Credential 目录，权限为 0700/0600；每次 Container 停止后立即幂等清理，不等待 Warm Container 到期。Secret 不进入镜像层、Docker 参数、对象 Key、日志或结果。

生产可访问公网，但容器仍通过受管 public-egress Network 和固定 Resolver。Runtime 默认不能回连 Worker；若同机模型网关必须经平台 TLS 入口访问，只能通过 `AGENT_ALLOWED_HOST_HTTPS_IPS` 显式允许公开 IPv4 的 TCP 443，其他 Worker 端口继续拒绝。MCP 测试也在相同隔离边界中执行；不允许 API 进程直接运行用户配置的包。
