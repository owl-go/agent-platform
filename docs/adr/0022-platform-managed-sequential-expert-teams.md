---
status: accepted
---

# Platform-managed sequential Expert Teams

The shared Provider Model and Runtime Engine portions of this decision are superseded by ADR-0023; the platform-managed order, fail-fast behavior, timeout, result handoff, and shared Workflow temporary Workspace remain accepted.

Expert Teams execute as a platform-managed, fixed-order chain: every Expert runs in an independent process and context, receives the current task plus bounded conversation context and preceding member results, and the last member produces the official response. The platform owns this contract because Claude Code, Codex, Hermes, OpenClaw, and PI Agent do not provide one common, conformance-proven native subagent model; relying on individual CLI orchestration would make product behavior vary by Runtime Engine.

The chain is deliberately sequential, fail-fast, and bounded to two through ten distinct Experts sharing one Provider Model, Runtime Engine, overall timeout, and Workflow temporary Workspace. We reject parallel fan-out, arbitrary graphs, dynamic routing, per-member models, extra coordinator synthesis, and Runtime-native subagents in the first version to keep cancellation, retry, snapshots, cost, Workspace rollback, and history deterministic.
