# 05 — Apply Credit admission and settlement to Workflow Runs

**What to build:** Manual, API, and Scheduled Workflow Runs use the same frozen Model Credit Rate and Credit Balance as Sessions, expose per-turn consumption, and provide entry-point-appropriate insufficient-credit behavior.

**Blocked by:** 04 — Configure and freeze Model Credit Rates.

**Status:** ready-for-agent

- [ ] Resolve and freeze the first Workflow Run's ordered Execution Stage rate data with the existing Workflow Snapshot and reuse the frozen snapshot for follow-up turns.
- [ ] Apply positive-balance admission immediately before every single-stage Workflow model invocation.
- [ ] Carry current-invocation Runtime Usage or estimated fallback through Worker execution into Workflow settlement.
- [ ] Settle the terminal Execution Stage, Credit Ledger consumption, balance projections, Run terminal state, and terminal Run Event in one database transaction.
- [ ] Make Workflow settlement idempotent for the Run execution identity and Stage position.
- [ ] Display each terminal Workflow Run turn's collapsed Credit Consumption in the existing conversation presentation.
- [ ] Let a User expand the Run consumption summary to see safe Token, multiplier, final Credit, and estimated-fallback details.
- [ ] Return Workflow Run Credit Consumption and safe detail through the authenticated status API only after settlement commits.
- [ ] Include final Credit Consumption in terminal SSE state without publishing a pre-commit or duplicate charge.
- [ ] Reject an insufficient-credit manual Workflow request without creating a Run.
- [ ] Reject an insufficient-credit Workflow API request with public code insufficient_credits and HTTP 429.
- [ ] Include only the caller's current balance and next Daily Credit Allocation time in the insufficient-credit API response.
- [ ] Create an explicit terminal failed, uncharged Run when a Scheduled Trigger is due without positive Credits, so the missed execution remains auditable.
- [ ] Recheck a queued or follow-up Run immediately before model invocation and surface an insufficient-credit terminal result if earlier User work consumed the balance after submission.
- [ ] Preserve Workflow owner isolation, API credential scope, Idempotency-Key behavior, serialized Workspace mutation, terminal Event rules, and success-only Workspace merge.
- [ ] Add application, repository, HTTP, SSE, API credential, scheduled-trigger, and Workflow page tests for success, exact and default rates, fallback, insufficient admission, queued recheck, idempotent replay, and visible consumption.
- [ ] Verify manual, API, scheduled, first-turn, follow-up, rerun, deleted-record, Chinese, English, desktop, and mobile behavior affected by this slice.
- [ ] Run targeted backend and frontend tests, full backend test and build gates for the shared Worker contract, web typecheck, web production build, and diff validation.
