# Connector Platform Contract

## Package validation

The package validator is the single boundary shared by ZIP upload and guided creation. It rejects empty or oversized archives, absolute or traversal paths, duplicate paths, symbolic links, hard-coded credential names or values, malformed JSON, missing required files, invalid semantic versions, and packages that declare both MCP and CLI manifests. A package must contain at least one `skills/<name>/SKILL.md`; every document must be UTF-8 with valid frontmatter and all referenced resources must stay inside that Skill directory.

`connector-meta.json` owns the globally unique lower-case `source`, semantic `version`, package `type`, user-facing name and description, examples, platform compatibility, and `auth_mode`. `mcp.json` declares one remote HTTPS or local fixed-runtime server. `cli.json` declares the managed runtime, executable, structured lifecycle argv arrays, status matching, and authorization-domain allowlist. No manifest accepts a shell command string.

## Lifecycle

Connector Revision, Installation, and Authorization are separate state machines. The active revision is changed only after validation. An invocation requires an available revision, an installed package, an active authorization when the package requires one, current policy, and a runtime that matches the declared digest and limits. Disablement and revocation fail closed before a new external process or request is started.

## Result and audit boundary

The execution plane returns `{ok,data,request_id,warnings}` on success and `{ok:false,error:{type,message,retryable,next_action},request_id}` on failure. It redacts exact credential bytes before persistence. Audit records contain actor, Connector, revision, mode, operation, risk, identity reference, timing, outcome, error type, request ID, policy revision, and approval reference. They never contain tokens, secrets, authorization URLs, raw stdout/stderr, full arguments, or business results.

## Migration

The first migration creates package, revision, installation, authorization, and audit projections without deleting existing MCP/CLI tables. A backfill maps existing available resources to synthetic package revisions. New writes use the unified seam; old snapshot columns remain readable until all historical consumers are migrated.
