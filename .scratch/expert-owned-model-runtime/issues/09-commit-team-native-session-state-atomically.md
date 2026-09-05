# 09 — Commit team Native Session state atomically

**What to build:** Give every frozen Expert Team member independent Native Session continuity and warm Runtime reuse without allowing a partial team turn to advance any member's durable state.

**Blocked by:** 07 — Run mixed-model and mixed-engine Expert Teams in Sessions; 08 — Run mixed-model and mixed-engine Expert Teams in Workflows.

**Status:** ready-for-agent

- [ ] Native Session state is keyed by owner, Session or Run Conversation, frozen Expert identity, and Runtime Engine.
- [ ] Each eligible member begins a turn from its preceding fully committed state and writes only to a temporary copy.
- [ ] All member states are promoted only after the complete turn and its required persistence succeed.
- [ ] Failure, cancellation, timeout, event publication failure, or final persistence failure discards every temporary member state from that turn.
- [ ] Retry begins every member from the preceding complete turn rather than a partially advanced checkpoint.
- [ ] Runtime images without conformance-proven native Resume continue correctly through Rolling Summary and recent context.
- [ ] Warm Runtime identity also includes the frozen Expert identity and image digest, preventing context sharing between members using the same engine.
- [ ] Warm definitions stop after each Stage, clean stage credentials, reject configuration drift, and expire after 30 idle minutes.
- [ ] Tests cover success, later-member failure, cancellation, retry, image change, unavailable Resume, and atomic promotion failure.
- [ ] No new Native Resume capability is reported without exact-image conformance evidence.
