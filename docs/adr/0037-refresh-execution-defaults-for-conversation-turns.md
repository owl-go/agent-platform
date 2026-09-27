---
status: accepted
---

# Refresh execution defaults for each conversation turn

A User changing the default Runtime Engine or Provider Model expects the next message in an existing Session or Run Conversation to use that choice. Freezing Personal Settings at conversation creation made old conversations keep invoking an engine and model the User had already replaced.

Each new Session response, Workflow follow-up Run, and historical Workflow rerun resolves the current Personal Settings Runtime Engine and that engine's default Provider Model when the turn is submitted. The turn freezes its resolved Execution Stage Snapshots, including the model connection version, protocol, and Credit Rate. Retained specialist and resource revisions still follow Conversation Selection rules. A Workflow follow-up retains its initiating goal, environment, and Workspace context; a rerun retains its source Run inputs and Workflow context while starting a new Run Conversation. Initial manual, scheduled, and API Runs already resolve the current settings.

Historical Response Snapshots and Workflow Snapshots stay immutable. Retrying a Session response uses its original Response Snapshot. Queued turns keep the configuration accepted at submission even if settings change before the Worker claims them. A current default that is missing, unavailable, or incompatible fails the new submission explicitly; it does not silently use the previous turn's model.

Native Session checkpoints may be reused only when the previous successful turn used the same Runtime Engine in every corresponding stage. A Runtime Engine change starts from platform-owned history and Rolling Summary. A model-only change within one Runtime Engine may continue through that Runtime's verified native Resume behavior.

This supersedes the conversation-wide Personal Settings freezing rule in ADR-0026 and ADR-0027 and the corresponding language in ADR-0017. Their historical snapshot, resource-selection, and platform-history boundaries remain in effect.
