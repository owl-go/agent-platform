# Sandbox Runner 技术规格

状态：实现完成，等待 Linux + gVisor Production Conformance

## Seam

`internal/sandbox.Runner` 是 Worker 与容器实现之间唯一的接口，提供 Create、Start、Stop、Destroy、Inspect 和 Reconcile。Runtime Adapter 不调用 Docker，也不持有 Docker Socket。

当前 `DockerRunner` 通过参数数组调用 Docker CLI，不经过 Shell。每个受管 Container 使用 `agent-platform.managed=true` 和 `agent-platform.run-id` 标签；Reconciler 只处理该标签命名空间，并只删除超过保护窗口且不属于活跃 Run 的 Container。

## Fail-closed

Create 每次读取 Docker Runtime 列表，未发现配置名 `runsc`、Docker 不可用或响应无法解析时都拒绝创建。镜像必须使用 `repository@sha256:<digest>`，Tag 不可作为执行输入。

Start 前重新 Inspect，并核对：

- Runtime 为 `runsc`；
- UID/GID 非 root；
- Rootfs 只读；
- `CAP_DROP=ALL` 与 `no-new-privileges`；
- CPU、内存与 PID 限制非零；
- `/tmp` 为带 size、noexec、nosuid、nodev 的 tmpfs；
- 唯一可写持久挂载是 `/workspace` Docker Volume；
- 唯一凭据挂载是配置根目录下的只读 Bind Mount；
- Network 为 `none` 或预配置的 public-egress Network。

任何漂移返回 `ErrIsolationDrift`，不会 Start。

## Credential Mount

`credentials.Materializer.Owner` 必须配置为与 Sandbox 相同的 UID/GID。Worker 在挂载前把 0700 临时目录和 0600 文件 chown 给该身份，Container 只读挂载到 `/run/agent-credentials`。另一个 Run 的目录不会被挂载。Worker 必须具备创建、chown 和清理该专用根目录的权限；不要授予普通宿主用户访问权限。

Secret 不通过 Docker 命令参数或镜像层传递。Runtime Entrypoint 从只读凭据文件读取并只在 Runtime 进程环境中导出；Container Destroy 后 Materializer 幂等清理宿主临时目录。

## Egress

`deploy/sandbox/configure-public-egress.sh` 在 Linux Worker 创建专用 IPv4 Bridge，并配置独立 iptables Chain：

- 允许已建立连接与其他 Docker 流量；
- 对该 Sandbox Subnet 拒绝 loopback、RFC1918、link-local/metadata、carrier-grade NAT、benchmark、documentation、multicast 和保留网段；
- 在 INPUT 链拒绝 Sandbox 主动连接宿主机；
- 允许其余公共 IPv4；
- 通过 `AGENT_EXTRA_DENY_CIDRS` 增加平台控制服务的公共地址。

脚本只刷新自己拥有的 Chain。若现有 Docker Network 的 Subnet 或 IPv6 属性与配置不符，脚本退出，不尝试放宽策略。

## Lifecycle

Stop 使用 Docker Grace Period；超时后 Docker 强杀 Container 主进程。Destroy 使用 force + volumes，并把 No Such Container 当作成功，因此可安全重试。Reconcile 的活跃 Run 集合必须来自数据库权威状态；保护窗口避免删除刚创建但事务尚未完成的 Container。

## Linux Conformance

macOS 只运行参数、状态机和 fail-closed 单元测试。Production 证据必须在专用 Linux Worker 上执行：

```bash
sudo deploy/sandbox/configure-public-egress.sh
SANDBOX_TEST_IMAGE='registry/runtime@sha256:...' scripts/conformance/sandbox-linux.sh
```

测试镜像必须包含 `sh`、`curl` 和 `id`。脚本验证真实 Runtime 是 runsc、非 root、只读 Rootfs、Docker Socket 不存在、凭据只读、Workspace 可写、公共 HTTPS 可达、metadata/RFC1918 不可达，以及 Stop 超时后的退出状态。

P0-08 还需记录宿主网关、平台控制地址、DNS rebinding、磁盘配额和 Worker Crash 后 Reconcile 的真实证据。
