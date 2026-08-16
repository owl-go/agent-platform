---
status: accepted
---

# 使用 Protobuf 作为 API 契约源

Agent Platform 后端采用 Proto-first API：`backend/api` 中的 Protobuf 是服务端接口的权威契约，并生成 Kratos HTTP/gRPC 代码和 OpenAPI；前端从生成的 OpenAPI 获得 TypeScript 类型。Proto 按限界上下文分为 Platform、Execution、Run Approval、Artifact、Audit、Runtime Catalog、Model Catalog、Source Control、Agent Lifecycle 和 Collaboration Service，禁止建立一个覆盖全平台的巨型 Service。

迁移必须保持现有 `/v1` HTTP Path、Method、JSON、Header、状态码和安全错误边界兼容，并通过旧 OpenAPI Fixture 与生成 OpenAPI 的契约比较门禁验证。初期只部署 HTTP Transport，不因生成 gRPC 代码而默认开放 gRPC 监听端口；禁止同时手工维护另一份权威 OpenAPI 契约。

已知结构的动态 JSON 必须收敛为明确 Proto Message，真正开放的 Approval、Audit 或 Event 数据使用 `google.protobuf.Struct`/`Value`；禁止用 Proto `bytes` 表示 JSON，幂等响应快照继续以不透明原始 JSON 存储和重放。Proto 生成使用 Buf，并固定 Buf、Kratos、OpenAPI、Validation、Go 和 TypeScript 插件版本；生成的 Go、Kratos HTTP/gRPC、OpenAPI 与前端 TypeScript Source 全部提交，CI 执行 Lint、Breaking Change、重新生成和零 Diff 检查，不安装 `@latest` 工具。
