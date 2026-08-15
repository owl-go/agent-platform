# P0-01 固定 Runtime Adapter Contract

状态：已完成（2026-08-15）

## 目标

把 `internal/agentruntime` 变成 Worker 与四种 Runtime 之间唯一的公开 seam。

## 工作

- 完成 Request、Result、Event、Capability 和错误分类类型。
- 明确取消、事件顺序、Checkpoint 和 Usage 语义。
- 编写 Fake Adapter 和 Interface 单元测试。
- 让 `docs/technical/runtime-adapter.md` 与 Go 类型保持一致。

## 验收

- Fake Adapter 可模拟成功、失败、流式、取消和 EventSink 失败。
- 测试证明 Sequence 单调、终态唯一、取消后无新事件。
- Worker 测试只依赖 Interface，不引用任何 CLI 包。

## 依赖

无。

## 完成证据

- `internal/agentruntime` 定义 Adapter、Request、Result、Capability、Event 和 Error Code。
- ContractSink 强制 Run ID、JSON Payload、连续 Sequence、唯一终态和取消后的事件拒绝。
- `runtimefake.Adapter` 可注入成功、失败、流式、取消和 EventSink 失败行为。
- `internal/runworker.Runner` 只依赖 Runtime Adapter Interface，并在调用 Adapter 前校验冻结输入。
- `go test -race ./...` 覆盖 Interface 与调用方 seam。
