# 02 — Manage Daily Credit Allocation and inspect the Credit Ledger

**What to build:** A User can open the Credit Balance row to understand today's expiring allowance, persistent balance, consumption, next reset, and complete ledger, while an Administrator can configure the User's next Daily Credit Allocation and make an auditable Credit Adjustment without seeing private execution details.

**Blocked by:** 01 — Complete the single-stage Session Credit loop.

**Status:** ready-for-agent

- [ ] Expand the Credit account projection to distinguish current Daily Credit remainder, persistent redeemed or adjusted balance, today's consumption, total balance, effective Credit Day, and next allocation time.
- [ ] Define the Credit Day by midnight in the User's effective Personal Settings time zone rather than UTC or a rolling 24-hour window.
- [ ] Materialize the current day's Daily Credit Allocation transactionally on the first balance read or model-invocation admission after the boundary.
- [ ] Enforce one allocation entry per User and Credit Day so concurrent reads or admissions cannot restore Credits twice.
- [ ] Expire unused Daily Credits at the Credit Day boundary without carrying them forward.
- [ ] Consume remaining Daily Credits before persistent Credits.
- [ ] Pin a model invocation to the Credit Day in effect when it starts, including when settlement occurs after midnight.
- [ ] Carry negative persistent balance across Credit Days and offset it with the next Daily Credit Allocation rather than forgiving it.
- [ ] Keep the current Credit Day's time-zone boundary stable when the User changes Personal Settings, and apply the new time zone beginning with the next Credit Day.
- [ ] Add Administrator account metadata for total balance, today's consumption, current allocation, pending next-day allocation, and effective date.
- [ ] Allow an Administrator to set a non-negative per-User Daily Credit Allocation that takes effect on the next Credit Day.
- [ ] Treat a configured allocation of zero as disabling future daily allocation without disabling the User account.
- [ ] Allow an Administrator, including the bootstrap Administrator for their own account, to add or subtract persistent Credits only through a non-empty reasoned Credit Adjustment.
- [ ] Append every allocation and adjustment to the immutable Credit Ledger and update projections in the same transaction; never directly overwrite a balance.
- [ ] Provide an owner-scoped, paginated, newest-first Credit Ledger containing safe entry type, signed amount, resulting balance, and time.
- [ ] Let the Administrator inspect allocation and adjustment records but not execution-level ledger entries, Session or Workflow identity, Provider Model, Token counts, prompt, or result.
- [ ] Open a localized Credit panel from the avatar balance row with total balance, today's remainder, persistent balance, today's consumption, next allocation time, and paginated ledger.
- [ ] Extend User Management with balance, today-consumed, and Daily Credit Allocation columns plus a reasoned adjustment action that remains usable on desktop and mobile.
- [ ] Do not introduce low-balance Toast, email, webhook, or configurable threshold notifications.
- [ ] Add deterministic time-zone, daylight-boundary, lazy-materialization, concurrent-materialization, cross-midnight, debit-order, negative-carry, pending-allocation, authorization, pagination, and responsive UI tests.
- [ ] Run targeted backend and frontend tests, backend build, web typecheck, web production build, and diff validation for this slice.
