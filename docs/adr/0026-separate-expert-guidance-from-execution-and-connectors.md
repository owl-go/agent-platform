---
status: accepted
---

# Separate Expert guidance from execution configuration and Connectors

Experts describe reusable specialist guidance through Core Capability, Operating Procedure, Output Standard, and optional Cautions; they no longer own a Provider Model or Runtime Engine. Personal Settings supplies one execution configuration that is frozen when a Session or Run Conversation starts and shared by an anonymous stage, a single Expert, or every ordered Team Member. This supersedes ADR-0023, restores ADR-0022's shared execution configuration, and supersedes ADR-0025's mutable-Expert connection reference, while retaining sequential fail-fast team execution, isolated member context, final-member response, and success-only Workspace merge.

Skills and Connectors are independent catalog resources selected by Experts. A User owns private Skills and MCP Connectors, while an Administrator owns platform-wide CLI Connector Definitions and Users privately own their enablements, external account authorizations, and approvals. Runtime execution may use only frozen, verified Connector revisions; CLI bundles are built outside User Runs and every direct command passes through one platform Wrapper that enforces structured capability, identity, argument, Egress, authorization, approval, timeout, and Secret boundaries.

High-risk CLI commands require a persisted, one-use decision from the authenticated owning User. Execution may enter `waiting_for_user`, retain its isolated Runtime and temporary Workspace, pause its ordinary execution timeout, and then resume or receive a structured rejection or expiry error; Administrator identity and Workflow API credentials cannot approve. Historical snapshots retain their original Expert-owned execution semantics and are not rewritten.
