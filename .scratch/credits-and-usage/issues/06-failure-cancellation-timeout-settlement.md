# 06 — Settle failed, cancelled, and timed-out model invocations exactly once

**What to build:** A User is charged for actual Provider Model work that occurred before a failure, cancellation, or timeout, is not charged when failure occurs before model invocation, and can understand each terminal charge without retries or Worker recovery creating duplicates.

**Blocked by:** 05 — Apply Credit admission and settlement to Workflow Runs.

**Status:** ready-for-agent

- [ ] Distinguish model invocation not started, started with usable Token Usage, and started without usable Token Usage at the Runtime-to-Workspace boundary.
- [ ] Preserve final current-invocation Token Usage when a Runtime returns a failed, cancelled, or timed-out outcome.
- [ ] Charge reported current-invocation input and output Tokens for failed, cancelled, and timed-out stages when Provider Model invocation started.
- [ ] Charge the frozen missing-Usage fallback when Provider Model invocation started but no usable Token values are available.
- [ ] Create no Credit Consumption when validation, attachment materialization, credential setup, Runtime preparation, admission, or another failure occurs before Provider Model invocation.
- [ ] Treat Retry and rerun as new executions with new admission and independent settlement rather than reopening or mutating a prior ledger entry.
- [ ] Make terminal Stage settlement idempotent across Worker lease loss, completion retry, repository retry, and process recovery.
- [ ] Preserve exactly one terminal Stage state and, for a single-stage execution, exactly one terminal Assistant Message or Run state.
- [ ] Commit failure, cancellation, or timeout Stage state, consumption ledger entry, and projections in one transaction.
- [ ] Stop execution and leave no terminal publication if Credit settlement cannot commit; recovery must retry the same idempotent settlement.
- [ ] Keep cancellation after terminal settlement from publishing another event or changing the recorded charge.
- [ ] Display actual or estimated Credit Consumption on failed, cancelled, and timed-out Session responses and Workflow Run turns.
- [ ] Show no consumption summary when no Provider Model invocation began.
- [ ] Label fallback charges as estimated and never infer or display fabricated Token counts.
- [ ] Keep terminal errors free of Redemption Code plaintext, Provider secrets, prompts, private reasoning, and internal rate data.
- [ ] Add table-driven application and Worker tests covering each failure phase, real zero Usage, missing Usage, partial Usage, cancellation races, timeout races, retry, duplicate completion, settlement failure, and terminal publication failure.
- [ ] Add Session, Workflow, HTTP, SSE, and frontend tests that assert only externally observable terminal state, charge, and error behavior.
- [ ] Run targeted backend and frontend tests, full backend test and build gates for shared terminal contracts, web typecheck, web production build, and diff validation.
