# 12 — Contract legacy execution overrides and verify the cutover

**What to build:** Complete the transition to Expert-owned or Personal Settings-owned execution by removing obsolete mutable override surfaces and old write paths after every caller uses ordered Execution Stage Snapshots. Preserve historical decoding and prove the complete cutover through repository gates and applicable conformance evidence.

**Blocked by:** 03 — Move no-Expert Sessions to Personal Settings; 04 — Move no-Expert Workflows to Personal Settings; 05 — Execute single-Expert Sessions with the Expert profile; 06 — Execute single-Expert Workflows with the Expert profile; 07 — Run mixed-model and mixed-engine Expert Teams in Sessions; 08 — Run mixed-model and mixed-engine Expert Teams in Workflows; 09 — Commit team Native Session state atomically; 10 — Enforce Expert availability across every trigger; 11 — Settle Credits per Execution Stage.

**Status:** ready-for-agent

- [ ] Public Session and Workflow contracts no longer expose Provider Model or Runtime Engine override fields.
- [ ] Generated backend bindings, OpenAPI, frontend types, clients, fixtures, and tests contain no calls to the removed overrides.
- [ ] A new immutable migration clears or removes obsolete mutable Session current-model and Workflow model/engine columns without changing historical snapshot JSON.
- [ ] New snapshot writers emit only the ordered Stage schema and no execution path relies on a synthetic top-level team model or engine.
- [ ] The explicit legacy snapshot reader remains covered for existing no-Expert, single-Expert, and shared-configuration team history.
- [ ] Searches confirm no active service, repository, Worker, UI, or API path reads the obsolete mutable override state.
- [ ] Documentation and generated contracts agree with the accepted Expert-owned execution ADR and product requirements.
- [ ] `gofmt`, generated-output verification, targeted package tests, all backend tests, and the backend build pass.
- [ ] Frontend typecheck, unit tests, and production build pass on desktop and mobile behavior.
- [ ] Migration or remote integration skips are reported as skips rather than passes.
- [ ] Runtime image smoke, Linux sandbox, and production conformance are run only in their required environments; any unrun gate or missing exact-digest evidence is reported explicitly.
