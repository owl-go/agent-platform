---
status: accepted
---

# Run Event SSE 使用 Gin Handler

`GET /v1/runs/{run_id}/events` 使用 Gin 实现自定义 SSE Handler，以保留 Cursor Replay、`Last-Event-ID`、Heartbeat、事件序列和代理禁用缓冲等现有语义。Kratos v3 原生 Server Streaming SSE 只发送固定的 `message`/`error` Event 和 Data，不能直接表达既有的 `id: <sequence>`、动态 Run Event Type 与注释 Heartbeat，因此本端点有意不使用生成的 HTTP Streaming Handler。

Gin 只负责这一特殊 HTTP Transport，业务认证、授权与 Event Replay 仍调用共享 Biz 接口；其他 API 继续使用 Proto 生成的 Kratos Handler，不能扩张出第二套通用 HTTP 应用层。

Gin 作为标准 `http.Handler` 挂载到同一个 Kratos HTTP Server 和监听端口，只注册 `GET /v1/runs/:run_id/events`，不使用 `gin.Default` 或独立 Server。Kratos Server 不设置全局请求总时长；普通 Unary API 由 Middleware 按 Operation 设置 Deadline，SSE 保留客户端断开与 Server Shutdown 的取消信号，并只为单次 Event Replay 查询创建短子超时。认证、Tracing、Access Log、Recovery 和安全 Header 使用外层共享 Filter，Gin Handler 在发送 `200` 前调用 Execution Biz 完成 Run 级授权。

Execution Proto 单独定义 `RunEventStreamService` Server Streaming RPC 和 HTTP Annotation，以继续生成契约与 OpenAPI，但运行时不注册其生成 HTTP Handler，而由 Gin 注册实际 SSE Route；普通 `ExecutionService` 仍使用 Kratos 生成 Handler。这样不会依赖重复路由的注册顺序，将来仍可独立开放生成的 gRPC Streaming Service。
