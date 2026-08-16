---
status: accepted
---

# 资源查询必须同时约束授权范围

Artifact Download 与 Run Approval Read 在 Kratos 迁移前改为以 Resource ID、Organization 以及授权 Team/Run Scope 联合查询，避免先按全局 ID 查询后再授权造成 `404`/`403` 差异泄露资源是否存在。该安全修复必须作为独立变更先补测试并更新 HTTP 兼容 Fixture，再成为 Kratos 迁移基线，不能隐藏在目录移动或生成代码的大型 Diff 中。
