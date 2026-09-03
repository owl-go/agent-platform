---
status: accepted
---

# 以平台摘要保证连续性并按能力使用原生 Resume

Session 的正确性由消息、Rolling Summary 和最近上下文保证；只有通过指定镜像 Conformance 的 Runtime Engine 才使用 Native Session Resume。Claude Code/Codex 可在能力验证后 Resume，Hermes/OpenClaw/PI Agent 暂走摘要；切换 Runtime Engine 名称会以摘要新建 Native Session，而相同名称即使 CLI、Adapter 或镜像版本变化仍先尝试旧 Checkpoint，这是为保持 User 认知简单而接受的兼容风险。只有明确分类为“执行任何动作前 Checkpoint 已失效”的错误可自动以摘要降级，其他错误必须显式失败以防重复副作用；Existing Session 冻结最初的 Expert/MCP/Skill Snapshot，每次发送消息再冻结独立 Response Snapshot。同一 Runtime Engine 下切换 Provider Model 时继续交给 Runtime Resume 处理；模型或协议拒绝必须显式失败，不改变平台历史。
