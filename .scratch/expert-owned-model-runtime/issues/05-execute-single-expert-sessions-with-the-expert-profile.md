# 05 — Execute single-Expert Sessions with the Expert profile

**What to build:** Make a Session selected with one Expert freeze and execute that Expert's complete execution profile from its first message onward. The Session must remain stable when the source Expert or Personal Settings later change.

**Blocked by:** 02 — Give Experts required execution profiles; 03 — Move no-Expert Sessions to Personal Settings.

**Status:** ready-for-agent

- [ ] The first Session message freezes one Expert Stage containing the Expert's model, connection version, protocol, engine, instruction, and exact Extensions.
- [ ] The Runtime Adapter invocation receives only the frozen Stage configuration and the current bounded conversation input.
- [ ] Later Expert, Extension, Model Provider Connection, or Personal Settings edits do not alter the existing Session snapshot.
- [ ] New Sessions created after an Expert edit freeze the new execution profile.
- [ ] Response metadata shows the Expert, Provider Model, Runtime Engine, compatibility state, elapsed time, and terminal result.
- [ ] Regeneration reuses the original Expert Stage and protected credential version.
- [ ] An Incomplete or currently Unavailable Expert cannot be selected for a new Session.
- [ ] Execution never falls back to Personal Settings when the frozen Expert Stage fails.
- [ ] Existing streaming, cancellation, attachments, Rolling Summary, and ownership guarantees remain intact.
- [ ] Planner, service, Runtime Executor, and Session component tests cover the end-to-end behavior.
