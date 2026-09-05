# 12 — Support high-risk command approvals and User Action Waits

**What to build:** Pause a Session response or Run before each high-risk CLI command, obtain precise one-use consent from the owning User, and resume the same isolated execution safely.

**Blocked by:** 11 — Execute low-risk CLI commands through the common Wrapper.

**Status:** ready-for-agent

- [ ] A high-risk command persists an approval request before exposing it and transitions the Session response or Run to `waiting_for_user` without publishing a terminal event.
- [ ] The approval displays Connector, identity, operation, target, and redacted arguments and is bound to a nonce plus an immutable digest of executable, argv, target, policy version, and expiry.
- [ ] Only the authenticated owning User can approve or reject; Administrator identity, another User, and Workflow API credentials cannot inspect or decide the request.
- [ ] One approval authorizes one matching command start and cannot be replayed, reused for changed arguments, or converted into a permanent grant.
- [ ] Each Execution Stage exposes only one active request while additional high-risk commands queue, and one execution may complete multiple sequential approvals.
- [ ] Waiting retains the Runtime container, temporary Workspace, Workflow serialization lock, and Credit lease, accepts cancellation, and pauses the ordinary execution timeout.
- [ ] The approval deadline defaults to five minutes, cannot exceed fifteen minutes, and may be lowered by Administrator policy.
- [ ] Rejection or expiry returns a structured CLI error to the Runtime without forcing the whole Run to fail; model Usage already incurred remains chargeable.
- [ ] Approval is consumed and all current policy and authorization state is revalidated immediately before process start.
- [ ] State-transition, race, replay, timeout, cancellation, event-ordering, credit, API-authority, Worker, and responsive approval UI tests cover the complete flow.
