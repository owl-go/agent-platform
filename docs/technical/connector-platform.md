# Connector Platform Contract

## Package validation

The package validator is the single boundary shared by ZIP upload and guided creation. It rejects empty or oversized archives, absolute or traversal paths, duplicate paths, symbolic links, hard-coded credential names or values, malformed JSON, missing required files, invalid semantic versions, and packages that declare both MCP and CLI manifests. A package must contain at least one `skills/<name>/SKILL.md`; every document must be UTF-8 with valid frontmatter and all referenced resources must stay inside that Skill directory.

`connector-meta.json` owns the globally unique lower-case `source`, semantic `version`, package `type`, user-facing name and description, examples, platform compatibility, and `auth_mode`. `mcp.json` declares one remote HTTPS or local fixed-runtime server. `cli.json` declares the managed runtime, executable, structured lifecycle argv arrays, status matching, authorization-domain allowlist, reviewed capabilities, and optional resource limits. A CLI package may carry `cli-bundle.tgz`; its executable path, expanded tar contents, mode bits, and SHA-256 are verified before object storage. No manifest accepts a shell command string.

## Lifecycle

Connector Revision, Installation, and Authorization are separate state machines. The active revision is changed only after validation. An invocation requires an available revision, an installed package, an active authorization when the package requires one, current policy, and a runtime that matches the declared digest and limits. Managed CLI packages are projected into the same catalog as legacy definitions, but selection freezes the verified bundle, capability policy, runtime digest, and installation authorization. Disablement and revocation fail closed before a new external process or request is started. CLI credentials are materialized only for one broker command as `CONNECTOR_CREDENTIALS_JSON`, and broker output is redacted before it crosses the Runtime boundary. MCP HTTP targets are checked against the declared Egress host list during materialization and isolated tests.

P1 adds a platform Publication projection above immutable revisions. A Publication selects one active revision for a package source and records its available or disabled state, the Administrator that changed it, an optimistic version, and update time. Publications never own User credentials. A User Installation selects one published revision and one current Authorization while retaining other account Authorizations for explicit selection. Creating or selecting another Authorization does not revoke the remaining accounts; disconnect and revocation operate on the targeted Authorization and clear the selected reference only when it points to that record.

CLI package manifests may select only a built-in authentication driver: `none`, `connector_package`, or `feishu`. Packages without the field retain the P0 derivation from `auth_mode`. The Feishu driver is platform code and cannot be supplied by a package.

## Result and audit boundary

The execution plane returns `{ok,data,request_id,warnings}` on success and `{ok:false,error:{type,message,retryable,next_action},request_id}` on failure. It redacts exact credential bytes before persistence. Audit records contain actor, Connector, revision, mode, operation, risk, identity reference, timing, outcome, error type, request ID, policy revision, and approval reference. They never contain tokens, secrets, authorization URLs, raw stdout/stderr, full arguments, or business results.

## Migration

Migration `000042_connector_packages.sql` creates package, revision, installation, authorization, and audit projections without deleting existing MCP/CLI tables. Migration `000043_connector_package_backfill.sql` maps existing tested MCP servers and enabled CLI definitions to synthetic revisions marked `legacy_projection`; these rows preserve identity and audit continuity but are excluded from the managed runtime catalog. New writes use the unified seam; old snapshot columns remain readable until all historical consumers are migrated.

Migration `000053_connector_package_publications.sql` adds the Administrator-owned Publication projection and indexes installation authorization state. It does not publish a synthetic legacy projection or delete old CLI records. A later P1 migration may activate the official Feishu package only after its exact bundle and Runtime Digests have passed Production Conformance.
