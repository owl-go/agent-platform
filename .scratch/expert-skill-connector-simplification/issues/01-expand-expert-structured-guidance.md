# 01 — Expand Expert structured guidance

**What to build:** Add the new Expert profile and guidance shape beside the legacy fields so Users can populate the accepted structure without breaking existing execution or historical data. This is the expand phase of the Expert contract migration.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Expert create, edit, detail, and list responses support a preset Profile Icon with a stable default, Introduction, Core Capability, Operating Procedure, Output Standard, and optional Cautions.
- [ ] Name, Introduction, Core Capability, Operating Procedure, and Output Standard are required for a newly completed Expert; Cautions are optional.
- [ ] The interface labels Operating Procedure as `工作流程`, while the public contract uses `operating_procedure`.
- [ ] Existing Capability Introduction migrates to Introduction and existing Execution Instruction migrates unchanged to Operating Procedure.
- [ ] New guidance fields remain empty for migrated Experts, which stay visible and editable but are marked incomplete until all required fields are supplied.
- [ ] Legacy model, Runtime, tag, and instruction data remains readable during the expand phase, and historical snapshots are not rewritten.
- [ ] Desktop and mobile Expert forms render and validate the expanded fields accessibly.
- [ ] Domain, migration, API, and UI tests cover both newly created and migrated Experts.
