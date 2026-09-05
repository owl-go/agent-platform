# 03 — Ship the standalone Skill catalog

**What to build:** Give Users a top-level `技能·连接器` catalog where they can manage their private Skills independently of Experts and Personal Settings.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Navigation exposes `Skills & Connectors` with the Chinese label `技能·连接器` and opens a dedicated catalog rather than Personal Settings.
- [ ] The catalog provides separate Skill and Connector tabs, with the Skill tab usable in this slice.
- [ ] A User can install a Skill from a Git URL or upload a ZIP whose root contains `SKILL.md`.
- [ ] A User can list, inspect, update, and delete only their own Skills; cross-owner access remains non-enumerating.
- [ ] Every accepted Skill revision records immutable normalized content, Object Key, and lowercase SHA-256.
- [ ] Updating a Skill makes the latest revision available for future snapshots without changing previously frozen revisions.
- [ ] Desktop and mobile states cover loading, empty, validation, success, and failure behavior.
- [ ] Browser-to-API tests cover Git and ZIP installation, ownership, revision history, and deletion.
