# 04 — Configure and freeze Model Credit Rates

**What to build:** An Administrator can create versioned platform Model Credit Rates, and every new single-stage Session invocation resolves and freezes the correct rate so Users receive stable, explainable consumption even after later rate edits.

**Blocked by:** 01 — Complete the single-stage Session Credit loop.

**Status:** ready-for-agent

- [ ] Add immutable, additive persistence for the platform default Model Credit Rate and exact-match Model Credit Rate revisions.
- [ ] Seed a versioned platform default with input multiplier 1.00, output multiplier 1.00, and missing-Usage fallback 10.00 Credits.
- [ ] Match an exact rate by Provider type, Model API Protocol, and exact Provider Model identifier.
- [ ] Do not use User identity, connection name, Endpoint, Runtime Engine, or private connection identity as a matching dimension.
- [ ] Resolve the current exact revision or platform default before an Execution Stage starts and freeze both revision identity and values into the Stage snapshot.
- [ ] Make an Administrator edit create a new immutable revision rather than updating a historical revision.
- [ ] Ensure queued, active, regenerated, and historical stages retain their frozen revision and final Credit Consumption after later edits.
- [ ] Allow explicit non-negative input multiplier, output multiplier, and fallback values, including zero to make matching usage intentionally free.
- [ ] Ensure an absent exact rule always selects the versioned platform default and never creates accidental free usage.
- [ ] Keep provider-reported currency cost outside the matching and calculation rules.
- [ ] Normalize Runtime Usage to the current invocation's Token delta before applying the frozen rate; never recharge cumulative native-session Tokens.
- [ ] When usable Token counts exist, calculate input and output components independently, sum them, round half-up once to two decimals, and apply the minimum non-zero 0.01 Credit charge.
- [ ] When no usable Token counts exist, charge the frozen fallback and mark the result as estimated without fabricated Token numbers.
- [ ] Distinguish a real Token value of zero from a missing value; if the Runtime contract cannot distinguish partial absence safely, select the whole-invocation fallback.
- [ ] Add Administrator-only APIs for listing current and historical safe rate metadata and creating a revision, with optimistic concurrency for competing edits.
- [ ] Add a Model Rates tab to User Management with platform-default and exact-rule forms, revision history, zero-rate confirmation, and localized validation.
- [ ] Expand the Session consumption disclosure to show frozen input/output multipliers, component Tokens, final Credits, rate revision metadata safe for the User, and estimated fallback state.
- [ ] Do not expose User-owned Model Provider Connections, API Keys, Endpoints, or model usage from another User in Administrator rate management.
- [ ] Add deterministic rate-resolution, revision, snapshot, regeneration, cumulative-Usage normalization, zero-rate, default fallback, decimal rounding, and authorization tests.
- [ ] Run targeted backend and frontend tests, backend build, web typecheck, web production build, and diff validation for this slice.
