---
status: accepted
---

# 一次性切换全部 HTTP API 到 Kratos

Agent Platform 不存在需要兼容运行的旧服务，全部 HTTP API 在一个重构分支内一次性切换到 Kratos：普通 Route 使用 Proto 生成 Handler，Run Event SSE 使用挂载在同一 Server 的 Gin Handler，旧 `internal/httpapi` 与旧 OpenAPI 权威源在同一批变更中删除。不引入 Legacy `NotFoundHandler`、双端口、Feature Flag、双写或代理分流；回退单位是整个提交或部署镜像，而不是请求级旧实现。

一次性切换仍须在合入前通过生成、后端、契约、前端、部署及适用的 Conformance 门禁。迁移不同时重命名现有数据库表列或重写历史 Migration，完整 Production Conformance 通过前不能宣布框架重构完成。
