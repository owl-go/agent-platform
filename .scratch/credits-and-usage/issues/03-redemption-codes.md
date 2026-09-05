# 03 — Issue and redeem single-use Redemption Codes

**What to build:** An Administrator can generate and distribute batches of secure single-use Redemption Codes, and a User can redeem one from the Credit panel to receive persistent Credits immediately, with generic failure messages and an immutable audit trail.

**Blocked by:** 02 — Manage Daily Credit Allocation and inspect the Credit Ledger.

**Status:** ready-for-agent

- [ ] Add immutable, additive persistence for Redemption Code batches, non-secret code identifiers, one-way code verifiers, fixed Credit value, optional expiry, lifecycle state, and successful redemption metadata.
- [ ] Generate cryptographically random bearer codes; never accept Administrator-supplied plaintext code values.
- [ ] Let an Administrator create a batch containing between one and one hundred codes that share one positive two-decimal Credit value and optional expiry.
- [ ] Return generated plaintext only in the successful creation response and do not make it recoverable through later reads.
- [ ] Provide localized copy and client-generated CSV download actions for the one-time batch response.
- [ ] Let an Administrator list safe batch and code status metadata and void an unused code without revealing plaintext.
- [ ] Add a Redemption Code input and submit action to the User's Credit panel.
- [ ] On successful redemption, atomically mark the code redeemed, record the User and redemption time, append the persistent Credit grant, and update balance projections.
- [ ] Enforce global single-use behavior so simultaneous redemption attempts produce exactly one successful grant.
- [ ] Preserve redeemed Credits across Credit Days and consume them only after the current Daily Credit remainder.
- [ ] Apply a redemption to negative persistent balance before making total balance positive.
- [ ] Return the updated Credit summary with a successful redemption so the Credit panel and avatar row refresh immediately.
- [ ] Return one public unavailable result for malformed, unknown, expired, void, and already-redeemed codes without identifying a previous redeemer or lifecycle state.
- [ ] Exclude Redemption Code plaintext from database columns, application logs, HTTP logs, errors, later API responses, SSE, browser persistence, Workspace, Artifacts, and telemetry.
- [ ] Add a Redemption Codes tab to User Management without adding a primary navigation item.
- [ ] Keep all User redemption reads and writes owner-scoped and all creation, listing, and void operations Administrator-only.
- [ ] Add contract and PostgreSQL concurrency tests for batch bounds, verifier matching, expiry, voiding, global single use, ledger idempotency, rollback, negative-balance repayment, and generic failures.
- [ ] Add frontend tests for one-time display, copy, CSV generation, void action, redemption success, updated balance, unavailable errors, loading, keyboard access, Chinese, English, desktop, and mobile behavior.
- [ ] Add secret canary tests proving plaintext codes do not escape the one-time creation response.
- [ ] Run targeted backend and frontend tests, backend build, web typecheck, web production build, and diff validation for this slice.
