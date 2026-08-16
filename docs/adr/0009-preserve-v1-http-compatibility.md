---
status: accepted
---

# 保留 v1 HTTP 兼容层

Kratos Proto-first 迁移继续保持现有 `/v1` 的 Path、Method、Header、JSON 类型与字段存在性、状态码和单字段 `{"error":"public_code"}` 错误 Body。Service Error Mapper 将 Biz Error 转为同时携带 HTTP Status、gRPC Code、Public Code 和内部 Cause 的 Platform Error；自定义 Kratos HTTP ErrorEncoder 只公开稳定 Public Code，gRPC 映射不能反向决定 HTTP 状态。

默认 ProtoJSON 无法保持现有 int64 Number、枚举字符串、null/omit 和动态 JSON 语义，因此 `/v1` 使用通用 ResponseEncoder 外壳和按 Proto Response 类型显式注册的 Legacy JSON Mapper，不使用全局反射猜测兼容规则。Request Decoder 在 Proto Binding 前限制 64 KiB、校验 JSON Content Type、拒绝未知字段和第二个 Document，并保留原始 Body 供幂等哈希；每个 Operation 使用 Golden Fixture 验证兼容，不能把默认 ProtoJSON 的变化伪装成无行为重构。
