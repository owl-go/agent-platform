# 10 — Enforce Expert availability across every trigger

**What to build:** Make Expert availability a consistent, visible, fail-closed rule across selection, Session execution, all Workflow triggers, queued work, and Model Provider Connection deletion.

**Blocked by:** 02 — Give Experts required execution profiles; 05 — Execute single-Expert Sessions with the Expert profile; 08 — Run mixed-model and mixed-engine Expert Teams in Workflows.

**Status:** ready-for-agent

- [ ] A complete Expert becomes Unavailable when its selected model, connection, Runtime Engine, or compatibility is not currently usable.
- [ ] Incomplete and Unavailable remain distinct states with localized explanations and repairable edit flows.
- [ ] An Expert Team is unavailable when any frozen candidate member is incomplete or unavailable.
- [ ] New Session selection excludes unavailable Experts and teams while catalog pages keep them visible.
- [ ] Manual Workflow execution fails before creating a Run when its selected Expert or team is unavailable.
- [ ] Workflow API invocation returns a classified configuration error before creating a Run.
- [ ] A Scheduled Trigger creates a failed, uncharged Run identifying the unavailable Expert and dependency.
- [ ] Queued work revalidates before its first Provider Model invocation and records an uncharged failure if the dependency became unavailable.
- [ ] A Model Provider Connection cannot be deleted while referenced by Personal Settings, a mutable Expert, or a continuable frozen conversation; the error identifies reference categories without exposing private content.
- [ ] No path silently substitutes another model, connection, protocol, Runtime Engine, Expert, or team member.
- [ ] Repository, service, trigger, Worker, API, and component tests cover each availability transition and entry point.
