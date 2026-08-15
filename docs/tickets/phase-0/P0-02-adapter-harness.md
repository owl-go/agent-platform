# P0-02 实现 Adapter Process Harness

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

