# Connector Platform Optimization

Status: accepted for P0 implementation

## Product contract

The platform treats a Connector as a versioned external integration package. Every package contains `connector-meta.json`, `icon.svg`, exactly one mode manifest (`mcp.json` or `cli.json`), and at least one Skill directory with `SKILL.md`. Users may upload a ZIP or complete a guided creation flow; both paths produce the same validated package.

MCP and CLI are exclusive modes. MCP supports Streamable HTTP and fixed-version `npx`/`uvx` stdio. CLI uses a platform-managed Node.js or Python runtime and structured argv arrays for lifecycle commands. `skill-only` remains a standalone Skill concept and is not a Connector mode.

P0 separates installation, connection, and authorization. Package revisions are immutable, upgrades are validated before activation, failures roll back, and historical snapshots retain their original revision. Platform encrypted storage owns credentials for both modes. No authorization is shared across Users.

Connector and capability risk declarations are advisory inputs to platform policy. The platform may raise risk, and every high-risk command requires a one-use, time-bounded User approval bound to an immutable command digest and nonce. Skill instructions cannot approve or bypass that policy.

The P0 deliverable includes package validation, safe ZIP handling, a unified domain and API seam, lifecycle and audit persistence, upload and guided-creation UI, and generic MCP/CLI fixtures. It does not ship a real third-party Connector. Package signatures, trust chains, marketplace review, and external supply-chain scanning remain P2 work.
