# Connector Platform P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a validated, secure Connector Package boundary and unified P0 lifecycle while preserving current historical MCP/CLI snapshots.

**Architecture:** A new `internal/connectorpackage` package parses and validates the immutable ZIP contract. Workspace domain and repository projections add package/revision/installation/audit records behind the existing application service; the public API exposes unified upload and guided-creation operations while legacy read paths remain compatible. The execution seam normalizes errors and continues to enforce existing CLI wrapper and MCP runtime boundaries.

**Tech Stack:** Go, Kratos Proto/HTTP, GORM/PostgreSQL migrations, objectstore, Vue 3/TypeScript, Vitest.

**Spec:** `docs/product/connector-platform-optimization.md`, `docs/adr/0036-connector-packages-and-unified-lifecycle.md`.

## Global Constraints

- A package contains `connector-meta.json`, `icon.svg`, exactly one of `mcp.json` or `cli.json`, and at least one `skills/*/SKILL.md`.
- ZIP upload and guided creation share the same validator; no shell command strings cross the process boundary.
- P0 stores checksums and performs static safety checks; cryptographic package signatures remain P2.
- User authorization is private and encrypted; credentials never enter logs, snapshots, errors, or audit payloads.
- New revisions are immutable and historical snapshots remain readable.

## Review Focus

- ZIP path traversal, symlink, duplicate, and enclosing-folder inputs: package validator tests reject them.
- MCP+CLI mixed or missing Skill packages: manifest tests reject them.
- CLI lifecycle shell injection: command parser tests require argv arrays.
- Upgrade failure with active authorization: repository tests preserve the old active revision and authorization.
- Disabled or revoked resources during invocation: lifecycle tests fail closed before execution.

### Task 1: Package validator

**Files:**
- Create: `backend/internal/connectorpackage/types.go`, `archive.go`, `manifest.go`, `validator_test.go`
- Modify: `CONTEXT.md`

Implement the immutable package contract, safe ZIP normalization, manifest validation, Skill frontmatter checks, and canonical SHA-256. Keep this package independent of HTTP and GORM.

### Task 2: Domain and persistence projection

**Files:**
- Create: `backend/internal/biz/workspace/domain/connector_package.go`
- Create: `backend/internal/infrastructure/gormdb/migrations/000042_connector_packages.sql`
- Create: `backend/internal/data/workspace/gormrepo/connector_packages.go`
- Test: `backend/internal/data/workspace/gormrepo/connector_packages_test.go`

Add immutable revisions, per-user installations, private authorizations, and redacted audit records. Add transactional activation and rollback helpers without deleting existing tables.

### Task 3: Unified application/API seam

**Files:**
- Modify: `backend/api/workspace/v1/workspace.proto`
- Regenerate: `backend/api/workspace/v1`, `backend/gen/openapi`, `frontend/src/api/generated.d.ts`
- Modify: `backend/internal/service/workspace/service.go`, `connector_packages.go`
- Test: `backend/internal/service/workspace/connector_packages_test.go`

Expose listing, ZIP upload, guided creation, install, status, disable, uninstall, and audit-safe results. Map stable package errors to the existing public error boundary.

### Task 4: Runtime result and policy adapter

**Files:**
- Create: `backend/internal/connectorpackage/result.go`, `result_test.go`
- Modify: `backend/internal/cliconnector/wrapper.go`, `backend/internal/data/workspace/runtimeexecutor/mcp.go`

Normalize MCP/CLI success and failure envelopes and ensure policy/authorization state is revalidated before starting an external call.

### Task 5: Frontend lifecycle flows

**Files:**
- Modify: `frontend/src/pages/SkillsConnectorsPage.vue`, `frontend/src/components/ConnectorDetails.vue`, `frontend/src/api/client.ts`
- Test: relevant Vue component tests

Add generic Connector Package upload and guided creation states, lifecycle status, safe errors, and retry-preserving forms without exposing secrets.

### Task 6: Verification and migration evidence

Run package, domain, repository, service, frontend typecheck/build, and full `make test`/`make build` gates. Document unavailable Linux/gVisor and external-provider evidence honestly.
