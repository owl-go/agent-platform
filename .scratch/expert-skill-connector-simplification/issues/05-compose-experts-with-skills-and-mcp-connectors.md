# 05 — Compose Experts with Skills and MCP Connectors

**What to build:** Let Users assemble an Expert from reusable catalog resources, create missing resources without losing edit context, and understand the impact before deleting a referenced resource.

**Blocked by:** 02 — Make Experts engine-independent; 03 — Ship the standalone Skill catalog; 04 — Ship the standalone MCP Connector catalog.

**Status:** ready-for-agent

- [ ] Expert create and edit can select existing User-owned Skills and successfully tested MCP Connectors.
- [ ] A User can install or upload a Skill inline and have the new catalog resource selected automatically.
- [ ] A User can create and test an MCP Connector inline and have it selected automatically only after the test succeeds.
- [ ] Expert cards show Skill and Connector counts without showing model or Runtime details.
- [ ] Starting a new Session or Run Conversation freezes the latest exact selected Skill and MCP Connector revisions; existing snapshots remain unchanged after updates.
- [ ] Skill or MCP Connector deletion first returns the affected mutable Experts and requires explicit confirmation.
- [ ] Confirmed deletion and detachment from mutable Experts commit transactionally, while immutable snapshots remain executable.
- [ ] API and responsive UI tests cover selection, inline creation, failed MCP testing, revision freezing, affected-resource confirmation, and rollback on failure.
