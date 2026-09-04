---
status: accepted
---

# Use a Credit Ledger with post-execution settlement

Credits form an independent bounded context rather than belonging to Account or Workspace. Credits owns the immutable Credit Ledger, balance and daily-usage projections, Daily Credit Allocations, Redemption Codes, Model Credit Rate revisions, and Credit Adjustments; Account supplies User identity, while Workspace uses Credits application ports for model-invocation admission and settlement. This boundary keeps authentication free of usage accounting and prevents Session or Run repositories from directly mutating balances.

Every actual model invocation freezes a Model Credit Rate and settles after execution because the input and output Token delta is not known reliably in advance. A positive balance admits one invocation, its actual consumption may produce a negative balance, and subsequent invocations remain blocked until daily or redeemed Credits cover it. Model invocations are serialized per User because the available Runtime contracts cannot enforce a common maximum Token reservation; this deliberately trades User-level execution concurrency for a meaningful no-Credits execution boundary.

Each daily allocation, redemption, adjustment, and consumption appends one immutable ledger entry while transactionally updating balance projections. Every invocation's terminal Execution Stage state and consumption commit in the same PostgreSQL Repository transaction; a single-stage execution or the final Expert Team stage also commits the Assistant Message or Run terminal state there. `(execution identity, stage position)` provides exactly-once settlement across Worker retries. Expert Team stages settle independently, so completed stages remain charged when a later stage cannot start, fails, or is cancelled, while unstarted stages remain uncharged.

Runtime-reported Token counts may drive this anti-abuse accounting without claiming verified Usage Capability support. Each Adapter must normalize cumulative native-session data to the current invocation delta; when no Token counts exist, the frozen rate's fixed fallback applies and the UI labels the charge as estimated. This is an entitlement mechanism for platform execution, not Provider billing or currency accounting.
