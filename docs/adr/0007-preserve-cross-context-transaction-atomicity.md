---
status: accepted
---

# 保留跨上下文事务原子性

Coding Task Launch/Continue、Run Finish 和 Run Approval 在 Kratos 重构后继续使用单 PostgreSQL 原子事务，不改成最终一致。跨上下文 Biz Workflow 通过事务绑定的 Repository 协调写入，各 `data/<context>` Repository 只能修改所属上下文的数据；同一事务还必须保留幂等响应快照、Audit Event 和可选 Webhook Delivery，Wire 因此需要注入 transaction-scoped Repository Factory，而不是只装配绑定全局数据库连接的单例 Service。
