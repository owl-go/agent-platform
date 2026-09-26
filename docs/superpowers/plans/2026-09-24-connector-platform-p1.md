# Connector Platform P1 Implementation Plan

**Status:** implemented in `codex/connector-platform-p1`; production Conformance remains environment-gated and cannot make a publication available until exact Linux + `runsc` evidence is attached

**Goal:** Publish and operate the official Feishu CLI as the first real Connector Package, using the P0 package, installation, authorization, audit, and Runtime boundaries instead of maintaining a second mutable CLI lifecycle.

**Architecture:** Add an Administrator-owned publication projection over immutable Connector Revisions. A User installs a published revision into a private Connector Installation. Feishu registration and OAuth remain built-in platform drivers, but their durable identity moves behind the Connector Installation and Authorization repositories. Runtime execution resolves only the frozen revision plus current installation and authorization state. Existing CLI Definition, Enablement, Feishu Application, and account-token rows remain readable during migration and are not deleted.

**Tech stack:** Go, Kratos Proto/HTTP, GORM/PostgreSQL migrations, objectstore, Vue 3/TypeScript, existing CLI builder/Wrapper, Feishu registration/OAuth adapters, Docker + gVisor Production Conformance.

**Product authority:** `docs/product/connector-platform-optimization.md`, `docs/product/expert-skill-connector-simplification.md`, `docs/product/agent-workspace-requirements.md`, and ADR 0036.

## Baseline to reuse

- P0 already validates and stores immutable Connector Packages and exposes per-User installation, authorization, audit, and managed CLI execution seams.
- The existing CLI Connector implementation already provides exact npm builds, bundle integrity, Runtime Digest Conformance, structured capabilities, high-risk one-use approvals, Egress enforcement, output limits, cancellation, and secret redaction.
- The existing Feishu adapters already provide one application per User, device registration, account OAuth, provider-returned application metadata, multiple account authorizations, token refresh/revocation, and User/Bot execution identity.
- P1 must adapt these components to the unified package model. It must not copy the old lifecycle into a third implementation.

## P1 deliverable

1. An Administrator can publish, disable, and supersede an official Connector Package revision.
2. Users can browse published packages without creating an installation or starting authorization.
3. Installing the same published package is idempotent per User and source.
4. The official Feishu package is built from an exact `@larksuite/cli` version, exact bundle SHA-256, and exact Runtime RepoDigest Conformance evidence.
5. Enabling Feishu uses the existing one-application-per-User registration flow and supports multiple isolated account authorizations.
6. User and Bot identity, scope recovery, refresh, revocation, disconnect, high-risk approval, cancellation, and restart recovery operate through the Connector Installation boundary.
7. Upgrades validate a new immutable revision before activation. A failure leaves the old revision and compatible authorizations active. Historical snapshots retain the revision they executed.
8. Existing Feishu users migrate without losing application identity, account authorizations, mutable Expert bindings, or historical snapshots.
9. Aggregate health exposes package/revision/runtime evidence and counts without User credentials or business content.
10. Production evidence records the exact package, bundle, and Runtime Digests on Linux + `runsc`.

## Non-goals

- Cryptographic package signatures and publisher trust chains.
- A public or third-party Marketplace review workflow.
- General external supply-chain malware scanning.
- Arbitrary package-provided authentication code.
- Automatic Feishu application renaming.
- Deleting legacy tables or rewriting historical snapshots during P1.

## Task 1: Publication and authorization persistence

**Files:**

- Create an additive migration after the current latest migration.
- Extend `backend/internal/biz/workspace/domain/connector_package.go`.
- Extend `backend/internal/data/workspace/gormrepo/connector_packages.go` and adjacent tests.

Add a platform publication record keyed by package source with an active immutable revision, state, optimistic version, Administrator identity, and timestamps. Keep publication separate from per-User installation. Model multiple Connector Authorizations per installation and an explicit selected authorization without copying credential bytes into the installation. Add repository transactions for publish, install, select authorization, disconnect, disable, upgrade, and rollback. Preserve the existing audit transaction boundary.

## Task 2: Official package build and validation

**Files:**

- Extend `backend/internal/connectorpackage`.
- Reuse `backend/internal/cliconnector` build and Conformance ports.
- Add an official Feishu package fixture and focused tests.

Extend the CLI package manifest with a built-in authentication-driver declaration restricted to platform-supported drivers. Produce the official Feishu package from the exact reviewed npm artifact and immutable bundle. Carry reviewed capabilities, scopes, identity choices, Egress hosts, resource limits, bundle digest, and Runtime RepoDigests into the package revision. Reject publication if any claimed combination lacks Conformance evidence.

## Task 3: Administrator publication API

**Files:**

- Modify `backend/api/workspace/v1/workspace.proto` and regenerate Go/OpenAPI/TypeScript outputs.
- Extend `backend/internal/service/workspace/connector_packages.go`.
- Add service and repository tests.

Add Administrator-only APIs to list draft and published revisions, publish a validated revision, disable a publication, and inspect aggregate health. Ordinary Users may list only active publications. Responses expose safe metadata and exact public Digests, never object-store URLs, credentials, raw provider responses, or User identities.

## Task 4: User installation and Feishu authorization

**Files:**

- Adapt `backend/internal/feishucli` behind installation-oriented ports.
- Extend Connector Package service and repository methods.
- Add ownership, idempotency, refresh, revocation, and concurrency tests.

Install from a publication rather than accepting a User-supplied official package. Reuse a User's retained Feishu Application and preserve the provider-returned name and developer-console URL. Store multiple account authorizations under one installation, let the User select an account for execution, distinguish User and Bot identity, and request only missing reviewed scopes. Encrypt credentials with installation- and external-identity-bound associated context.

## Task 5: Runtime and approval cutover

**Files:**

- Extend `backend/internal/data/workspace/runtimeexecutor` and `backend/internal/cliconnector` only at their existing seams.
- Extend Worker and Wrapper contract tests.

Freeze the published package revision, bundle digest, Runtime RepoDigest, capability policy, installation identity, and selected authorization identity in the execution snapshot. Immediately before process start, revalidate publication, installation, authorization, scopes, approval nonce/digest, and current policy. Keep one-use high-risk approvals, `waiting_for_user`, cancellation, timeout pausing, restart recovery, Egress, Workspace, output, and redaction behavior unchanged.

## Task 6: Catalog, installation, and recovery UI

**Files:**

- Consolidate `frontend/src/components/ExtensionManager.vue` and `ConnectorPackagePanel.vue` around the publication and installation APIs.
- Update client types, localization, and focused Vue tests.

Present published Connectors once in the Connector catalog. Browsing must not authorize. Install, continue setup, account selection, disconnect, disable, upgrade, and recovery actions must retain form state and display localized safe errors. Show package version, publication state, authorization state, provider application name, developer-console link, account identity, required scope recovery, and Conformance availability on desktop and mobile.

## Task 7: Incremental Feishu migration

**Files:**

- Add a separate additive migration and repository migration tests.
- Update `docs/technical/connector-platform.md` and service architecture notes.

Create an official publication only from a verified available Feishu CLI Definition. Project existing enabled Users into Connector Installations while retaining their Feishu Application and account Authorization identities. Map mutable Expert bindings to the new installation source for future snapshots. Keep legacy snapshot readers unchanged and make the migration restart-safe and idempotent. Do not mark synthetic P0 legacy projections executable.

## Task 8: Verification and production evidence

Run focused package, repository, service, Worker, Wrapper, and Vue tests, followed by `make test`, `make build`, `make web-typecheck`, and `make web-build`. Run Production Conformance on Linux with `runsc` using the exact Feishu bundle and every advertised Runtime RepoDigest. Cover registration idempotency, multiple accounts, User/Bot identity, scope recovery, refresh/revocation, approval/rejection/expiry, cancellation, restart recovery, Egress rejection, timeout, Workspace writes, output bounds, and Secret canaries. Store evidence under `docs/evidence/agent-workspace` with timestamps and Digests. Missing provider credentials or environment gates remain explicitly unverified and cannot make the publication available.

## Completion criteria

- The unified package model is the only mutable source of truth for newly installed official Feishu Connectors.
- Existing Users retain working applications, account authorizations, Expert bindings, and history after migration.
- No production code path installs a package during a User Run or accepts a Shell command string.
- No API, audit record, event, log, snapshot, or Artifact exposes Feishu secrets or raw authorization responses.
- Publication availability is backed by exact package, bundle, and Runtime Digest evidence.
- All applicable repository gates pass, and unavailable external evidence is reported without being counted as passing.
