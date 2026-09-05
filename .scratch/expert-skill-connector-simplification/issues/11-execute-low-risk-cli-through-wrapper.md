# 11 — Execute low-risk CLI commands through the common Wrapper

**What to build:** Let a User enable, select, and safely use an available low-risk CLI Connector through one platform enforcement point shared by every Runtime Engine.

**Blocked by:** 05 — Compose Experts with Skills and MCP Connectors; 10 — Build and publish immutable CLI bundles.

**Status:** ready-for-agent

- [ ] Users can browse available CLI Connectors without authorization and enable one for their own account; ordinary Users cannot alter its Definition.
- [ ] An enabled CLI Connector can be selected for an Expert, and a new conversation snapshot freezes its Definition version, bundle digest, capability policy, and Authorization identity.
- [ ] App Secrets and Tokens never enter ordinary snapshots; current encrypted credentials are resolved only for the individual command invocation.
- [ ] Every direct CLI invocation passes through one common Wrapper rather than Runtime-specific enforcement code.
- [ ] The Wrapper verifies exact bundle and Runtime Digests, executable, identity, argv, capability, scope, Egress, timeout, output, and Workspace policy before starting a process.
- [ ] External commands use executable and argument arrays, never a concatenated Shell command, and cannot mutate approved arguments after validation.
- [ ] Invocation Secrets are materialized with restrictive permissions, added to exact-byte redaction, and cleaned idempotently on success, failure, timeout, and cancellation.
- [ ] A harmless fixture command works end to end through each claimed Runtime seam; rejected inputs never start the underlying process.
- [ ] Recommended Skills appear as explicit install offers and are never installed or injected automatically.
- [ ] Wrapper contract, snapshot, ownership, Runtime fake, Secret canary, Workspace, Egress, output-limit, cancellation, API, and UI tests cover the flow.
