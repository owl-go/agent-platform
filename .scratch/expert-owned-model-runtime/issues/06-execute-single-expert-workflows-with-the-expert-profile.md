# 06 — Execute single-Expert Workflows with the Expert profile

**What to build:** Make every new Run Conversation selected with one Expert freeze and execute that Expert's complete execution profile while preserving Workflow conversation, queueing, Workspace, and trigger semantics.

**Blocked by:** 02 — Give Experts required execution profiles; 04 — Move no-Expert Workflows to Personal Settings.

**Status:** ready-for-agent

- [ ] The first Run freezes one Expert Stage containing the Expert's model, connection version, protocol, engine, instruction, Extensions, and Model Credit Rate.
- [ ] Manual, scheduled, and API-triggered conversations use the same frozen Stage construction.
- [ ] Follow-up Runs reuse the initiating snapshot after the source Expert or Personal Settings change.
- [ ] A new Run Conversation freezes the latest complete and available Expert profile.
- [ ] Runtime execution receives the Stage-specific Adapter, credential version, Extensions, and instruction while Workflow environment stays common.
- [ ] Workflow history and API status expose the Expert, Provider Model, Runtime Engine, state, elapsed time, and result.
- [ ] Execution failure preserves existing fail-closed Workspace and Artifact behavior and never falls back to Personal Settings.
- [ ] Existing serialization, idempotency, cancellation, rerun, schedule, and owner isolation behavior remains intact.
- [ ] Planner, repository, service, Runtime Executor, and Workflow component tests cover the complete path.
- [ ] Targeted and full backend/frontend gates pass.
