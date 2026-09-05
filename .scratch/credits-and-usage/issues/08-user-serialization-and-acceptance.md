# 08 — Serialize User model invocation and prove the Credits rollout

**What to build:** Credits becomes safe to enable for all existing Users: Session, Workflow, and Expert Team model invocations share one User-level serialized admission boundary, migration and security behavior are proven, and a real browser-to-API flow demonstrates the complete Administrator and User experience.

**Blocked by:** 03 — Issue and redeem single-use Redemption Codes; 07 — Settle Credits for each Expert Team Execution Stage.

**Status:** ready-for-agent

- [ ] Enforce at most one actively executing credit-consuming Provider Model invocation per User across Session, manual Workflow, Workflow API, Scheduled Trigger, follow-up, rerun, and Expert Team Stage entry points.
- [ ] Let additional eligible work wait under existing queue semantics and recheck positive Credit Balance immediately before actual model invocation.
- [ ] Prevent concurrent submissions with a small positive balance from starting multiple model invocations or amplifying negative balance.
- [ ] Release the User-level execution lease after every success, failure, cancellation, timeout, pre-model error, and settlement recovery path.
- [ ] Preserve existing Session-level, Run Conversation-level, and Workflow Workspace serialization while adding the User-level Credit boundary.
- [ ] Apply the additive rollout migration to every existing User with the current Credit Day's full 600 Credits and zero redeemed balance.
- [ ] Make rollout initialization idempotent and compatible with a User whose account is created concurrently with migration or first access.
- [ ] Verify Credit Ledger and projection reconstruction produce the same balance, daily remainder, persistent balance, and today's consumption.
- [ ] Verify every balance-changing operation uses an immutable ledger entry and no API or Administrator action directly overwrites a projection.
- [ ] Verify Credit Ledger entries, Redemption Code records, Credit Adjustments, and Model Credit Rate revisions cannot be deleted through first-version APIs.
- [ ] Run secret canaries through Redemption Code generation and all existing model, Workflow, MCP, Skill, Git, Artifact, result, log, Event, and SSE paths and prove plaintext values do not escape their authorized one-time response.
- [ ] Verify Administrator views expose account balance, today usage, allocation, redemption, and adjustment metadata but no User-owned execution-level details.
- [ ] Verify an unverified Runtime-reported Token count is presented only as anti-abuse accounting and is not recorded or advertised as verified Usage Capability evidence.
- [ ] Verify a Runtime with no Token counts uses the frozen 10.00 fallback unless an explicit zero or other revision applies.
- [ ] Complete a real browser-to-API Administrator flow for User allocation, reasoned adjustment, Model Credit Rate revision, Redemption Code batch generation, and safe status management.
- [ ] Complete a real browser-to-API User flow for avatar balance, Credit panel, code redemption, ledger, Session consumption, Workflow consumption, insufficient Credits, negative balance, and next-day recovery.
- [ ] Complete a real two-member Expert Team flow that shows per-stage and total consumption and a later-stage insufficient-credit rollback.
- [ ] Verify Chinese and English copy, keyboard operation, accessible status semantics, compact desktop layout, and mobile information order for all Credits interfaces.
- [ ] Verify public insufficient-credit errors consistently use insufficient_credits and HTTP 429 where applicable without leaking internal rates or another User's data.
- [ ] Run targeted tests first, then the complete backend test suite, backend build, web test suite, web typecheck, web production build, migration validation, browser-to-API acceptance, and diff check.
- [ ] Run applicable Runtime image conformance only when the exact Linux, container, and image prerequisites exist; report unavailable environment gates and do not count skips as passing evidence.
- [ ] Update implementation-status documentation only after the real browser-to-API flow passes; keep accepted specification distinct from current capability until then.
