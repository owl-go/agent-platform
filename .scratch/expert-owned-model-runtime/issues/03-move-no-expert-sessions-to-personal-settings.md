# 03 — Move no-Expert Sessions to Personal Settings

**What to build:** Make a no-Expert Session use the current Personal Settings execution defaults for every newly sent message, with no Session-level model or engine choice. Freeze the resolved anonymous Stage for audit and deterministic regeneration.

**Blocked by:** 01 — Expand to ordered Execution Stage Snapshots.

**Status:** ready-for-agent

- [ ] The Session composer contains no Provider Model or Runtime Engine selector and submits no execution override.
- [ ] Sending a no-Expert message resolves the current default Runtime Engine and that engine's default Provider Model into exactly one anonymous Stage.
- [ ] A later Personal Settings change affects the next new message but does not mutate queued, active, or terminal responses.
- [ ] Response history shows the anonymous Stage's effective model and engine as read-only execution identity.
- [ ] Regeneration reuses the original Response Snapshot even when Personal Settings have changed.
- [ ] Missing, unavailable, or incompatible defaults fail before Provider Model invocation and never fall back to another pair.
- [ ] Stale non-empty Session model override input is rejected while the compatibility field remains present during expansion.
- [ ] Existing attachment, streaming, cancellation, Rolling Summary, archive, and ownership behavior remains intact.
- [ ] Backend planner/service tests and Session component tests cover the complete user path.
- [ ] Generated-output verification, backend tests, frontend typecheck, unit tests, and production build pass.
