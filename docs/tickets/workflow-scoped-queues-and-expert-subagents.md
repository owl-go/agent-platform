# Workflow-scoped queues and Expert Team Subagents

Triage: `ready-for-agent`

## Problem Statement

Agent Workspace currently has two conflicting execution assumptions. Expert Teams need platform-managed specialist execution, but the desired contract is not yet explicit enough about which layer creates and coordinates Subagents. A single Expert should remain a direct specialist invocation, while an Expert Team should make each Team Member an isolated Subagent and let the main execution coordinator pass results between members in a deterministic order.

Workflow concurrency is also too coarse. A User may open multiple Sessions and own multiple Workflows, but a User-level Runtime execution lease makes unrelated work wait behind one another. At the same time, a Workflow owns one persistent Workspace whose changes must be serialized. Concurrent follow-ups in one Run Conversation are rejected even though they should be durable queued turns, and the product does not provide a complete, recoverable view of queue position and waiting work.

The current Credit admission model relies on one unique execution lease per User because text usage is unknown before invocation. That lease prevents the required cross-Workflow concurrency. The platform needs a bounded stage-level reservation that protects Available Credit while preserving post-execution Token settlement, negative-balance carry-forward, exact-once ledger writes, and the existing ownership and secret boundaries.

## Solution

Keep a single Expert as one direct Execution Stage with no coordinator or extra synthesis call. For an Expert Team, create one platform-managed Subagent execution per Team Member. The main execution coordinator schedules the frozen members strictly in Team Member order, gives each member the current task, bounded conversation context, attachments, and preceding members' final text, and isolates each member's Runtime context, credentials, Native Session state, and resource revisions. The final member remains the official response author. Any required member failure is fail-fast; retry starts the entire frozen team again.

Introduce a persistent FIFO Workflow Queue for each Workflow. Manual, scheduled, API, follow-up, and rerun requests use the same queue. Only one Run for a Workflow may be `running` or `waiting_for_user`, because the Workflow's persistent Workspace is the serialization boundary. Different Workflows and different Sessions may execute concurrently, including when owned by the same User. A Workflow has at most five queued Runs in the first version, excluding the active Run. Queue position is calculated from authoritative enqueue order rather than stored as a mutable ordinal.

Accept concurrent follow-ups in one Run Conversation. Each accepted turn receives a stable Run ID and a transactionally assigned, never-reused turn number in enqueue order. Requests return immediately with `202`; GET and SSE expose the authoritative state and dynamic queue position. Manual/API requests beyond the five queued Run limit return `429 queue_full` without creating a Run; a scheduled trigger records an auditable failed Run with the same safe error classification.

Replace the unique User-wide text execution lease with an Execution Credit Reservation created atomically when a Stage starts. The reservation equals the frozen Model Credit Rate fallback for that stage. Expert Team members reserve and settle one at a time. Measured usage is settled exactly once and may exceed the reservation under the existing negative-balance semantics; a zero fallback creates no reservation. Active reservations reduce Available Credit, and an admission that cannot create its reservation fails before provider invocation without charging that stage.

## User Stories

1. As a User, I want a single selected Expert to execute directly, so that choosing an Expert does not add an invisible coordinator call.
2. As a User, I want each Expert Team Member to run as its own Subagent, so that specialist contexts remain isolated and attributable.
3. As a User, I want an Expert Team to execute members in the configured order, so that later specialists can depend on earlier results.
4. As a User, I want every later Subagent to receive the current task and bounded conversation context, so that the team remains grounded in the same request.
5. As a User, I want every later Subagent to receive preceding members' final text in order, so that explicit specialist handoff is deterministic.
6. As a User, I want current attachments and File References to reach every team member, so that each specialist can inspect the same accepted inputs.
7. As a User, I want each member's own Expert guidance and default resources to remain isolated, so that one member cannot accidentally inherit another member's tools.
8. As a User, I want raw reasoning, tool logs, and private Runtime events excluded from team context, so that collaboration does not leak internal execution details.
9. As a User, I want the final Team Member to produce the official response, so that the platform does not silently charge for an extra synthesis model call.
10. As a User, I want the main execution coordinator to publish member progress, so that I can tell which specialist is queued, running, complete, or failed.
11. As a User, I want an Expert Team to fail fast when a required member fails, so that an incomplete collaboration is not presented as a complete answer.
12. As a User, I want a failed team retry to restart from the first member, so that all member results belong to one coherent attempt.
13. As a User, I want a cancelled team to stop the active member and skip remaining members, so that cancellation remains responsive.
14. As a Workflow owner, I want all team members in one turn to use the same temporary Workspace sequentially, so that later members can inspect earlier file changes.
15. As a Workflow owner, I want the persistent Workspace merged only after the complete team succeeds, so that failed teams cannot leave partial files behind.
16. As a User, I want each Team Member's Native Session state isolated and promoted atomically, so that one failed member cannot partially advance continuity.
17. As a User, I want different Sessions to execute concurrently, so that starting a response in one conversation does not block another conversation.
18. As a User, I want one Session's responses to remain serialized within that Session, so that its message history stays ordered.
19. As a User, I want different Workflows to execute concurrently even when I own them all, so that unrelated work does not wait on a User-wide lock.
20. As a Workflow owner, I want one Workflow's persistent Workspace changes serialized, so that concurrent Runs cannot corrupt or overwrite the shared tree.
21. As a Workflow owner, I want manual, scheduled, API, follow-up, and rerun requests to share one Workflow queue, so that every Workspace mutation follows one understandable order.
22. As an API client, I want a request accepted into the queue to return immediately with a stable Run ID, so that I do not need to hold an HTTP connection open.
23. As an API client, I want `202` for accepted queued work, so that queued is distinguishable from provider execution failure.
24. As an API client, I want `Idempotency-Key` replay to return the original Run, so that transport retries cannot create duplicate work.
25. As an API client, I want a same-Workflow request to queue rather than fail merely because another Run is active, so that transient concurrency is handled durably.
26. As a Run Conversation user, I want concurrent follow-ups accepted and ordered, so that I can submit the next turn without racing the current turn.
27. As a Run Conversation user, I want each follow-up turn number assigned transactionally and never reused, so that cancelled or failed turns do not reorder history.
28. As a User, I want a queued Run's current position shown, so that I can understand why it has not started.
29. As a User, I want queue position to update after cancellation or completion, so that the display reflects the authoritative queue.
30. As a User, I want queue state and position restored after refresh or reconnect, so that leaving the page does not hide active work.
31. As a User, I want queued, running, waiting-for-user, succeeded, failed, and cancelled states shown consistently, so that I can distinguish waiting from failure.
32. As a User, I want to cancel a queued Run, so that unwanted work does not consume a queue slot.
33. As a User, I want cancelling a queued Run to immediately allow the next Run to advance, so that the queue does not stall.
34. As a User, I want to cancel an active Run using the existing context-cancellation behavior, so that cancellation does not require a separate force-kill control.
35. As a User, I want a Run waiting for approval to retain its Workflow queue slot, so that shared Workspace and credential state remain isolated.
36. As a User, I want an approval wait in one Workflow not to block another Workflow, so that unrelated work remains concurrent.
37. As a User, I want deleting or disabling a Workflow to cancel Runs that have not started, so that removed resources cannot execute later.
38. As a User, I want Worker restart to preserve each Workflow queue's enqueue order, so that recovery does not reorder requests.
39. As a User, I want provider outcomes that cannot be confirmed to fail closed without automatic replay, so that recovery cannot duplicate external side effects.
40. As a User, I want Available Credit to account for active text-stage reservations, so that concurrent admissions cannot ignore in-flight entitlement.
41. As a User, I want each started text stage to reserve the frozen rate fallback atomically, so that parallel Workflows have a bounded admission cost.
42. As a User, I want a stage whose reservation cannot be created to fail before provider invocation, so that I am not charged for work that never started.
43. As a User, I want measured usage above the reservation to settle under the existing negative-balance rule, so that actual consumption remains authoritative.
44. As a User, I want a zero-fallback rate to remain free without a synthetic reservation, so that explicit free rates retain their meaning.
45. As a User, I want each Expert Team member to reserve and settle independently, so that only started members consume Credits.
46. As a User, I want a later team member blocked when the preceding settlement leaves no Available Credit, so that unstarted members remain uncharged.
47. As a User, I want every reservation and settlement to be idempotent by execution identity and stage position, so that retries and Worker recovery cannot double-charge me.
48. As a User, I want Credit reservations released or settled when an execution reaches any terminal state, so that abandoned work cannot hold entitlement forever.
49. As an Administrator, I want account-level Credit and ledger data preserved while execution locks become Workflow-scoped, so that concurrency changes do not weaken auditability.
50. As a User, I want stage identities, member states, safe errors, and total Credit Consumption visible without private reasoning or raw provider output, so that I can audit outcomes safely.
51. As a mobile User, I want queue state, position, member progress, and cancellation controls to remain usable on narrow screens, so that background work is manageable away from desktop.
52. As a User, I want historical Run and Session snapshots to remain immutable, so that new queue or Subagent behavior cannot rewrite prior execution meaning.

## Implementation Decisions

- Extend the Workspace execution orchestration seam so that a single Expert remains one direct stage and an Expert Team expands to one ordered Subagent stage per frozen Team Member.
- Keep Subagent behavior platform-managed. Runtime Drivers continue to implement only their CLI construction, version detection, and output parsing; Runtime-native subagent features are not required.
- Pass each Subagent the current task, bounded Rolling Summary/recent messages, accepted attachments and File References, visible Team Member role context, its own Expert guidance/default resources, and all preceding members' final text. Do not pass raw reasoning, tool logs, or private Runtime events.
- Keep the final Team Member as the official response author. Do not add an implicit coordinator synthesis invocation.
- Preserve the existing shared temporary Workspace, success-only merge, per-member isolated Runtime context, atomic Native Session promotion, overall timeout, cancellation, fail-fast, and full-team retry rules.
- Add a persistent Workflow Queue concept backed by the Run persistence model. Queue order is `queued_at` plus a stable tie-breaker; queue position is calculated from current authoritative rows and is never treated as a durable ordinal.
- Enforce a first-version limit of five queued Runs per Workflow. Running and `waiting_for_user` Runs do not count toward this queued limit. Manual/API over-limit requests return `queue_full` with HTTP 429 and do not insert a Run; scheduled over-limit triggers insert a failed, uncharged history Run.
- Make Workflow ID the Workspace concurrency key. All triggers share that key, while different Workflows and different Sessions use independent execution slots.
- Remove the unique User-wide text execution lease from Worker claiming and Credits admission. Preserve per-Session serialization and per-Workflow serialization independently.
- Change concurrent Run Conversation follow-up creation from conflict rejection to transactional enqueue. Lock the conversation root while assigning the next turn number; cancelled turns retain their assigned number.
- Keep accepted API creation asynchronous: return HTTP 202 and a stable Run ID, expose state and queue position through normal reads and the existing Run SSE stream, and preserve `Idempotency-Key` replay behavior.
- Permit cancellation of queued Runs and immediately make the next queued Run eligible. Keep existing context cancellation for active Runs; do not introduce a force-kill product control.
- On Workflow deletion/disablement, cancel not-yet-started Runs and preserve historical records according to existing deleted-record rules. On Worker restart, requeue recoverable non-terminal work in its Workflow Queue and fail closed for provider-unknown outcomes.
- Add a stage-level Execution Credit Reservation to the Credits application boundary. At Stage start, lock the User credit projection, materialize the current Credit Day, and reserve the frozen Model Credit Rate fallback before invoking the Provider Model.
- Include active text-stage reservations in Available Credit. Store enough source/day metadata to release or settle reservations exactly once without changing the existing daily-expiry and redeemed-balance rules.
- Settle measured usage or the frozen fallback exactly once at stage terminal transition. Measured usage may exceed the reservation and may produce the existing negative Credit Balance; a zero fallback creates no reservation.
- Reserve and settle Expert Team stages one at a time. If a later reservation cannot be admitted, fail the turn before that provider call, charge only prior started stages, discard temporary Workspace/Native Session state, and leave unrelated Workflow queues eligible.
- Keep event invariants unchanged: Run ID matches, sequences start at 1 and increase monotonically, exactly one terminal event exists, and publishing/persistence failures stop execution. Add structured queue and member-stage progress payloads containing only stable IDs, display names, positions, states, and safe errors.
- Extend Run and Session response projections with dynamic queue position and ordered member-stage state/results where already authorized. Never expose credentials, raw Runtime events, private reasoning, or provider responses.
- Add an additive database migration for queue/reservation data and any indexes needed for Workflow-scoped claiming. Existing immutable Run, Session, snapshot, Artifact, and ledger records remain readable.
- Preserve snapshot immutability. Queueing may revalidate a frozen first stage before invocation, but mutable Expert, Workflow, Personal Settings, or connection edits cannot rewrite an accepted snapshot.
- Keep object storage, Workspace path safety, credential materialization, Secret redaction, Connector approval, and Runtime sandbox boundaries unchanged.
- Record this change as the accepted Workflow-scoped concurrency ADR that supersedes only ADR-0024's former User-wide serialization rationale; ADR-0022 remains the authority for sequential Expert Team behavior.

## Testing Decisions

- Assert externally observable behavior at the highest existing seams: Workspace Application orchestration, Credits application boundary, Repository transactions, Worker claim/recovery, authenticated HTTP/SSE, and Vue page behavior. Do not assert helper names, SQL formatting, goroutine layout, or private JSON implementation details.
- Add table-driven Expert Team orchestration tests covering one direct Expert, two-to-ten ordered Subagents, repeated Expert references with isolated member identities, preceding-result handoff, attachment handoff, resource isolation, final-member response, fail-fast, cancellation, retry, and bounded context behavior.
- Extend Runtime Executor tests with recording Adapters that prove one invocation per member, strict order, isolated credentials/checkpoints, shared temporary Workspace visibility, no extra synthesis call, and no raw reasoning/tool-log handoff.
- Add Credit tests for concurrent reservations from one User across different Workflows, source/day accounting, positive Available Credit admission, measured usage above reservation, zero fallback, exact-once settlement, cancellation, timeout, Worker recovery, and negative-balance carry-forward.
- Add Repository integration tests for Workflow queue FIFO order, five queued limit, dynamic position after cancellation/completion, concurrent Workflow claims, per-Session serialization, same-Conversation concurrent follow-ups, stable non-reused turn numbers, and deletion/disablement cancellation.
- Add Worker integration tests proving one active Run per Workflow, no cross-Workflow blocking, approval wait retaining its Workflow slot, queue advancement after cancellation, restart requeue ordering, provider-unknown fail-closed behavior, and no duplicate terminal events.
- Add API contract tests proving 202 responses, stable Run IDs, Idempotency-Key replay, `429 queue_full`, scheduled failed history records, dynamic queue position, queued cancellation, same-Conversation follow-up acceptance, and owner isolation.
- Add SSE tests for replay and live queue/member events, monotonic sequence numbers, safe payload fields, reconnect recovery, terminal closure, and no events after terminal state.
- Add Vue component/page tests for queued state, position changes, queue-full errors, cancellation, cross-Workflow active cards, member-by-member progress, final response rendering, reconnect refresh, mobile layout, and inaccessible controls.
- Reuse prior art from existing Expert Team fail-fast/Workspace rollback tests, Workflow detail and Run polling tests, Run SSE replay tests, Credits repository concurrency tests, Runtime Executor recording fakes, and Worker restart/reconcile tests.
- Run focused Go tests for Workspace domain/application, Credits, Workflow Repository, Worker, Runtime Executor, and transport packages first. Then run the affected full backend test/build gates and frontend typecheck, unit tests, and production build.
- Run Linux + gVisor sandbox and production conformance before claiming deployed Runtime or Connector support. Missing external credentials or unavailable infrastructure is reported as a skipped gate, never as a pass.

## Out of Scope

- Parallel Expert Team fan-out, arbitrary DAGs, dynamic routing, loops, conditions, or free-form inter-agent conversation.
- Runtime-native subagent orchestration or Provider Model/Runtime Engine selection inside an Expert Team.
- A coordinator or extra synthesis Provider Model call after the final Team Member.
- User-level global execution serialization as a product rule.
- Per-User global Workflow queues, cross-Workflow Workspace sharing, or per-Run persistent Workspaces.
- Queue priority, manual insertion, starvation-prevention policy beyond FIFO, or user-configurable queue limits in the first version.
- More than five queued Runs per Workflow in the first version.
- Reusing partial Expert Team results, Native Session state, or Workspace changes after a failed/cancelled team retry.
- Changing the existing Credit Ledger ownership model, Model Credit Rate calculation, Image Credit Reservation semantics, or negative-balance carry-forward rules beyond adding text-stage reservations.
- Provider billing, guaranteed Token reservations, or an assertion that Runtime-reported Usage is conformance evidence.
- Force-killing external providers, blind retries after unknown provider outcomes, or exposing Runtime diagnostics as product controls.
- Commit, Push, Review Branch, pull request, or merge request workflows.

## Further Notes

- The current code already has adjacent seams for ordered execution planning, stage results, Run events, Workflow Run claiming, Credit admission/settlement, and Run SSE. The implementation should deepen those seams rather than introduce a second queue or a Runtime-specific orchestration layer.
- The first implementation must reconcile the current unique User-wide `credit_execution_leases` behavior with the new Workflow-scoped queue and stage reservation model. Leaving the old lease in the claim query would make cross-Workflow concurrency appear accepted while remaining serialized in practice.
- Queue position is informational and race-prone by nature. FIFO execution must rely on transactional claim order and Workflow locking, not on a client-provided or persisted position number.
- A queued Run freezes its execution snapshot before admission, but Credit Reservation is created only when its Stage is about to invoke the Provider Model. This keeps queueing free while preventing unbounded concurrent entitlement use.
- The spec intentionally keeps Session and Workflow terminology separate: Session is a continuing text conversation without a persistent Workspace; Run Conversation is the continuing conversation inside one Workflow. Both are independently serialized within their own scope and may run concurrently across scopes.
- This local spec is ready for implementation triage under the `ready-for-agent` label. No issue-tracker publication was available in the current environment.
