---
status: accepted
---

# Keep Connector Authorization generic and adapt provider protocols at one seam

The P1 Feishu implementation reused the common Connector Installation and
Authorization records, but its interactive setup, device flow, refresh, and
Runtime credential materialization were wired directly into the platform's
authorization entry points. A second CLI such as DingTalk can therefore pass
package validation yet cannot obtain or use a User Authorization. Treating a
generic JSON credential as a DingTalk CLI login would report a false success.

The platform owns the common lifecycle: identify the active immutable Connector
Revision, authorize the owning User, validate requested scopes, create an
expiring flow, encrypt and select an Authorization, audit changes, rotate or
revoke credentials, and revalidate the frozen revision and Authorization before
each command. A reviewed authentication driver supplies the provider-specific
protocol: whether setup is needed, how to create an authorization action URL,
how to complete or refresh a grant, and how that grant is materialized for one
Runtime invocation. Driver selection comes from the validated revision policy,
never from a caller-supplied provider name or URL. An unavailable driver fails
before contacting a provider or marking an Installation authorized.
The existing provided-credentials path remains available only for a reviewed
`connector_package` CLI or authenticated MCP revision; it cannot synthesize a
provider-managed OAuth grant or assert scopes absent from the revision policy.

Credential delivery is part of the driver contract, not an assumption that
every CLI reads `CONNECTOR_CREDENTIALS_JSON`. An OAuth token can be delivered
through a reviewed environment mapping when the CLI explicitly supports it.
A CLI that owns an encrypted local profile needs a reviewed import/export
adapter that restores it inside a short-lived isolated command and returns
rotated refresh material to encrypted platform storage. Secret bytes must not
appear in argv, URLs, logs, audit records, model-visible output, or a
persistent Workspace. Refresh and profile update must be serialized per
Authorization so two commands cannot reuse a rotated refresh token.

Feishu remains a built-in adapter during migration. DingTalk requires its own
adapter for its device login and encrypted DWS profile; the platform lifecycle
and User-facing authorization state must be the same. Built-in adapters are
reviewed platform code, while package manifests can only choose allowlisted
drivers. A package's lifecycle argv and Skill do not themselves confer an
authorization driver. Do not publish a Connector Revision as usable until its
driver, exact bundle/Runtime Conformance, authorization completion, and at
least one real invocation have been verified.

We reject arbitrary OAuth URLs and token-response templates supplied by ZIPs:
they would let an uploaded package redirect User grants or platform credentials
to an attacker-controlled endpoint. We also reject another provider-specific
branch in the shared authorization entry points, because it would duplicate
expiry, ownership, audit, refresh, and revocation rules for each Connector.

This decision extends ADR 0036. Its implementation is incremental; the
existing Feishu path is the only interactive adapter until another driver and
its credential delivery have passed the same checks.
