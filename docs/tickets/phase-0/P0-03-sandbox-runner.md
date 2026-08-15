# P0-03 验证 Docker + gVisor Sandbox Runner

状态：实现完成，等待 Linux + runsc Production Conformance

## 目标

在 Linux Worker 上证明平台可以用 `runsc` 安全创建、控制和销毁 Runtime Container。

## 工作

- 实现 Create、Start、Stop、Destroy 和 Inspect。
- 配置非 root、只读根、Workspace 挂载和资源限制。
- 实现公网 Egress 与私网、元数据、宿主机拒绝策略。
- 实现孤儿 Container Reconcile。

## 验收

- gVisor 不可用时 fail-closed。
- Container 无法访问 Docker Socket、宿主机、元数据和 RFC1918 私网。
- Stop 超时后强杀，Destroy 可重复调用。
- Worker 崩溃后 Reconciler 能清理孤儿 Container。

## 依赖

P0-01；需要 Linux + `runsc` 测试环境。

## 当前证据

- `internal/sandbox.Runner` 实现 Create、Start、Stop、Destroy、Inspect 和基于活跃 Run 集合的孤儿 Reconcile。
- Create 每次检查 Docker Runtime 列表，缺少 `runsc` 时 fail-closed；镜像只接受不可变 SHA-256 Digest。
- Create 固定非 root、只读 Rootfs、Drop ALL Capabilities、no-new-privileges、CPU/内存/PID、受限 tmpfs、Workspace Volume 和只读凭据目录。
- Start 前 Inspect 并复核 Runtime、用户、Rootfs、Capabilities、SecurityOpt、资源限制、tmpfs、挂载和 Egress Network，漂移时返回 `ErrIsolationDrift`。
- Stop 使用 Docker Grace Period 后强杀语义；Destroy 将不存在视为成功；单元测试证明 Reconciler 不删除活跃或保护窗口内的 Container。
- `deploy/sandbox/configure-public-egress.sh` 以可切换 iptables Chain 配置公网出口，阻断宿主、metadata、RFC1918 与保留网段；更新规则时不产生空策略窗口。
- `scripts/conformance/sandbox-linux.sh` 提供真实 runsc 验收入口；当前 macOS 开发机没有运行 Docker daemon，也没有 `runsc`，因此网络隔离、磁盘配额和崩溃后真实回收尚未产生证据。
- 完成 Linux 脚本、Workspace 配额验证与真实 Crash/Reconcile 前，本 Ticket 不标记“已完成”。
