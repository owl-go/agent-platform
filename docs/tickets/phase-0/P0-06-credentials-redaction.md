# P0-06 实现临时凭证与 Secret Redaction

状态：代码路径已完成，等待 P0-03 gVisor 隔离验收

## 目标

向 Runtime 提供模型 Key、Git SSH Key 和构建凭证，同时证明它们不会进入持久数据。

## 工作

- 定义 EnvironmentRef 与临时凭证目录。
- 以最小文件权限或环境变量注入所选 Runtime 凭证。
- 实现 stdout/stderr、Event、Diff、Artifact 和 Snapshot 脱敏。
- Run 结束和 Container 销毁时清理凭证。

## 验收

- 测试 Secret 出现在命令、错误、文件和二进制输出中的场景。
- 所有持久结果只包含脱敏值。
- 非选中 Runtime、其他 Run 和宿主进程无法读取凭证。
- 清理操作幂等。

## 依赖

P0-01；与 P0-02、P0-03 并行协作。

## 当前证据

- `internal/credentials.Materializer` 按 EnvironmentRef 创建 0700 临时目录，以环境变量或 0600 文件注入本次 Run 选中的凭据。
- 凭据文件拒绝绝对路径和 `..` 路径穿越；创建失败会删除部分目录，Run 清理可重复执行。
- `Environment.Redactor` 只包含当前凭据环境中的值；测试证明不同 Run 的 Redactor 不会混入另一 Run 的 Secret。
- `Redactor.Bytes` 覆盖 Event、命令、错误和小型 Diff；`Redactor.Reader` 对 stdout/stderr、Artifact 与 Snapshot 做有界流式替换，并覆盖跨读取边界和二进制 Secret。
- `agentruntime.NewRedactingEventSink` 与 `processharness.NewRedactingSink` 把脱敏放在持久化 seam，而不是依赖各 Runtime 自觉处理。
- 尚未关闭的验收项是“其他 Run、非选中 Runtime 和宿主进程无法读取凭据”；该项需要 P0-03 的独立 gVisor Container、挂载与 UID 隔离共同证明。
