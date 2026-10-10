# Expert Package and Expert Team execution

Status: design confirmed on 2026-10-09; refreshed development branch established; application implementation completed; final local acceptance and review in progress.

The [confirmed design](../product/expert-package-team-refactor.md) and [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md) govern this task. The User explicitly approved Administrator-only Expert Teams, deletion of ordinary-User-owned team definitions without altering private historical snapshots, removal of automatic tags and separate classification, and direct discovery of both platform resource types from `experts/`.

The synthesized implementation specification is published as [Issue #71](https://github.com/owl-go/agent-platform/issues/71), labeled `ready-for-agent`. It includes 68 User Stories and AC-EX-01 through AC-EX-10. On 2026-10-09 the User confirmed the test seams: the existing authenticated API is the main acceptance entry, the existing Executor and replaceable Runtime Adapter cover execution behavior, real PostgreSQL covers migration/concurrency/accounting, and component/browser checks cover the interface. No production API is added solely for testing. Publication and this confirmation do not establish implementation or test completion.

## Repository preparation

The User requested consolidation before implementation, excluding the existing design branch. Pending feature branches were merged into `main_temp` and then into `main` through [PR #70](https://github.com/owl-go/agent-platform/pull/70), using a merge commit to preserve branch ancestry. `main` and `main_temp` now point to `fa701ab2709d3347d28953970fbc44c5bdda9d0c`. Annotated tag `v1.0.2` marks that baseline and has been pushed.

Four merged feature branches were deleted locally and remotely: `codex/execution-activity-layout`, `codex/project-harness-init`, `codex/scan-registration`, and `codex/select-unverified-models`. Forty-two existing secondary worktrees were removed after fresh clean-status checks; three missing worktree records were pruned. Only the primary checkout remains. The existing `codex/expert-package-team-refactor` design branch is preserved. The new `codex/expert-team-refactor` branch starts from the tagged baseline and carries its design documents through a merge without rewriting the earlier commits.

Git history, detached worktree heads, and non-cache ignored local files were archived outside the repository at `/Users/frank/.codex/backups/agent-platform/20261009-180820`. The bundle and local-file archives were verified before cleanup. Dependency and build caches were excluded. This archive concerns local repository cleanup; it is not a database backup or a production recovery exercise.

Preparation validation: `make test`, `make build`, frontend tests (55 files, 626 tests), `make web-typecheck`, `make web-build`, `make resources-check`, Markdown relative-link checks, `git diff --check`, and the `AGENTS.md`/`CLAUDE.md` mirror check passed. Local PostgreSQL tests were skipped without a DSN. All four jobs of CI run `37915977726` passed, including the backend PostgreSQL and race checks, generated contracts, frontend, and deployment checks. Local component browser checks passed at 1440×1000 and 390×844; screenshots are retained in the local archive. These checks validate the integrated baseline and do not establish the new refactor’s behavior or production deployment.

## Published implementation tickets

On 2026-10-09 the User approved the 22-ticket breakdown. Each ticket is published with `ready-for-agent`; the 32 blocking edges are native GitHub issue dependencies and are also listed in each ticket body. The parent Issue #71 remains unchanged. Publication does not establish implementation or acceptance. Read-back verification matched all 22 approved bodies, titles, open states and labels, and all 32 exact dependencies. The initial unblocked frontier is Issues #72, #73, and #74.

| Ticket | Published issue | Blocked by |
|---|---|---|
| T01 | [#72 — 以 Markdown 指引创建、编辑和执行单专家](https://github.com/owl-go/agent-platform/issues/72) | None |
| T02 | [#73 — 移除自动标签生成和独立分类](https://github.com/owl-go/agent-platform/issues/73) | None |
| T03 | [#74 — 管理员维护专家团，普通用户选择平台专家团](https://github.com/owl-go/agent-platform/issues/74) | None |
| T04 | [#75 — 删除旧私人专家团并保留私有历史重试](https://github.com/owl-go/agent-platform/issues/75) | [#74](https://github.com/owl-go/agent-platform/issues/74) |
| T05 | [#76 — 编辑独立团队成员并明确指定领队](https://github.com/owl-go/agent-platform/issues/76) | [#72](https://github.com/owl-go/agent-platform/issues/72), [#74](https://github.com/owl-go/agent-platform/issues/74) |
| T06 | [#77 — 通过中性 ZIP 导入导出专家](https://github.com/owl-go/agent-platform/issues/77) | [#72](https://github.com/owl-go/agent-platform/issues/72) |
| T07 | [#78 — 管理员通过 ZIP 导入导出独立专家团](https://github.com/owl-go/agent-platform/issues/78) | [#76](https://github.com/owl-go/agent-platform/issues/76), [#77](https://github.com/owl-go/agent-platform/issues/77) |
| T08 | [#79 — 从目录初始化和升级平台专家](https://github.com/owl-go/agent-platform/issues/79) | [#77](https://github.com/owl-go/agent-platform/issues/77) |
| T09 | [#80 — 从同一目录初始化和升级平台专家团](https://github.com/owl-go/agent-platform/issues/80) | [#78](https://github.com/owl-go/agent-platform/issues/78), [#79](https://github.com/owl-go/agent-platform/issues/79) |
| T10 | [#81 — 编辑和携带专家头像与开场提示](https://github.com/owl-go/agent-platform/issues/81) | [#78](https://github.com/owl-go/agent-platform/issues/78) |
| T11 | [#82 — 会话式创建统一专家与管理员专家团](https://github.com/owl-go/agent-platform/issues/82) | [#81](https://github.com/owl-go/agent-platform/issues/81) |
| T12 | [#83 — 在 Session 中由领队按需委派并给出正式回答](https://github.com/owl-go/agent-platform/issues/83) | [#76](https://github.com/owl-go/agent-platform/issues/76) |
| T13 | [#84 — 在 Workflow 中完成领队协作并成功后提交 Workspace](https://github.com/owl-go/agent-platform/issues/84) | [#83](https://github.com/owl-go/agent-platform/issues/83) |
| T14 | [#85 — 执行包内技能并绑定用户自己的连接器](https://github.com/owl-go/agent-platform/issues/85) | [#78](https://github.com/owl-go/agent-platform/issues/78), [#84](https://github.com/owl-go/agent-platform/issues/84) |
| T15 | [#86 — 并行执行三个成员并由领队处理文件冲突](https://github.com/owl-go/agent-platform/issues/86) | [#84](https://github.com/owl-go/agent-platform/issues/84) |
| T16 | [#87 — 允许一次任务修复并拒绝未完成的必需工作](https://github.com/owl-go/agent-platform/issues/87) | [#83](https://github.com/owl-go/agent-platform/issues/83) |
| T17 | [#88 — 按响应 Credit 预算准入每次调用](https://github.com/owl-go/agent-platform/issues/88) | [#83](https://github.com/owl-go/agent-platform/issues/83) |
| T18 | [#89 — 全体等待审批时暂停主动执行计时](https://github.com/owl-go/agent-platform/issues/89) | [#86](https://github.com/owl-go/agent-platform/issues/86) |
| T19 | [#90 — 中断后明确失败并由用户决定重试](https://github.com/owl-go/agent-platform/issues/90) | [#86](https://github.com/owl-go/agent-platform/issues/86) |
| T20 | [#91 — 在 Task Panel 展示真实委派与成员结果](https://github.com/owl-go/agent-platform/issues/91) | [#86](https://github.com/owl-go/agent-platform/issues/86), [#87](https://github.com/owl-go/agent-platform/issues/87) |
| T21 | [#92 — 收口旧写入和目录原件，保留历史读取](https://github.com/owl-go/agent-platform/issues/92) | [#73](https://github.com/owl-go/agent-platform/issues/73), [#80](https://github.com/owl-go/agent-platform/issues/80), [#82](https://github.com/owl-go/agent-platform/issues/82), [#84](https://github.com/owl-go/agent-platform/issues/84) |
| T22 | [#93 — 完成跨入口验收和迁移恢复证据](https://github.com/owl-go/agent-platform/issues/93) | [#85](https://github.com/owl-go/agent-platform/issues/85), [#88](https://github.com/owl-go/agent-platform/issues/88), [#89](https://github.com/owl-go/agent-platform/issues/89), [#90](https://github.com/owl-go/agent-platform/issues/90), [#91](https://github.com/owl-go/agent-platform/issues/91), [#92](https://github.com/owl-go/agent-platform/issues/92) |

## Acceptance tracking

Each acceptance item requires implementation and focused evidence before completion. Final implementation evidence is recorded in [the local acceptance report](../evidence/agent-workspace/2026-10-10-expert-refactor-local-acceptance.md). Earlier progress sections below describe their dated state; baseline checks do not count as refactor acceptance.

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

## Implementation evidence — 2026-10-09

Implementation is in progress on `codex/expert-team-refactor`; the User selected `5269ea8` as the fixed review baseline. No implementation ticket or acceptance group is complete yet.

- T01: authenticated HTTP Markdown create/read and controlled structured-input conversion passed against a disposable local PostgreSQL 17 database. The Executor test confirms that Markdown takes precedence and display metadata/obsolete form guidance is excluded. Expert revision immutability and retention after definition deletion passed. A second custom Expert exposed an existing nullable `system_key` insertion defect; its regression is covered by the same HTTP test.
- T02: the obsolete tag-generation claim and model invocation path has been removed. A PostgreSQL Worker test confirms that legacy queued tag work is not scheduled. The catalog component test confirms removal of tag/category controls. Public compatibility fields still await T21 contract closure.
- T03–T05: authenticated Team writes reject ordinary Users before resolving content. Repository tests verify Administrator-only maintenance, Platform Team reading, independent copied member guidance after a source edit/deletion, and explicit lead selection. New selections are being moved to Team-owned member definitions. Editor and remaining authoring/package paths are still in progress.
- T04: a fixture reconstructed from the unchanged migration chain through `000074` passes forward migration: ordinary-User Team definitions and mutable Session references are removed, Administrator Team definitions and historical response content remain, and original structured guidance text/spacing survives conversion. Repeat migration passes. Workflow/Run reference, accounting, concurrency and restoration evidence remains pending.

Commands executed for these slices: focused `go test` selections in `internal/service/workspace`, `internal/data/workspace/gormrepo`, and `internal/data/workspace/runtimeexecutor`; `make generate`; focused editor/catalog Vitest files; `make web-typecheck`. PostgreSQL uses a task-owned local container and per-test disposable databases. This is local implementation evidence, not production migration, deployment, or Runtime image conformance.

## Implementation evidence — 2026-10-10

The implementation remains in progress; no ticket or acceptance group is declared complete.

- T06–T09: authenticated Expert/Team ZIP import and export, idempotence, content conflict, revision upgrade, copied member identity, unsafe manifest rejection, and visibility tests passed. Direct directory discovery and concurrent default initialization tests passed for both kinds. All eight distributed Expert definitions now use `.plugin/plugin.json` and Markdown. Bundled Skills, profile assets, dependency export, upgrade UI, and remaining rejection/rollback evidence are still pending.
- T12/T15/T16: Executor tests passed for lead delegation and official response separation, distinct invocation identities, three-member concurrency, independent writable Workspaces, explicit file-conflict resolution, one repair, required failure, invalid control actions, and invocation limits. Raw invocation events and lead coordination JSON are withheld from user activities. Workflow conversion and cross-entry execution evidence remain pending.
- T17: authenticated settings and a component test verify an optional Team response admission budget in Credits, including the explanation that the last admitted call can overrun its reservation. New response snapshots freeze the setting; manual Session retry preserves it. PostgreSQL tests passed for parallel reservation, measured overrun, blocking subsequent admission, unused-call abort, and idempotent settlement. An Executor test using the real accounting Repository confirms that an over-budget lead settlement prevents a member model from starting and is not charged twice at failure. Additional Department, cancellation, failed usage and retry evidence remains pending.
- T18/T19: a controlled-clock test verifies that active time pauses only when every executing invocation awaits approval, resumes without resetting, and cannot escape an exhausted budget. PostgreSQL parallel approval tests verify aggregate wait state, exclusion of queued tasks, and approval closure on cancellation. New-strategy Session/Workflow restart tests verify failure without replay while retaining completed evidence and consumption. Legacy restart tests still pass. Executor cancellation stops active and queued members and removes staged files; graceful Worker shutdown finalizes the new strategy. Real connector expiry and broader concurrent recovery evidence remain pending.
- T20: authenticated HTTP and Task Panel component tests verify actual lead/member identities, repairs, queued state, resolved conflicts, and member results. The call counter excludes queued invocations. Coordination JSON and internal accounting values are filtered from public Task Panel records. Stream, browser, roster and limit-state coverage remains pending.

Executed checks include the focused PostgreSQL tests above, the Runtime Executor target package, Credits/Workspace Domain and Application target packages, both Task Panel and Settings Vitest files (16 tests), `make generate`, and frontend typecheck. Final backend/frontend gates, full acceptance review, exact-image Runtime conformance and Linux sandbox checks have not yet been completed.


## Final implementation evidence — 2026-10-10

All 22 implementation slices are present on the same branch. [The local acceptance report](../evidence/agent-workspace/2026-10-10-expert-refactor-local-acceptance.md) maps every one of the 68 User Stories and all ten acceptance groups to the executed seams and records migration recovery, browser and gate evidence. Final review and generated verification remain in progress. No deployment, new exact-image Runtime conformance or Linux sandbox evidence is asserted; the parent Issue remains unchanged.
