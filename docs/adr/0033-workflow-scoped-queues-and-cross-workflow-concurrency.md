---
status: accepted
---

# Use Workflow-scoped queues and cross-Workflow concurrency

This decision supersedes only the User-wide model-invocation serialization in ADR-0024. ADR-0024 remains the authority for the Credit Ledger, post-execution settlement, immutable rate revisions, and negative-balance semantics.

Each Workflow owns a persistent FIFO Workflow Queue. Manual, scheduled, API, follow-up, and rerun requests for that Workflow enter the same queue, including concurrent follow-ups in one Run Conversation. A queued Run receives a stable identity immediately; its turn number is assigned transactionally in enqueue order and is never reused. The queue has a first-version limit of five queued Runs, excluding a Run that is already running or waiting for user action. Queue position is computed from the authoritative order rather than persisted.

The queue is the concurrency boundary for the Workflow's persistent Workspace: only one Run for a Workflow may be running or waiting for user action at a time, and a cancelled queued Run immediately releases its position. Different Workflows, including Workflows owned by the same User, may run concurrently. A Session remains serialized within its own conversation, while different Sessions may also execute concurrently. Worker restart preserves queued order; deleting or disabling a Workflow cancels Runs that have not started while retaining their historical records.

Runtime-backed text invocations no longer use a unique User-wide execution lease. When a Stage starts, Credits atomically creates an Execution Credit Reservation equal to the frozen Model Credit Rate fallback. Expert Team members reserve and settle one at a time. Actual measured usage is settled exactly once and may exceed the reservation under ADR-0024's existing negative-balance rule; a zero fallback creates no reservation. Admission failure prevents that Stage from starting, leaves later Team Members uncharged, and allows unrelated Workflow queues to continue.

An Expert Team remains a platform-managed sequential, fail-fast chain under ADR-0022. The main execution coordinator schedules isolated Subagents in stable Team Member order, passes each member the bounded preceding final results, and publishes structured stage progress without raw reasoning or tool logs. The final member supplies the official response; retry reruns the complete chain and does not reuse partial Native Session or Workspace state.

The API returns `202` and a stable Run ID for accepted queued work. GET and SSE expose the authoritative state (`queued`, `running`, `waiting_for_user`, `succeeded`, `failed`, or `cancelled`) and dynamic queue position. API requests retain `Idempotency-Key` behavior. A full queue returns `429 queue_full` without creating a manual/API Run; a scheduled trigger records a failed history Run with that error.
