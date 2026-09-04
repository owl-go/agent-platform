---
status: accepted
---

# Experts own Provider Model and Runtime Engine selection

Every Expert requires its own Provider Model and Runtime Engine, and both choices are frozen into each new Session or Run Conversation's Expert Snapshot. A single Expert executes with its model and engine, while an Expert Team may mix models and engines by running each member as an isolated Runtime Execution; the team has no model or engine override. Sessions and Workflows do not select or persist model or engine overrides: execution without an Expert resolves the current Personal Settings defaults at the Response or Run Conversation boundary. This supersedes ADR-0022's requirements that all team members share one Provider Model and Runtime Engine. The team's overall timeout, fail-fast order, final-member response, and Workflow temporary Workspace remain shared.

Historical snapshots without member engines retain their original shared-engine semantics instead of being reinterpreted from mutable Expert definitions. New team turns stage each member's Native Session state independently and promote all member states only after the complete turn succeeds, so cancellation, failure, and retry cannot leave partial conversation continuity. Warm Runtime reuse is scoped by conversation, frozen Expert identity, and Runtime Engine; it does not let team members share an execution context.

New Response and Workflow Snapshots represent every actual model invocation as an ordered Execution Stage Snapshot containing its resolved Provider Model and Runtime Engine. Execution without an Expert has one anonymous stage, a single Expert has one Expert stage, and an Expert Team has one stage per member; mixed teams have no synthetic top-level model or engine. Stage identity is retained in history and Workflow API results for audit and failure diagnosis.

Legacy mutable Session and Workflow model or engine overrides are removed and ignored for new execution after migration; only already-frozen snapshots retain their old shared configuration. A complete Expert whose selected model, connection, engine, or compatibility becomes unavailable remains editable but cannot start new execution, and execution never silently falls back to another model or engine.

Saving an Expert permits verified and warned unverified model-engine pairs but rejects incompatible pairs. A Model Provider Connection cannot be deleted while referenced by Personal Settings, a mutable Expert, or a continuable frozen conversation. Manual and API Workflow requests fail before creating a Run when their Expert is unavailable, while a Scheduled Trigger creates an explicit uncharged failed Run; queued work revalidates before its first stage.

Each team stage checks Credit Balance immediately before invoking its Provider Model and settles its own frozen Model Credit Rate. Insufficient Credits before a later member fails the whole turn without starting that member; completed stages remain charged, while temporary Workspace changes and all staged Native Session state are discarded.
