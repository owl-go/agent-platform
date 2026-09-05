# 07 — Migrate Expert Teams to stable Team Members

**What to build:** Turn an Expert Team into an ordered set of named roles with stable identity, so the same Expert can participate more than once without mixing execution context.

**Blocked by:** 02 — Make Experts engine-independent.

**Status:** ready-for-agent

- [ ] Expert Team create and edit support a preset Profile Icon, required display-only Introduction, required display-only Core Capability, and two to ten ordered Team Members.
- [ ] Every Team Member has a stable generated ID, team-unique name, Expert reference, position, and up to five Member Labels of at most twenty characters.
- [ ] The same Expert may be referenced by multiple Team Members, and rename or reorder preserves each member's stable identity.
- [ ] Member name and labels are visibly injected before that member's Expert guidance; team Introduction and Core Capability are not injected.
- [ ] Each member has isolated Native Session and Runtime context keyed by stable member identity while all members share the conversation's frozen execution configuration.
- [ ] Sequential fail-fast execution, preceding-result handoff, final-member official response, full-team retry, atomic Native Session promotion, and success-only Workspace merge remain intact.
- [ ] Deleting an Expert referenced by a mutable team is rejected with affected-team information; immutable snapshots do not block deletion.
- [ ] Existing ordered Expert references migrate to stable members named from their Experts with empty labels, and historical snapshots are not rewritten.
- [ ] API, execution, migration, reorder, duplicate-Expert, deletion-conflict, desktop, mobile, and accessibility tests cover the complete team flow.
