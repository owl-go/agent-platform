---
status: proposed
---

# Portable Experts and lead-coordinated Expert Teams

On 2026-10-09, the User confirmed three architectural directions: Experts and Expert Teams support portable packages using a neutral `.plugin/plugin.json` entry; each Expert has one authoritative Markdown guidance document and a separate display-only Introduction; Expert Teams use a Team Lead that delegates according to the task and produces the official response, with platform-enforced delegation, concurrency, and consumption limits. This makes specialist definitions portable and allows teams to use the members needed for a task rather than invoking every member unconditionally. A creation wizard may generate the same guidance document but must not maintain a second authoritative instruction representation.

These directions are confirmed; the complete execution and migration contract is still under design in [the refactor design](../product/expert-package-team-refactor.md). This proposal will replace ADR-0022's fixed-order execution and final-member response decision, and ADR-0026's independently authored structured guidance fields, once the remaining decisions are settled. The existing implementation still follows those earlier contracts; this record claims no implementation, deployment, or conformance evidence.

Platform-controlled orchestration preserves the shared execution boundary across Runtime Engines. Member contexts remain isolated, packages contain no account credentials, and package import does not grant Connector Authorization. Package schema, member dependency ownership, legacy migration, concurrent Workspace handling, failure/retry behavior, and numerical execution limits remain open decisions; they are not implicitly accepted by this proposal.
