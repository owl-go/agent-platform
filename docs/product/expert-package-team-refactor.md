# Expert Packages and Expert Team Refactor

Status: design in progress. Foundational directions and data/execution boundaries confirmed on 2026-10-09; remaining decisions and final design review pending. No implementation or deployment is asserted.

## Confirmed directions

1. Both Experts and Expert Teams support standalone packages, import, and export. The neutral package entry is `.plugin/plugin.json`, with role definitions in `agents/`, optional bundled Skills in `skills/`, and optional profile images in `avatars/`. User-facing terminology remains Expert and Expert Team. The package contains no account credentials.
2. Each Expert's Markdown guidance is the single authoritative instruction document. Creation assistance may generate or edit that document. Introduction remains independent and display-only; a form and Markdown document must not retain separate authoritative versions of the same instruction.
3. Expert Team collaboration is coordinated by a Team Lead. The lead chooses members according to the task, delegates work, and produces the official response. The platform controls actual scheduling, limits concurrency and delegation, enforces consumption limits, and isolates member contexts. A prompt alone cannot establish that these controls exist.
4. Project documentation, package directory names, metadata extensions, and examples use neutral vocabulary. No supplier-specific names or reference links are included in these documents.

## Team member and dependency ownership

An Expert Team owns each member definition, including its guidance and resource bindings. Adding a catalog Expert copies its selected definition into a team-owned role; editing or deleting the source Expert does not rewrite that role. Members remain individually editable and keep stable identity through renaming or display reordering. The Team Lead coordinates these definitions rather than creating an execution-order dependency from their display order.

Bundled Skills remain within the Expert Package's content and are not automatically registered as separate Skill catalog resources. External Connector declarations are dependencies that the User resolves against available Connectors and authorizes through the existing platform flow. Import does not provide an account grant, verified installation, or Runtime conformance status. Package content and actual external access remain distinct boundaries.

## Legacy conversion and historical execution

Existing Expert guidance fields are converted to one Markdown document with deterministic headings, retaining the original authored text. Introduction remains display-only. Existing teams require an explicit Team Lead selection to adopt the new collaboration model. Automatic conversion must not reinterpret an old member as a coordinator without the User choosing it.

Historical response and Run snapshots retain their original execution strategy and guidance content. A historical sequential-team retry remains sequential; editing or upgrading the current team must not rewrite that snapshot. A new versioned execution contract distinguishes lead-coordinated work from the earlier ordered-stage format. Existing immutable database migrations remain unchanged; conversion uses new migrations and preserves historical compatibility.

## Parallel execution and Workspace merge

Parallel execution is supported with a default limit of three simultaneously executing members. Each member invocation operates on its own copy of the eligible staged Workspace rather than sharing a writable directory with another invocation. Member contexts and staged Native Session state remain isolated.

The platform checks proposed file changes before merging them into the response's staged Workspace. Conflicting changes to the same path are returned to the Team Lead for resolution rather than being applied by completion order. Resolution remains subject to the overall invocation and timeout limits. The persistent Workflow Workspace is promoted only after the entire response succeeds.

## Invocation, timeout, and Credit limits

The default maximum is twenty actual model invocations per response, counting initial and resumed Team Lead invocations, member calls, and repair delegations. Each invocation needs its own immutable execution identity and idempotent accounting; a reused member identity cannot identify a repeated call. The default overall execution timeout is two hours.

A configurable per-response Credit budget governs admission before every invocation, together with the existing User balance and applicable Department budget checks. A model reservation is not a hard bound on measured usage: the last admitted call may settle above its reservation or remaining response budget. After settlement exhausts the budget, further model calls stop. The interface must describe this admission behavior accurately rather than promise an exact maximum actual charge.

## Failure, repair, and retry

A member failure is reported to the Team Lead. Each delegated task permits at most one new delegation for repair, and all actual model invocations consume the response's invocation and Credit limits. Unresolved required work fails the whole response, and failure prevents Workspace promotion.

A User retry starts a new attempt from the original frozen input and definition, preserving the historical execution strategy. All model invocations that actually consumed usage remain charged, including failed attempts. External side effects are reported explicitly because discarding the staged Workspace cannot undo them; exact approval and recovery rules remain to be settled.

## Current implementation boundary

The implementation on the design baseline stores Experts as structured fields and catalog-resource references. Expert Teams have stable member identities and invoke every member in fixed order, with the final member providing the official response. Expert and Expert Team management does not yet provide the proposed package import/export contract, authoritative Markdown guidance, or Team Lead delegation.

This proposal changes established contracts. [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md) records the confirmed direction and the intended replacement scope for [ADR-0022](../adr/0022-platform-managed-sequential-expert-teams.md) and [ADR-0026](../adr/0026-separate-expert-guidance-from-execution-and-connectors.md). The root [glossary](../../CONTEXT.md) records the resolved domain meanings; that vocabulary is not implementation evidence. Existing product and technical contracts continue to describe current behavior until this design is finalized and their replacement scope is explicitly recorded.

## Open decisions

- Package schema, version identity, install/update conflicts, and supported import formats.
- References to standalone catalog Skills, portable dependency identities, and authorization recovery.
- Team Lead/member permissions, context handoff, delegation transport, and required-task semantics.
- Call/timeout/budget exhaustion, approval waits, cancellation, Worker recovery, and side-effect replay controls.
- Profile images, tags, editing/creation flows, and shared-resource ownership scope.
- Implementation scope, required validation, and final shared-understanding review.

## Evidence

Read-only inspection established the existing contracts; no application tests, database integration checks, Runtime image checks, or production conformance have been executed for this proposal. Documentation checks are reported with each documentation commit and do not establish execution capability.
