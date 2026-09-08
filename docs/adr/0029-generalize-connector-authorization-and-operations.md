---
status: accepted
---

# Generalize Connector authorization and operations

CLI and MCP Connectors share a versioned Connector Manifest, Permission catalog, Authorization Scheme, Action Requirement, Connector Operation, Operation Approval, and audit contract. Transport adapters retain their protocol-specific execution, while provider authorization logic is registered as trusted platform code. Connector packages may reference an Authorization Scheme but cannot provide executable authorization code, credential scripts, or frontend components.

Connector creation or discovery produces a Manifest Draft. An exact-version platform Profile is authoritative when present; otherwise a package Manifest may supply the declaration. Schema validation, trusted-reference validation, exact-version transport Conformance, and Administrator review freeze a Resolved Connector Manifest. README content remains human-facing explanation and cannot define execution or security policy. A selected Connector exposes a compact Capability Index; the Agent reads the frozen Capability Contract and reviewed Usage Guide through a platform-local description operation and invokes capabilities with structured input.

Connector Enablement, Connector Setup, Connector Authorization, and Connector Operation Approval have separate lifecycles. Selecting or opening a Connector never begins Setup or Authorization. The first protected Connector Operation with missing requirements creates a typed, time-bounded Action Requirement in that conversation. Provider verification resumes the same execution without another model invocation. Each User and Connector has at most one active Authorization, and Setup or Authorization credentials never cross Connector boundaries. High-risk CLI and MCP operations require a one-use Approval after the external identity is established.

The Connector Broker revalidates the frozen definition, enablement, Setup, Authorization, Permissions, risk, approval, Egress, input, and execution limits immediately before transport invocation. CLI Wrapper and MCP Gateway emit typed business operation events rather than relying on Runtime command text. Recovery follows the frozen capability's idempotency policy; a write whose external result cannot be established becomes `outcome_unknown` rather than being retried blindly. Audit records contain only minimal structured, Secret-redacted metadata.

This decision supersedes the Feishu-specific rules that enablement initiates provider application setup and that one User and Connector may retain multiple active account authorizations. Feishu becomes the first registered Authorization Adapter and exact-version Profile. Existing data migrates through an expand-contract sequence; records that cannot be proven safe require reauthorization. Static MCP transport Secrets retain the ownership rules accepted in ADR-0028 and are distinct from per-User external account Authorization.
