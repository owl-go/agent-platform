---
status: accepted
---

# 显式本地 runc 执行模式

2026-10-18 用户明确要求在个人 macOS Docker Desktop 上运行会话，同时保留生产 gVisor 隔离。新增显式 `sandbox.profile: local`，仅该模式接受 `runc` 和本地不可变 `sha256:<Image ID>`；省略 profile 或选择 `production` 时仍要求 `runsc` 与 Registry RepoDigest，不因运行时缺失自动降级。本决策仅取代 ADR-0019 中本地开发也必须使用 gVisor 的隐含要求，生产要求不变。

本地 Worker 在 Docker 内运行，执行容器继续使用非 root、只读 Rootfs、资源限制、只读凭证挂载及相同的脱敏、取消和清理契约。API、Web 和管理端口仅发布到环回地址；Runtime 使用专用 Docker bridge 及 Docker DNS，该网络不提供生产 iptables 公网隔离，适用于单用户可信本机。CLI Connector 的 Egress Gate、隔离 Builder 和消息渠道保持关闭，不能把生产的 bundle/Runtime Conformance 当成本地证据。固定 CLI 的版本检查仅证明镜像就绪，真实供应商/引擎组合和 Production Conformance 分别验收。
