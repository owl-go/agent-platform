# 08 — Contract legacy Expert and Extension surfaces

**What to build:** Finish the expand–contract migration by removing obsolete mutable Expert execution fields and the User-visible Extension surface after all callers use Experts, Skills, and Connectors directly.

**Blocked by:** 05 — Compose Experts with Skills and MCP Connectors; 06 — Generate Derived Expertise Tags; 07 — Migrate Expert Teams to stable Team Members.

**Status:** ready-for-agent

- [ ] New Expert contracts no longer accept or return Capability Introduction, broad Execution Instruction, Provider Model, Runtime Engine, or User-authored tags.
- [ ] New Skill and MCP Connector APIs use their standalone routes, and deprecated `/extensions` aliases are removed rather than retained.
- [ ] Personal Settings no longer contains an Extension manager, and no User-facing screen uses Extension terminology.
- [ ] The Experts entry contains Expert and Expert Team tabs; the Skills & Connectors entry contains Skill and Connector tabs on desktop and mobile.
- [ ] Legacy Expert model, Runtime, and hand-authored tag columns are ignored for new execution and may remain only for the documented compatibility window.
- [ ] Historical Expert and execution snapshot readers preserve their original model, Runtime, instruction, Skill, and MCP semantics without rewriting stored JSON.
- [ ] Contract generation, API compatibility fixtures, navigation tests, and full backend/frontend gates prove no current caller depends on the contracted forms.
