# 16 — Harden revocation, waiting, and recovery

**What to build:** Keep Connector execution safe when authorization or platform state changes during a wait, and recover deterministic state across asynchronous triggers and Worker restarts.

**Blocked by:** 12 — Support high-risk command approvals and User Action Waits; 15 — Execute Feishu operations as User or Bot.

**Status:** ready-for-agent

- [ ] Disabling a CLI Definition or User Enablement, disconnecting an Authorization, or disabling the owning User closes pending and approved-but-not-started commands immediately.
- [ ] The Wrapper rechecks Definition, bundle/Runtime availability, Enablement, Authorization, identity, scopes, capability policy, and approval immediately before every process start.
- [ ] A stale approval cannot execute after revocation, policy revision, argument change, expiry, cancellation, or another Worker consuming its nonce.
- [ ] Scheduled and Workflow API Runs may enter `waiting_for_user`, but only the owning User in the authenticated product can approve before the configured deadline.
- [ ] Worker restart and reconciliation reconstruct waiting state only from persisted requests and decisions, without repeating a completed external command or creating a second active request.
- [ ] Workflow serialization, temporary Workspace, Runtime container, Credit lease heartbeat, and ordinary-timeout pause remain correct throughout a wait and release on every exit path.
- [ ] Cancellation and terminal transitions stop the process group, clean current Secrets, close queued approvals, and publish no events after the single terminal event.
- [ ] Rejection and expiry remain structured tool errors so the Runtime can recover or offer an alternative; already measured model Usage settles exactly once.
- [ ] Administrator health views expose only aggregate Connector state and never User identity, credentials, arguments, targets, or execution content.
- [ ] Race, failover, scheduled/API, disabled-User, lease, duplicate-delivery, terminal-event, cleanup, and aggregate-privacy tests cover all recovery paths.
