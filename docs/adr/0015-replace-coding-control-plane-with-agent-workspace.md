---
status: accepted
---

# 用 Agent Workspace 取代 Coding Agent 控制面

产品从围绕 Organization、Team、Agent Draft/Release、Repository Binding、Coding Task 和 Operations 的企业 Coding Agent 控制面，转为按 User 私有隔离的 Sessions、Workflows、Experts、Extensions 与 Personal Settings。该决策选择删除旧业务 API、Schema、页面、兼容层和历史产品文档，并重置无真实 Run 的业务数据，而不是维护两套模型或把旧系统隐藏在新 UI 后；保留 OIDC、Runtime Adapter、Run/Event/SSE、Worker、Sandbox、Secret、Object Storage、PostgreSQL、Vue/Go 和部署底座。接受后，新的 Protobuf/API、User 所有权和 Workflow 编排直接取代旧 `/v1` 兼容、Organization/Team Scope 与 Coding Task/Approval 跨上下文设计。
