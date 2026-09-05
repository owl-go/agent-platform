# 07 — Settle Credits for each Expert Team Execution Stage

**What to build:** A mixed-model and mixed-Runtime Expert Team admits, measures, and settles every member independently, shows per-member and total Credit Consumption, and stops safely when a preceding member exhausts the User's balance.

**Blocked by:** 06 — Settle failed, cancelled, and timed-out model invocations exactly once.

**Status:** ready-for-agent

- [ ] Freeze a Model Credit Rate revision and Credit Day for each ordered Execution Stage Snapshot using that Expert's Provider Model and API Protocol.
- [ ] Apply the correct exact or default rate independently when a team mixes Provider Models, protocols, or Runtime Engines.
- [ ] Recheck total Credit Balance immediately before every member invokes its Provider Model.
- [ ] Admit the first member only with positive balance and allow its actual settlement to make the balance non-positive.
- [ ] If a completed member leaves a non-positive balance, fail the overall turn before starting the next member.
- [ ] Leave every unstarted member uncharged and without fabricated Runtime Usage or terminal-success data.
- [ ] Preserve Credit Consumption for every completed member even when a later member is insufficient, fails, is cancelled, or times out.
- [ ] Settle every member's terminal Stage state, ledger entry, and balance projections atomically and idempotently by execution identity and Stage position.
- [ ] Commit the overall Assistant Message or Run terminal state with the final member settlement on success.
- [ ] On later-member insufficient Credits or failure, commit the overall failed or cancelled state without reversing legitimate earlier-member charges.
- [ ] Discard the whole turn's temporary Workflow Workspace changes when the team does not complete successfully.
- [ ] Discard every member's staged Native Session state when the team does not complete successfully, while preserving prior successful-turn continuity.
- [ ] Restart Retry at the first frozen member and charge each newly invoked member independently.
- [ ] Sum completed member Credit Consumption into the terminal Assistant response or Run total without double counting.
- [ ] Persist and expose safe per-member Provider Model, Runtime Engine, input Tokens, output Tokens, multipliers, estimated marker, and Credit Consumption to the owning User.
- [ ] Keep intermediate member results collapsed by default and add localized per-stage consumption detail without exposing private reasoning or raw Runtime events.
- [ ] Ensure Administrator account views contain only aggregate User credit metadata and cannot reveal Expert Team membership, stage models, Tokens, or execution consumption.
- [ ] Add Session and Workflow tests for two through ten members, mixed models and engines, exact and default rates, fallback, zero rate, later-stage exhaustion, failure, cancellation, timeout, retry, total calculation, Workspace rollback, and Native Session rollback.
- [ ] Extend real Runtime evidence only when exact image and mixed-engine prerequisites exist; deterministic fakes remain required for contract coverage and are not production conformance.
- [ ] Run targeted backend and frontend tests, full backend test and build gates for shared Expert Team and Worker behavior, web typecheck, web production build, and diff validation.
