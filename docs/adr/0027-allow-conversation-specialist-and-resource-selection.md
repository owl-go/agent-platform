---
status: accepted
---

# Allow conversation specialist and resource selection

Session messages and Workflow Run Conversation follow-ups may select Skills and Connectors directly and may change their Expert or Expert Team. The specialist and Connector selections persist for subsequent messages; explicitly selected Skills apply only to the message being sent. Each execution preserves its actual selection so changing the next message cannot rewrite historical execution or its retry configuration. This changes the conversation-wide specialist and resource freezing boundary in ADR-0026 while retaining frozen Personal Settings execution configuration, immutable historical snapshots, platform-managed sequential team execution, and Connector authorization enforcement.

This accepts additional per-turn snapshot and selection complexity so Users can bring a catalog resource into an existing conversation without creating a new conversation or modifying a reusable Expert. Workflow follow-up selections are scoped to that Run Conversation; the initial Workflow execution and its goal editor are outside this interaction change. Retained selections keep their exact revisions until explicit reselection, and each execution preserves the merged resource selection without changing reusable Expert definitions. Explicit additions reach every Team Member while inherited defaults remain member-specific.

File References preserve the content accepted with the message rather than reading a mutable Workspace path at eventual execution time. Saving an immutable copy costs storage but preserves the User's input across queued Workspace changes and source Artifact expiry. This freezes referenced inputs, while the Workflow continues to operate on its normal temporary Workspace and success-only merge.

The detailed behavior, revision precedence, and implementation evidence are recorded in `docs/product/conversation-resource-selection.md`. Dynamic selections use platform history and resource-specific warm container identities; native Resume is disabled for these turns until changing-resource Resume has its own conformance evidence.
