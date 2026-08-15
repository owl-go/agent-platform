# P0-03 验证 Docker + gVisor Sandbox Runner

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

