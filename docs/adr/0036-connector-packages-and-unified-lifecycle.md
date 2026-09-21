---
status: accepted
---

# Use one validated Connector Package and separate installation from authorization

Agent Workspace distributes external integrations as immutable, versioned Connector Packages. A package contains `connector-meta.json`, `icon.svg`, exactly one of `mcp.json` or `cli.json`, and at least one Skill directory with a required `SKILL.md`. ZIP upload and guided creation are two input paths for the same package contract; neither path may bypass validation.

MCP and CLI are mutually exclusive package modes. MCP packages use the supported Streamable HTTP or fixed-version `npx`/`uvx` stdio transports. CLI packages use platform-managed runtimes and structured argv arrays for `init`, `auth`, `status`, and `unAuth`. The execution plane normalizes both modes to one result and error envelope.

The package revision, per-user installation, and per-user authorization are separate records. Installation makes a validated revision available, while authorization grants access to an external identity. Platform-owned encrypted credential storage, redaction, status checks, and pre-invocation policy validation apply to both modes. A User cannot share an authorization with another User; Administrators publish governed platform packages but cannot read User credentials.

Revisions are immutable. An upgrade installs and validates a new revision before switching the active revision, and a failed upgrade rolls back without changing compatible authorization state. New executions resolve the active revision, while historical snapshots retain their original revision. Uninstall revokes credentials, stops new calls, removes mutable bindings and package files, and retains only the audit and historical identity needed for reproducibility.

This decision replaces the assumption that MCP and CLI definitions have unrelated package and lifecycle contracts. Existing MCP Server and CLI Connector Definition records are migrated incrementally into package/revision projections; legacy snapshot readers remain read-only for historical data.
