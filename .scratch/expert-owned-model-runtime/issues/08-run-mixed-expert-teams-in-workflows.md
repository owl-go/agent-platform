# 08 — Run mixed-model and mixed-engine Expert Teams in Workflows

**What to build:** Let a Workflow execute a frozen mixed-model, mixed-engine Expert Team through one shared temporary Workspace and one overall turn contract. The complete ordered execution must be visible through the product and Workflow API.

**Blocked by:** 06 — Execute single-Expert Workflows with the Expert profile; 07 — Run mixed-model and mixed-engine Expert Teams in Sessions.

**Status:** ready-for-agent

- [ ] A new Run Conversation freezes one ordered Stage per current Expert Team member, including each member's distinct model, engine, connection version, instruction, and Extensions.
- [ ] Every Stage uses its own Runtime Adapter and isolated execution environment while mounting the same per-turn temporary Workspace.
- [ ] Later members observe earlier members' valid temporary file changes and preceding final text results.
- [ ] Only complete team success merges the temporary Workspace and creates Artifacts; failure or cancellation discards all temporary file changes.
- [ ] One overall timeout applies across all member Stages, with each member receiving only the remaining duration.
- [ ] The final member's result remains the official Run result and no extra coordinator invocation occurs.
- [ ] Workflow API status returns overall state plus ordered member identity, model, engine, state, elapsed time, and result.
- [ ] Workflow history shows the final execution identity by default and allows all Stages to be inspected.
- [ ] Follow-up Runs reuse the initiating ordered snapshot even after the mutable team or Experts change.
- [ ] Runtime Executor, repository, API contract, and Workflow component tests cover success, failure, cancellation, timeout, and Workspace rollback.
