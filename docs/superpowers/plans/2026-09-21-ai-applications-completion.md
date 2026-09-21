# AI Applications Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task in the current worktree.

**Goal:** Complete the first-version AI Applications acceptance boundary while reusing the existing Session, Worker, credit, knowledge, and redaction contracts.

**Architecture:** Keep AI Application configuration and immutable snapshots in `backend/internal/biz/aiapplication`; expose owner-scoped operations through the existing workspace service; route all model-backed answers through the existing Session/Worker execution path. Add only the lifecycle and snapshot seams required by the product requirements; no second Runtime or model queue.

**Tech Stack:** Go, Kratos HTTP handlers, GORM/PostgreSQL migrations, Vue 3 + TypeScript + Element Plus, Vitest, existing Agent Workspace Runtime and credit services.

**Spec:** `docs/superpowers/specs/2026-09-21-ai-applications-completion-design.md`

## Global Constraints

- Session and Workflow execution remain the source of truth for admission, credits, cancellation, event ordering, history, and redaction.
- User-owned Smart Assistants and Digital Humans are owner-scoped; administrators cannot read or mutate user-owned application content.
- `/ai-apps` is canonical and `/ai-creation/image-generation` redirects to `/ai-apps/image-creation`.
- Direct FAQ answers do not invoke a model or consume credits; model-backed answers use the existing Session path.
- Secrets and provider responses never enter ordinary browser payloads, snapshots, logs, or artifacts.
- Do not merge or delete the ongoing `codex/connector-platform-p0` branch.

## Review Focus

- Stale version or owner mismatch must return a structured conflict without mutating an application.
- Copy and delete must not leak or reuse share tokens, Session IDs, external conversation IDs, or provider secrets.
- Disabled or incomplete resources must remain editable but cannot start new conversations.
- A failed ingestion or embedding generation must leave the previous Ready generation serving retrieval.
- Public iframe requests must not expose internal Session IDs, Knowledge Base internals, signed URLs, or credentials.

### Task 1: Canonical routes and lifecycle API contracts

**Files:**
- Modify: `frontend/src/router.ts`
- Modify: `frontend/src/router.test.ts` or create route-focused test beside the router
- Modify: `backend/internal/biz/aiapplication/domain/model.go`
- Modify: `backend/internal/service/workspace/ai_applications.go`
- Test: `backend/internal/biz/aiapplication/domain/model_test.go`
- Test: `backend/internal/service/workspace/*_test.go`

**Interfaces:**
- Add canonical route names and a legacy redirect without changing existing image-generation APIs.
- Add validated lifecycle inputs for Smart Assistant and Digital Human actions; HTTP handlers map `GET`, `POST`, `PATCH`, and `DELETE` to owner-scoped application service methods and preserve structured error codes.

- [ ] Write failing route tests for the legacy redirect, canonical child routes, and duplicate-route absence.
- [ ] Write failing domain tests for documented scenario values, incomplete assistants, copy token omission, and lifecycle state transitions.
- [ ] Implement the route and domain/API contract changes.
- [ ] Run `pnpm --dir frontend test --run` and targeted Go tests; expect all existing tests plus the new cases to pass.
- [ ] Commit as `feat: close ai application route and lifecycle contracts`.

### Task 2: Smart Assistant lifecycle and editor closure

**Files:**
- Modify: `backend/internal/biz/aiapplication/application/service.go`
- Modify: `backend/internal/data/aiapplication/gormrepo/repository.go`
- Modify: `backend/internal/infrastructure/gormdb/migrations/`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/pages/SmartAssistantsPage.vue`
- Modify: `frontend/src/pages/SmartAssistantDetailPage.vue`
- Test: `backend/internal/biz/aiapplication/application/*_test.go`
- Test: `backend/internal/data/aiapplication/gormrepo/*_test.go`
- Test: `frontend/src/pages/*Assistant*.test.ts`

**Interfaces:**
- `ListAssistants(ctx, owner, filter)` supports name and scenario filtering.
- `CopyAssistant(ctx, owner, id)`, `SetAssistantState(ctx, owner, id, state, version)`, and `DeleteAssistant(ctx, owner, id, confirmation)` preserve snapshot history.
- The frontend exposes search, scenario selection, copy, enable/disable, explicit delete confirmation, and all visible configuration fields.

- [ ] Add failing application and repository tests for copy isolation, owner checks, stale versions, enable validation, and filter behavior.
- [ ] Implement the service/repository operations and migration only if the schema needs state/index changes.
- [ ] Add HTTP mapping and frontend API methods.
- [ ] Implement accessible list/editor controls and FAQ edit/order/publication controls.
- [ ] Run targeted Go and Vitest tests, then the frontend typecheck.
- [ ] Commit as `feat: complete smart assistant lifecycle`.

### Task 3: Digital Human lifecycle, preview, and reference conflicts

**Files:**
- Modify: `backend/internal/biz/aiapplication/domain/model.go`
- Modify: `backend/internal/biz/aiapplication/application/service.go`
- Modify: `backend/internal/data/aiapplication/gormrepo/repository.go`
- Modify: `backend/internal/service/workspace/ai_applications.go`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/pages/DigitalHumansPage.vue`
- Modify: `frontend/src/pages/DigitalHumanDetailPage.vue`
- Test: domain, application, repository, HTTP, and Vue tests adjacent to the changed files

**Interfaces:**
- `CopyDigitalHuman`, `SetDigitalHumanState`, `PreviewDigitalHuman`, and `DeleteDigitalHuman(..., detach bool)` return deterministic validation or explicit reference conflicts.
- Assistant creation/update rejects disabled Digital Humans.

- [ ] Add failing tests for state validation, preview side-effect absence, copy isolation, disabled selection, and referenced delete conflict.
- [ ] Implement persistence, application validation, HTTP actions, and frontend controls.
- [ ] Run targeted backend/frontend tests and verify no provider secret is serialized.
- [ ] Commit as `feat: complete digital human lifecycle`.

### Task 4: Knowledge ingestion generations and citation boundary

**Files:**
- Modify: `backend/internal/biz/aiapplication/domain/knowledge.go`
- Modify: `backend/internal/biz/aiapplication/application/knowledge.go`
- Modify: `backend/internal/data/aiapplication/gormrepo/knowledge.go`
- Modify: `backend/internal/infrastructure/gormdb/migrations/`
- Modify: worker wiring and ingestion processor files under `backend/internal/knowledgebase/`
- Test: knowledge domain, application, repository, worker, and conformance tests

**Interfaces:**
- Accepted documents create durable Processing state; a worker transitions them to Ready or Failed and supports idempotent retry.
- Embedding generations activate atomically; Search returns bounded citation metadata and retains the previous Ready generation after a failed rebuild.

- [ ] Add failing tests for processing/retry transitions, blocked safety content, generation activation rollback, and citation bounds.
- [ ] Implement the durable ingestion job and generation tables through immutable migrations.
- [ ] Wire the worker and retrieval provider without changing existing Workflow knowledge selection behavior.
- [ ] Run targeted knowledge tests, `make test`, and migration tests.
- [ ] Commit as `feat: complete ai application knowledge ingestion`.

### Task 5: Assistant Session execution and snapshot isolation

**Files:**
- Modify: `backend/internal/biz/aiapplication/application/answer.go`
- Modify: `backend/internal/biz/workspace/application/`
- Modify: `backend/internal/data/workspace/gormrepo/`
- Modify: `backend/internal/service/workspace/ai_applications.go`
- Modify: `frontend/src/pages/SmartAssistantDetailPage.vue` and session navigation components
- Test: assistant application, session application, repository, service, and frontend tests

**Interfaces:**
- A new assistant conversation creates a normal Session carrying an immutable assistant binding; accepted messages use the existing model execution path.
- FAQ responses terminate before credit admission; grounded answers add bounded retrieval context to the existing execution request; no-grounding returns the localized refusal.

- [ ] Add failing tests proving FAQ no-credit behavior, model-backed execution admission, snapshot isolation after edits, cancellation propagation, and event/history compatibility.
- [ ] Implement the binding and execution context seam at the existing Session boundary.
- [ ] Remove the current behavior that returns raw grounded chunks as the final answer.
- [ ] Run targeted execution tests plus `make test` and `make build`.
- [ ] Commit as `feat: execute smart assistant conversations through sessions`.

### Task 6: Public share and iframe execution

**Files:**
- Modify: `backend/internal/service/workspace/public_assistant.go`
- Modify: `backend/internal/biz/aiapplication/application/answer.go`
- Modify: `backend/internal/data/aiapplication/gormrepo/`
- Modify: `frontend/src/pages/SmartAssistantDetailPage.vue`
- Test: public assistant HTTP/service tests and iframe/browser tests

**Interfaces:**
- Public FAQ selection remains synchronous and free; free-text requests create isolated External Conversations and poll worker-backed responses.
- Token rotation, disable, origin, size, rate, owner-credit, and payload redaction behavior are all tested.

- [ ] Add failing HTTP tests for token revocation, origin/size validation, public payload redaction, FAQ no-credit, and external conversation isolation.
- [ ] Implement the shared execution path and polling/history behavior.
- [ ] Verify CSP and iframe behavior with a browser-to-API test where the environment supports it.
- [ ] Commit as `feat: close smart assistant public sharing`.

### Task 7: Image Creation placement and acceptance evidence

**Files:**
- Modify: `frontend/src/router.ts`, `frontend/src/App.vue`, and image page tests only where required
- Test: route, image page, and browser acceptance tests
- Docs: `docs/technical/production-conformance.md` or the applicable acceptance evidence file

- [ ] Add failing tests for legacy redirect, canonical label, ownership, history, cancellation, regeneration, and credit settlement coverage.
- [ ] Implement only the missing route/placement behavior; preserve existing image domain APIs.
- [ ] Run all backend/frontend tests, typecheck, production builds, and browser-to-API acceptance.
- [ ] Commit as `test: close ai applications acceptance coverage`.

### Task 8: Integrate, deploy, and final audit

**Files:**
- No feature files unless verification exposes a defect.
- Modify: `docs/technical/production-conformance.md` with real evidence only.

- [ ] Run `make test`, `make build`, `make web-typecheck`, `make web-build`, and all applicable browser/conformance checks.
- [ ] Inspect `git diff`, migration ledger, and secret-safe payload evidence.
- [ ] Merge the development branch into `main_temp`, deploy from `main_temp`, and verify web/API/worker/ready health endpoints.
- [ ] Merge `main_temp` into `main`; do not merge `codex/connector-platform-p0`.
- [ ] Commit documentation of evidence and report any environment-gated checks that remain unverified.
