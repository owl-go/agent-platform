# Expert Packages and Expert Team Refactor

Status: shared understanding confirmed by the User on 2026-10-09; implemented, locally verified and reviewed. Includes Administrator-only team creation, deletion of legacy ordinary-User-owned team definitions, and removal of automatic tags and separate classification. Implementation evidence is tracked in the execution document; no deployment is asserted.

## Confirmed directions

1. Both Experts and Expert Teams support standalone packages, import, and export. The neutral package entry is `.plugin/plugin.json`, with role definitions in `agents/`, optional bundled Skills in `skills/`, and optional profile images in `avatars/`. User-facing terminology remains Expert and Expert Team. The package contains no account credentials.
2. Each Expert's Markdown guidance is the single authoritative instruction document. Creation assistance may generate or edit that document. Introduction remains independent and display-only; a form and Markdown document must not retain separate authoritative versions of the same instruction.
3. Expert Team collaboration is coordinated by a Team Lead. The lead chooses members according to the task, delegates work, and produces the official response. The platform controls actual scheduling, limits concurrency and delegation, enforces consumption limits, and isolates member contexts. A prompt alone cannot establish that these controls exist.
4. Project documentation, package directory names, metadata extensions, and examples use neutral vocabulary. No supplier-specific names or reference links are included in these documents.

## Team member and dependency ownership

An Expert Team owns each member definition, including its guidance and resource bindings. Adding a catalog Expert copies its selected definition into a team-owned role; editing or deleting the source Expert does not rewrite that role. Members remain individually editable and keep stable identity through renaming or display reordering. The Team Lead coordinates these definitions rather than creating an execution-order dependency from their display order.

Bundled Skills remain within the Expert Package's content and are not automatically registered as separate Skill catalog resources. External Connector declarations are dependencies that the User resolves against available Connectors and authorizes through the existing platform flow. Import does not provide an account grant, verified installation, or Runtime conformance status. Package content and actual external access remain distinct boundaries.

## Package lifecycle and authoring

Import and export use ZIP archives in the neutral package format. The manifest declares a schema version, a stable package identity, and a semantic package version. Importing the same identity, version, and validated content is idempotent. Different content under the same identity and version is a conflict, not an implicit overwrite. Updates create immutable revisions; existing selections and historical snapshots retain their selected content.

Conversational creation, direct editing, and ZIP import are supported authoring paths. All produce the same validated definition. The editor manages name, Introduction, authoritative Markdown guidance, and resource bindings; the team editor additionally manages its Team Lead and independent member definitions. Profile images are optional with a default Profile Icon, and each profile may declare up to three starter prompts. Creating or editing a profile never starts a conversation automatically.

Automatic tag generation and separate Expert/Expert Team classification are removed from this refactor. The catalog, editor, package manifest, and creation proposals do not introduce tag or category fields. Capability and applicability are expressed through the profile's Introduction and Expert Guidance. The existing asynchronous Expert tag-generation path is removed; legacy stored projection fields may only be retained for migration or historical reading, not refreshed or displayed as a new product feature. User-authored member responsibility labels remain distinct role guidance.

## Ownership and permissions

Ordinary Users may create private Experts. Administrators may create Platform Experts and Platform Expert Teams. Expert Team creation is Administrator-only across direct API calls, manual and conversational creation, package import, copying, and any guided creation confirmation. Ordinary Users may browse and select available Platform Expert Teams; choosing a team never grants access to an Administrator's account authorization.

Administrator-owned resources remain editable only by their owning Administrator. Platform default originals are immutable; an authorized creator may copy them into a separately named custom resource. Ordinary Users cannot obtain a new team by copying or importing a platform team. Department-scoped specialist sharing is outside this refactor.

The User explicitly chose deletion of existing ordinary-User-owned private team definitions. They are not published, transferred to an Administrator, or preserved as a read-only team catalog. The migration deletes these definitions and clears mutable Workflow, conversation-selection, and draft bindings so they cannot initiate another response. This deletion does not require reading or exposing their authored content. Historical conversations, accepted inputs, and immutable execution snapshots remain private to their original owner and retain their own recorded content.

## Platform directory loading

Platform Experts and Platform Expert Teams are both distributed through the top-level `experts/` directory, alongside the existing `skills/` and `connectors/` resource roots. Each `experts/<key>/` subdirectory contains a neutral Expert Package, whose `.plugin/plugin.json` declares whether it defines an Expert or Expert Team. Adding a valid subdirectory is sufficient for discovery; no central inventory entry or hard-coded resource list is required.

Installation, application deployment, and API/Worker startup use the existing default-resource discovery and initialization boundary. All directory definitions, package content, and dependencies are validated before startup database mutations. Stable keys, package versions, content digests, initialization-ledger identities, and transactional locking preserve resource identity and avoid duplication across repeat or concurrent startup. Content changes require a new package version; the same identity and version with different content is rejected.

Directory originals belong to the Bootstrap Administrator and remain immutable Platform Resources. Managed upgrades preserve their resource identity and frozen historical content. Existing custom Administrator resources are not silently taken over, and User-private resources are not inspected or overwritten by initialization. Removing a directory is not a request to delete an initialized resource. Loading a directory provides neither Connector Authorization nor Runtime conformance evidence.

The current `expert.json` directory format is migrated to this common package contract, and the loader learns both Expert and Expert Team definitions. The current initialization implementation does not yet provide this team-package path; [default resource documentation](../technical/default-resources.md) separates its existing behavior from this planned replacement.

## Legacy conversion and historical execution

Existing Expert guidance fields are converted to one Markdown document with deterministic headings, retaining the original authored text. Introduction remains display-only. Retained Administrator-owned teams require explicit Team Lead selection by their owning Administrator to adopt the new collaboration model. Automatic conversion must not reinterpret an old member as a coordinator without that choice. Ordinary-User-owned team definitions are deleted under the ownership decision above.

Historical response and Run snapshots retain their original execution strategy and guidance content. A historical sequential-team retry remains sequential; editing or upgrading the current team must not rewrite that snapshot. A new versioned execution contract distinguishes lead-coordinated work from the earlier ordered-stage format. Existing immutable database migrations remain unchanged; conversion uses new migrations and preserves historical compatibility.

## Parallel execution and Workspace merge

Parallel execution is supported with a default limit of three simultaneously executing members. Each member invocation operates on its own copy of the eligible staged Workspace rather than sharing a writable directory with another invocation. Member contexts and staged Native Session state remain isolated.

The platform checks proposed file changes before merging them into the response's staged Workspace. Conflicting changes to the same path are returned to the Team Lead for resolution rather than being applied by completion order. Resolution remains subject to the overall invocation and timeout limits. The persistent Workflow Workspace is promoted only after the entire response succeeds.

## Invocation, timeout, and Credit limits

The default maximum is twenty actual model invocations per response, counting initial and subsequent Team Lead invocations, member calls, and repair delegations. Each invocation needs its own immutable execution identity and idempotent accounting; a reused member identity cannot identify a repeated call. The default overall active execution timeout is two hours. Limit exhaustion explicitly fails the response and stops remaining work rather than promoting partial results.

A configurable per-response Credit budget governs admission before every invocation, together with the existing User balance and applicable Department budget checks. A model reservation is not a hard bound on measured usage: the last admitted call may settle above its reservation or remaining response budget. After settlement exhausts the budget, further model calls stop. The interface must describe this admission behavior accurately rather than promise an exact maximum actual charge.

## Failure, repair, and retry

A member failure is reported to the Team Lead. Each delegated task permits at most one new delegation for repair, and all actual model invocations consume the response's invocation and Credit limits. Unresolved required work fails the whole response, and failure prevents Workspace promotion.

A User retry starts a new attempt from the original frozen input and definition, preserving the historical execution strategy. All model invocations that actually consumed usage remain charged, including failed attempts. External side effects are reported explicitly because discarding the staged Workspace cannot undo them. An interruption does not establish that an external write was rolled back or did not happen.

## Structured coordination and context

The Team Lead emits a strictly validated structured action for each coordination round: delegate work to frozen members, or complete with the official response. Platform scheduling, accounting, and file merge respond only to validated actions, not inferred intent in prose. Every requested member must belong to the frozen roster, and members cannot create nested teams. A completed Runtime invocation supplies the action; the platform collects the member results and starts the lead's next invocation without requiring native subagent or internal MCP support.

Members receive their assigned task and the bounded conversation context, accepted attachments, and eligible Workspace content needed for it. Resource permissions remain member-specific, with explicit composer additions and exclusions applied under the existing Conversation Selection contract. Results and safe file-change summaries return to the Team Lead; another member's raw reasoning, tool logs, credentials, or unused resource bindings are not shared.

## Approval waiting, cancellation, and interruption

The active execution timer pauses only when all outstanding execution is waiting for User approval. If another member is still executing, time continues to accrue. Each approval retains the existing maximum fifteen-minute expiry and one-use command/identity/target binding. Rejection and expiry cannot execute the command. The owning User, rather than the team author or another Administrator, decides execution approvals.

Cancellation stops the entire attempt, including queued and active members, prevents later events and Workspace promotion, and cleans up per-invocation resources. Worker interruption makes the interrupted response explicitly failed; recorded usage and safe operation facts are retained, pending approvals are closed, and the User chooses whether to retry. Automatic reconstruction of an interrupted team or uncertain external writes is not part of this version.

## User-visible collaboration

The existing Task Panel displays persisted delegated tasks, member states, invocation count, consumption, and Workspace conflicts. Member final results are expandable; the Team Lead's result is the official conversational answer. Activity comes from actual platform transitions. Structured coordination control output is not shown as an official answer, and raw private reasoning, credentials, or unrestricted tool payloads are not exposed.

## Current implementation boundary

The implementation on the design baseline stores Experts as structured fields and catalog-resource references. Expert Teams have stable member identities and invoke every member in fixed order, with the final member providing the official response. Expert and Expert Team management does not yet provide the proposed package import/export contract, authoritative Markdown guidance, or Team Lead delegation.

This proposal changes established contracts. [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md) records the confirmed direction and the intended replacement scope for [ADR-0022](../adr/0022-platform-managed-sequential-expert-teams.md) and [ADR-0026](../adr/0026-separate-expert-guidance-from-execution-and-connectors.md). The root [glossary](../../CONTEXT.md) records the resolved domain meanings; that vocabulary is not implementation evidence. This confirmed design governs the refactor. Existing product and technical contracts continue to describe the earlier implementation until their affected sections are updated with implementation and validation evidence.

## Final review

All interview decisions are settled and the User confirmed the complete shared understanding, including deletion of ordinary-User-owned team definitions, removal of automatic tags and separate classification, and directory loading of both platform resource types. No further design approval is pending.

## Implementation and validation scope

The refactor covers the Domain model and immutable snapshots, package parsing and storage, versioned database migrations, authenticated API enforcement, Expert and Expert Team authoring, directory-loaded platform resources, platform coordination, Workspace merge, accounting, approval timing, interruption handling, and actual Task Panel activity. Runtime Drivers remain responsible for their CLI-specific execution and parsing; orchestration belongs above the existing Adapter boundary.

Required focused tests cover package safety and content conflicts; automatic Expert/Expert Team directory discovery, repeat/concurrent initialization, managed upgrades, immutable originals, and preservation of custom resources; ordinary-User rejection of every team creation path; resource visibility and private-content isolation; removal of private-team definitions and mutable references without changing immutable history; exact revision retention; lossless conversion and historical retry; absence of automatic tag generation and category fields; roster/action validation; three-member concurrency and repeated-call identities; parallel file conflicts; transactional reservations and settlements; repair and global limits; approval expiry and active-time accounting; cancellation and interruption; and user-visible final versus member output. The implementation must execute affected package tests, backend test/build gates, frontend typecheck/build and focused interaction tests, `make resources-check`, and PostgreSQL migration/permission/accounting/default-resource integration checks when the database environment is available.

Real Runtime and Linux sandbox acceptance remains a separate gate for each applicable exact image and engine. Passing local fake-runtime tests does not establish model coordination quality or production conformance. Following the User’s explicit repository consolidation request, development starts on `codex/expert-team-refactor`, created from the refreshed `main` at tag `v1.0.2`. The earlier design branch is preserved, and its confirmed documents are carried into this development branch. Commits and pushes follow the repository instructions, while integration and any release use the prescribed `main_temp` path. Progress and acceptance tracking are recorded in [the execution ticket](../tickets/expert-package-team-refactor-execution.md).

## Evidence

Read-only inspection established the existing contracts; no application tests, database integration checks, Runtime image checks, or production conformance have been executed for this proposal. Documentation checks are reported with each documentation commit and do not establish execution capability.
