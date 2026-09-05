# 02 — Give Experts required execution profiles

**What to build:** Let a User create, edit, browse, and select Experts whose required Provider Model and Runtime Engine form a visible execution profile. Preserve legacy Experts as editable Incomplete Experts rather than inventing configuration for them.

**Blocked by:** 01 — Expand to ordered Execution Stage Snapshots.

**Status:** ready-for-agent

- [ ] An additive immutable migration adds nullable Provider Model and Runtime Engine references to existing Experts without rewriting historical snapshots.
- [ ] New and completed Experts require an owned, available Provider Model, a supported Runtime Engine, and a non-empty Execution Instruction.
- [ ] Saving a verified pair succeeds; saving an unverified pair succeeds with a persistent warning; saving an incompatible pair fails with a classified validation error.
- [ ] Existing Experts missing an instruction, model, or engine remain visible and editable as Incomplete Experts but are excluded from new selection and execution.
- [ ] The Expert editor suggests the selected engine's Personal Settings default model but requires the User to confirm the model and engine before saving.
- [ ] Expert cards and selectors expose Provider Model, Runtime Engine, compatibility, completeness, and availability in Chinese and English.
- [ ] Expert Team member summaries expose each referenced Expert's model and engine without adding a team-level override.
- [ ] API contracts, generated clients, persistence mappings, optimistic concurrency, and owner isolation agree on the new fields.
- [ ] Component tests cover desktop and mobile information ordering without hiding required status.
- [ ] Targeted backend and frontend tests, generated-output verification, typecheck, and production build pass.
