# 04 — Move no-Expert Workflows to Personal Settings

**What to build:** Make a no-Expert Workflow resolve Personal Settings only when a new Run Conversation begins, then retain that anonymous execution Stage for every follow-up Run. Remove model and engine choices from Workflow configuration without disturbing its environment, schedule, API credential, Git Source, or Workspace behavior.

**Blocked by:** 01 — Expand to ordered Execution Stage Snapshots.

**Status:** ready-for-agent

- [ ] Workflow create and edit surfaces contain no Provider Model or Runtime Engine override controls.
- [ ] The first Run in a no-Expert Run Conversation freezes exactly one anonymous Stage from the then-current Personal Settings defaults.
- [ ] Follow-up Runs reuse the initiating Workflow Snapshot after Personal Settings change.
- [ ] A new Run Conversation uses the latest valid defaults without mutating earlier conversations.
- [ ] Workflow detail and history show the resolved model and engine as read-only execution metadata.
- [ ] Stale non-empty Workflow model or engine override input is rejected while compatibility fields remain present during expansion.
- [ ] Missing, unavailable, or incompatible defaults fail before an interactive or API Run is created; scheduled behavior remains unchanged until the availability ticket.
- [ ] Existing queueing, idempotency, environment, schedule, API credential, Git Source, and Workspace isolation behavior remains intact.
- [ ] Repository, service-contract, Workflow page, and Run Conversation tests cover snapshot timing and follow-up behavior.
- [ ] Generated-output verification, backend tests, frontend typecheck, unit tests, and production build pass.
