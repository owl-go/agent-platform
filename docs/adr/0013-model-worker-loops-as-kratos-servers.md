---
status: accepted
---

# Worker 循环实现独立 Kratos Server 生命周期

Execution、Webhook、Retention 和 Reconcile 分别实现可独立测试的 Kratos `transport.Server` Adapter，由无业务逻辑的聚合函数根据严格配置构造 Worker App 的 Server 集合。每个 Server 的 `Start` 运行循环直至 Context 取消或不可恢复错误，`Stop` 触发停止并等待当前安全边界；可恢复的单次处理错误只记录并继续轮询。API 与 Worker 使用不同 Kratos App、Wire Injector 和 Server 集合，不把后台能力重新聚合成大型 `cmd/worker/main.go`。

Worker 另有只在 Control Network 暴露的 Management HTTP Server，提供 `/healthz`、`/readyz` 和 `/metrics`。Readiness 验证 Worker Server 已启动、PostgreSQL 可用且没有使所有处理循环失效的不可恢复错误，不在探针中执行 Docker、Runtime、Secret Store 或 Object Store 的深度有副作用检查；不可恢复错误必须终止 App。
