---
status: accepted
---

# MVP 使用 PostgreSQL 编排 Run

MVP 使用 PostgreSQL 队列、Worker Lease 和 Reconciler，不引入 Temporal。在 10 个并发 Run 的初始目标下，这能保持部署和 Go 实现精简，同时支持等待、中断、恢复和安全的基础设施重试；代价是平台必须自行维护 Run 状态机，并通过测试证明 Lease 过期、状态收敛和幂等行为正确。

## 影响

Run 状态转换和重试资格必须只有一个权威实现。Sandbox 创建和外部副作用必须使用稳定 Idempotency Key，Reconciler 必须收敛孤儿 Lease 与 Container。若未来出现复杂 Timer、Child Run 编排、长期外部 Signal，或运维要求超出当前状态机能力，应重新评估 Temporal。
