---
status: accepted
---

# 分离认证与授权

Kratos HTTP Middleware 解析 Bearer Token、验证身份、加载 Principal 并写入 Context；Identity Biz 继续拥有 Organization、Team、Role Grant 和资源范围授权规则。缺少 TokenVerifier、Identity Repository 或 Access Provider 时 Wire 必须让 API App 构建失败，非法 Token 与认证基础设施不可用必须映射为不同公开结果；Health 与 Readiness 使用明确的 Public Operation Allowlist。未文档化的 `X-Team-ID` 不进入新 Proto 契约，Team Scope 继续来自显式请求字段或受控资源关系。
