# 11 — Settle Credits per Execution Stage

**What to build:** Apply the Credit Ledger contract to every actual Expert Team Provider Model invocation so mixed-model cost is checked, frozen, settled, displayed, and rolled back at the same Stage boundaries as execution.

**Blocked by:** 08 — Run mixed-model and mixed-engine Expert Teams in Workflows; 09 — Commit team Native Session state atomically; external prerequisite — the Credit Ledger capability must be available.

**Status:** ready-for-agent

- [ ] Every Stage freezes the Model Credit Rate matched by Provider type, Model API Protocol, and exact Provider Model identifier.
- [ ] Credit Balance is checked immediately before each Stage starts its Provider Model invocation.
- [ ] A started Stage settles measured usage or its frozen missing-Usage fallback independently of other members.
- [ ] Successful, failed, cancelled, and timed-out invocations charge only according to the Credit Ledger contract.
- [ ] If a completed member makes the balance non-positive, the turn fails before the next member invokes a model.
- [ ] Completed member charges remain immutable and unstarted members receive no charge.
- [ ] Mid-team insufficient Credits discards the turn's temporary Workspace and all staged Native Session state.
- [ ] Session responses, Workflow Runs, member details, and API results show per-Stage consumption and the summed turn total.
- [ ] Retry after Credits become available restarts from the first frozen member and creates new independent consumption records.
- [ ] Per-User invocation serialization and immediate pre-start recheck prevent concurrent amplification of a negative balance.
- [ ] Tests cover different member rates, measured usage, fallback usage, insufficient Credits at every position, rollback, totals, and retry.
