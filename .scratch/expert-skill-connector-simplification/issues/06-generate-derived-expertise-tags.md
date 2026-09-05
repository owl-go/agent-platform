# 06 — Generate Derived Expertise Tags

**What to build:** Automatically enrich Expert discovery from Core Capability so Users no longer maintain a second set of capability labels.

**Blocked by:** 02 — Make Experts engine-independent.

**Status:** ready-for-agent

- [ ] Saving a changed Core Capability schedules asynchronous tag projection with the User's current Personal Settings default Provider Model.
- [ ] Projection produces at most five normalized Derived Expertise Tags and cannot accept User-authored tag mutations.
- [ ] Tag generation invokes no User-credit admission or settlement and creates no Credit Ledger consumption.
- [ ] Expert save and execution succeed while projection is queued, running, or failed.
- [ ] A failed refresh preserves the last successful projection and exposes a recoverable status without replacing it with empty tags.
- [ ] Derived tags support Expert card display, search, and filtering but never enter Expert instructions.
- [ ] Owner isolation prevents one User's projection or tags from being read through another User's Expert.
- [ ] Fake-model, concurrency, failure, search, API, and responsive UI tests remain deterministic and make no external model call.
