# 01 — Expand to ordered Execution Stage Snapshots

**What to build:** Introduce the compatible execution-planning foundation that can represent every Provider Model invocation as an ordered Execution Stage Snapshot without changing current user-visible execution behavior. Session and Workflow planning must share this seam, and historical snapshots must remain readable while later tickets migrate each caller.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] One shared Workspace Application planner resolves no Expert, one Expert, or an ordered Expert Team into Execution Stage Snapshots.
- [ ] A Stage can freeze the optional Expert identity, Provider Model and connection version, API Protocol, Endpoint, Runtime Engine, Execution Instruction, Extensions, and Model Credit Rate needed for one invocation.
- [ ] Common Session or Workflow input, Personality, attachments, environment, overall timeout, and Workspace identity remain outside individual Stages.
- [ ] New snapshots carry an explicit schema identity and can round-trip an ordered Stage list without using a synthetic team-level model or engine.
- [ ] The legacy decoder reads existing top-level model and engine snapshots with their original shared semantics and never consults mutable Expert data.
- [ ] Existing Session, Workflow, Worker, and history behavior remains unchanged until its migration ticket adopts the new plan.
- [ ] Table-driven planner tests cover anonymous, single-Expert, ordered-team, malformed, cross-owner, and legacy inputs.
- [ ] Targeted backend tests and the full backend build remain green.
