# Expert Package and Expert Team execution

Status: design confirmed on 2026-10-09; refreshed development branch established; application implementation pending.

The [confirmed design](../product/expert-package-team-refactor.md) and [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md) govern this task. The User explicitly approved Administrator-only Expert Teams, deletion of ordinary-User-owned team definitions without altering private historical snapshots, removal of automatic tags and separate classification, and direct discovery of both platform resource types from `experts/`.

## Repository preparation

The User requested consolidation before implementation, excluding the existing design branch. Pending feature branches were merged into `main_temp` and then into `main` through [PR #70](https://github.com/owl-go/agent-platform/pull/70), using a merge commit to preserve branch ancestry. `main` and `main_temp` now point to `fa701ab2709d3347d28953970fbc44c5bdda9d0c`. Annotated tag `v1.0.2` marks that baseline and has been pushed.

Four merged feature branches were deleted locally and remotely: `codex/execution-activity-layout`, `codex/project-harness-init`, `codex/scan-registration`, and `codex/select-unverified-models`. Forty-two existing secondary worktrees were removed after fresh clean-status checks; three missing worktree records were pruned. Only the primary checkout remains. The existing `codex/expert-package-team-refactor` design branch is preserved. The new `codex/expert-team-refactor` branch starts from the tagged baseline and carries its design documents through a merge without rewriting the earlier commits.

Git history, detached worktree heads, and non-cache ignored local files were archived outside the repository at `/Users/frank/.codex/backups/agent-platform/20261009-180820`. The bundle and local-file archives were verified before cleanup. Dependency and build caches were excluded. This archive concerns local repository cleanup; it is not a database backup or a production recovery exercise.

Preparation validation: `make test`, `make build`, frontend tests (55 files, 626 tests), `make web-typecheck`, `make web-build`, `make resources-check`, Markdown relative-link checks, `git diff --check`, and the `AGENTS.md`/`CLAUDE.md` mirror check passed. Local PostgreSQL tests were skipped without a DSN. All four jobs of CI run `37915977726` passed, including the backend PostgreSQL and race checks, generated contracts, frontend, and deployment checks. Local component browser checks passed at 1440×1000 and 390×844; screenshots are retained in the local archive. These checks validate the integrated baseline and do not establish the new refactor’s behavior or production deployment.

## Acceptance tracking

Each acceptance item requires implementation and focused evidence before completion. These items are all pending; the baseline checks above do not count as refactor acceptance.

| ID | Given / action / observable result | Rules and gates |
|---|---|---|
| AC-EX-01 | Valid neutral Expert or Expert Team ZIP is imported/exported with one authoritative guidance document; unsafe paths, links, unknown fields, invalid versions, conflicting content, and invalid images are rejected. | ARC-002/003, COD-002/003/006; G-GO, G-API, G-SEC |
| AC-EX-02 | Adding a valid `experts/<key>/` directory discovers either resource type; repeat/concurrent initialization and versioned upgrades preserve identity and history without taking over custom resources. | DB-001/003/005, ARC-003; G-GO, G-DB |
| AC-EX-03 | Ordinary Users cannot create, update, import, copy, or confirm creation of a Team through any entry point; they can select platform Teams using their own permissions. | COD-002, DB-004; G-SEC, G-API, G-WEB |
| AC-EX-04 | Upgrading representative old data losslessly converts Expert guidance, deletes ordinary-User-owned Teams and mutable references, and preserves historical private snapshots and sequential retry semantics. | DB-001/004/005/006; G-DB, G-GO |
| AC-EX-05 | Editing/importing a Team copies independent member definitions and records an explicit lead; source Expert changes cannot rewrite those definitions or frozen executions. | ARC-002/003, DB-001/003; G-GO, G-DB, G-API |
| AC-EX-06 | Valid lead actions execute only frozen members, with at most three concurrent members and twenty actual calls by default; each repeated call has independent identity and accounting. Invalid actions and limit exhaustion fail explicitly. | COD-002/003, DB-003; G-GO, G-DB, G-ARCH |
| AC-EX-07 | Independent member Workspaces merge checked changes; conflicts return to the lead. Required failure, cancellation, or interruption prevents promotion and preserves actual usage and safe external-operation facts. | COD-003/004/006, DB-003; G-GO, G-SEC, G-DB |
| AC-EX-08 | A task permits one repair delegation. Response Credit limits govern invocation admission; approval pauses active time only while every outstanding execution awaits approval, retaining individual expiry. | ARC-002, COD-003, DB-003; G-GO, G-DB |
| AC-EX-09 | Editors, direct APIs, and conversational authoring use the same definition; optional images and at most three starter prompts are validated. No automatic tag or independent classification UI/generation remains. | COD-002/006, UI-001–005; G-API, G-WEB, G-E2E |
| AC-EX-10 | Task Panel shows persisted task/member states, invocation count, consumption, conflicts, and expandable member final results; only the lead’s final answer becomes the official reply. | COD-004/006, UI-002–005; G-GO, G-API, G-WEB, G-E2E |

## Implementation order and recovery

Start with the neutral package validation boundary and authoritative guidance model, then directory discovery and immutable revision persistence. Enforce Team permissions and migrate private definitions with representative PostgreSQL fixtures. Extend the versioned execution snapshot and coordinator above the existing Runtime Adapter, including Workspace merge and accounting. Finally update authoring, package transfer, and the Task Panel against the real API contracts.

Historical migrations remain immutable. Database deletion and conversion require a new migration and documented forward-repair/data-restoration behavior. No production migration, release, or Runtime conformance is claimed. Exact-image and Linux sandbox evidence remains a separate environment gate. Keep progress, failures, recovery steps, and actual commands in this ticket as implementation proceeds.
