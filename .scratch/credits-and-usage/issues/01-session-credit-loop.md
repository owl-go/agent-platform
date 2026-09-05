# 01 — Complete the single-stage Session Credit loop

**What to build:** A User with a positive Daily Credit Allocation can send a no-Expert Session message, have the current model invocation measured and settled, see the updated Credit Balance in the avatar menu, and see the response's Credit Consumption. This tracer bullet establishes the independent Credits boundary and the smallest complete schema-to-UI path without claiming that Workflow, Redemption Code, or Expert Team behavior is finished.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Introduce an independent Credits domain and application boundary that owns admission, settlement, Credit Ledger entries, and balance projections while depending on Account only for User identity.
- [ ] Add immutable, additive schema for a User Credit account, append-only Credit Ledger, daily allocation identity, and idempotent single-stage consumption without modifying an existing migration.
- [ ] Represent Credit amounts as integer hundredths and rate arithmetic as exact decimals; do not use floating point for persisted or domain Credit values.
- [ ] Give a newly created User the current Credit Day's default 600 Credits and zero persistent Credits.
- [ ] Admit a no-Expert Session model invocation only when the User's total Credit Balance is positive.
- [ ] Carry the Runtime's current-invocation input and output Token counts through execution into application settlement instead of discarding them.
- [ ] Ignore Runtime-reported provider currency cost when calculating Credits.
- [ ] Calculate one invocation as input Tokens divided by 10,000 times 1.00 plus output Tokens divided by 10,000 times 1.00, rounded half-up once to two decimals with a minimum non-zero charge of 0.01 Credits.
- [ ] Charge the default 10.00 Credit fallback when the Runtime reports no usable input or output Token counts, and mark the settlement as estimated.
- [ ] Freeze the resolved default Model Credit Rate values on the single Execution Stage before invocation begins.
- [ ] Settle the terminal Execution Stage, Credit Ledger consumption entry, Daily Credit remainder, persistent balance, and total balance projection in one database transaction.
- [ ] Make settlement idempotent for the Session response's execution identity and single Stage position.
- [ ] Allow the admitted invocation's actual settlement to make total balance negative and block a subsequent Session invocation while the balance is not positive.
- [ ] Extend the current-User API representation with the Credit summary required by the avatar balance row.
- [ ] Extend a terminal Assistant response with total Credit Consumption and the safe input/output Token and estimate fields required by its disclosure.
- [ ] Display a localized avatar-menu Credit Balance row matching the established compact account-menu hierarchy.
- [ ] Display a localized collapsed summary such as 共消耗 ✧ 79.05 beneath a terminal Assistant response, with an expandable default-rate or estimated-fallback breakdown.
- [ ] Preserve existing Session streaming, cancellation controls, Response Snapshot behavior, secret redaction, and owner isolation.
- [ ] Add deterministic Credits application, Runtime-to-application, HTTP contract, and Session page tests for positive admission, exact calculation, fallback, atomic settlement, idempotent replay, negative balance, and visible UI states.
- [ ] Run targeted backend and frontend tests, backend build, web typecheck, web production build, and diff validation for this slice.
