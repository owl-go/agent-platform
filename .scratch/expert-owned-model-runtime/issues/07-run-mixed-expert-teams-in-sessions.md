# 07 — Run mixed-model and mixed-engine Expert Teams in Sessions

**What to build:** Let a Session execute a frozen Expert Team whose members use different Provider Models and Runtime Engines, while preserving fixed order, bounded context handoff, isolation, fail-fast behavior, and final-member response semantics.

**Blocked by:** 05 — Execute single-Expert Sessions with the Expert profile.

**Status:** ready-for-agent

- [ ] The first Session message freezes one ordered Stage per Expert Team member with no synthetic team-level model or engine.
- [ ] Every Stage selects its own Runtime configuration, Adapter, credential version, instruction, MCP Servers, and Skills.
- [ ] Members execute strictly in frozen order and later members receive only bounded conversation context, attachments, and preceding final text results.
- [ ] Members using the same Runtime Engine still receive distinct execution context, credentials, Extensions, and warm identity.
- [ ] Failure identifies the active Expert and prevents all later members from starting.
- [ ] Cancellation stops the active member and skips the rest of the frozen order.
- [ ] The final member remains the official Agent response without an additional synthesis invocation.
- [ ] Session progress and history expose ordered member identity, model, engine, state, elapsed time, and final text.
- [ ] Retry restarts from the first frozen member; atomic Native Session behavior remains disabled or compatibility-preserving until ticket 09.
- [ ] Runtime Executor and Session component tests cover at least two different models and two different engines.
