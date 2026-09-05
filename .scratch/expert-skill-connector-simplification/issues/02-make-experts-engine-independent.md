# 02 — Make Experts engine-independent

**What to build:** Let Users create and run Experts without configuring a Provider Model or Runtime Engine. Every new Session or Run Conversation instead freezes one execution configuration from Personal Settings and uses it with no Expert, one Expert, or every Expert Team member.

**Blocked by:** 01 — Expand Expert structured guidance.

**Status:** ready-for-agent

- [ ] Expert create, edit, cards, and public responses no longer require or expose mutable Provider Model or Runtime Engine selections.
- [ ] The first Session message and the first Run in a Run Conversation resolve the current Personal Settings Runtime Engine and its default Provider Model exactly once.
- [ ] Later Personal Settings changes do not alter that Session or Run Conversation, while a new conversation receives the new defaults.
- [ ] Core Capability, Operating Procedure, Output Standard, and Cautions are injected under fixed visible headings in that order.
- [ ] Introduction, Profile Icon, and Derived Expertise Tags are not injected into model instructions.
- [ ] Execution-configuration failures are reported as execution dependency failures and do not mark the Expert itself unavailable.
- [ ] Regeneration and follow-up turns reuse the frozen execution configuration and retain existing snapshot compatibility.
- [ ] API, execution-planning, Runtime fake, Session, Workflow, and responsive UI tests cover anonymous and single-Expert execution.
