---
status: accepted
---

# 使用中立 Biz Workflow 协调跨上下文事务

Execution、Collaboration、Approval 等上下文 Biz Package 不直接互相导入；Coding Task Launch、Run Completion 和 Run Approval 由 `internal/biz/workflow` 中的独立用例协调，以避免 Go Import Cycle 和上下文所有权泄漏。Workflow 只依赖各上下文公开 Command Port 与 Transaction Manager Port，不能接收 GORM 或其他基础设施类型。

`internal/data/transaction` 在单 PostgreSQL 事务内构造 transaction-scoped Repository 和 Command Service，并调用 Workflow Callback；各 `data/<context>` Repository 只能修改所属上下文的数据。项目不建立包含数据库、Object Store、Secret、Runtime 和 Docker 的全能 `Data` Service Locator，Service 层也不得直接操作 GORM、状态机或跨上下文事务。
