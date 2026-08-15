# P0-02 实现 Adapter Process Harness

状态：已完成（2026-08-15）

## 目标

用一个共享 Module 隐藏 CLI 进程组、stdout/stderr、输出限制、脱敏和终止复杂度。

## 工作

- 启动独立进程组并并发读取输出。
- 实现优雅终止和 Deadline 强杀。
- 实现输出大小限制、UTF-8 检查和 Artifact 转存。
- 提供 Fake CLI fixture 验证异常输出与孤儿子进程。

## 验收

- Context 取消后全部子进程在 Deadline 内退出。
- EventSink 失败会停止 CLI。
- 大输出、二进制输出和 Secret 不会污染标准事件。
- `go test -race ./...` 通过。

## 依赖

P0-01。

## 完成证据

- `internal/agentruntime/processharness` 使用真实子进程验证独立进程组、stdout/stderr 分流、退出码与取消语义。
- Context 取消先向进程组发送 SIGTERM，Grace Period 到期后发送 SIGKILL；测试覆盖孤儿子进程和忽略 SIGTERM 的进程。
- 总输出与单行采用硬上限，达到上限会立即终止进程组，而不是继续读取到内存。
- 输出先写临时 Spool，再以 `io.Reader` 交给 `OutputSink`；UTF-8 小输出可内联，大输出和二进制输出保持 Artifact 路径。
- `OutputObserver` 支持 Adapter 增量解析；Observer/Event 持久化失败会停止 CLI 并保留原始错误。
- `NewRedactingSink` 保证 stdout/stderr 在进入持久化边界前执行流式 Secret Redaction。
- `go test -race ./...`、`go vet ./...`、前端 typecheck/build 均通过。
