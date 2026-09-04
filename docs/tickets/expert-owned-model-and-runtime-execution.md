# Expert-owned Provider Model and Runtime Engine execution

## Problem Statement

Users currently choose one Provider Model and Runtime Engine at the Session or Workflow level. A single Expert inherits that execution configuration, and every member of an Expert Team shares it. This makes an Expert's behavior depend on where it is invoked and prevents a team from combining specialists that require different models or Runtime Engines. It also creates competing configuration authorities across Personal Settings, Sessions, Workflows, and Experts.

Users need each Expert to carry a stable, inspectable execution identity. A team should be able to run one Expert with Claude Code and another with Codex, or use different Provider Models for different members, without adding a team-level override or asking the caller to configure each execution. Historical conversations must remain reproducible, and mixed execution must preserve the existing fail-fast, rollback, security, and audit guarantees.

## Solution

Make a required Provider Model and Runtime Engine part of every complete Expert. A single Expert always executes with its own pair, and an Expert Team executes each frozen member with that member's pair. Sessions and Workflows no longer offer or persist Provider Model or Runtime Engine overrides. Execution without an Expert resolves the current Personal Settings defaults at the appropriate snapshot boundary.

Represent every actual Provider Model invocation as an ordered Execution Stage Snapshot. A no-Expert execution contains one anonymous stage, a single Expert contains one Expert stage, and an Expert Team contains one stage per member. Each stage freezes its complete execution identity and produces independently auditable status, usage, Credit Consumption, and result metadata. The team still shares its order, overall timeout, preceding-result handoff, and Workflow temporary Workspace.

Preserve legacy frozen snapshots under their original shared-model and shared-engine semantics. New execution uses only the new stage structure. Runtime state, credentials, containers, compatibility, and Credits are resolved and enforced per stage, while whole-team success remains the commit boundary for Workflow changes and Native Session state.

## User Stories

1. As a User, I want every Expert to have a required Provider Model, so that its model behavior is stable wherever I use it.
2. As a User, I want every Expert to have a required Runtime Engine, so that its execution capabilities do not depend on a Session or Workflow setting.
3. As a User creating an Expert, I want the current default model for the selected Runtime Engine to be suggested, so that common setup remains quick.
4. As a User creating an Expert, I want to confirm the model and engine explicitly before saving, so that defaults are never mistaken for an intentional Expert configuration.
5. As a User editing an Expert, I want to change its Provider Model and Runtime Engine together with its other visible configuration, so that the Expert has one understandable execution profile.
6. As a User, I want an unverified model-engine pair to remain selectable with a clear warning, so that I can use configurations that have not yet completed conformance.
7. As a User, I want an incompatible model-engine pair to be rejected, so that I cannot save an Expert that is already known not to run.
8. As a User, I want an Expert whose dependencies later become unusable to remain visible and editable, so that I can repair it without recreating it.
9. As a User, I want an Unavailable Expert to be excluded from new execution, so that the platform fails before doing partial work.
10. As a User, I want execution to fail explicitly instead of silently replacing an unavailable model or engine, so that I know what actually ran.
11. As a User, I want Expert cards to show their Provider Model and Runtime Engine, so that I can compare Experts before selecting one.
12. As a User, I want Expert selectors to show model, engine, compatibility warnings, and availability, so that I can make an informed selection.
13. As a User, I want migrated Experts without a model or engine to appear as Incomplete Experts, so that legacy data is preserved without inventing execution choices.
14. As a User, I want an Incomplete Expert to remain editable but unavailable for new execution, so that migration cannot launch an unintended model or engine.
15. As a User, I want a single selected Expert to execute directly with its own model and engine, so that no coordinator changes its behavior or cost.
16. As a User, I want each Expert Team member to execute with its own model and engine, so that I can compose genuinely specialized execution environments.
17. As a User, I want an Expert Team to mix Provider Models and Runtime Engines, so that team composition is not limited to a common compatibility intersection.
18. As a User, I want Expert Teams to have no team-level model or engine override, so that each execution has one authoritative source.
19. As a User, I want a team's member list to display each member's model and engine, so that its execution plan is visible before use.
20. As a User, I want team members to continue running in their configured order, so that preceding results and file changes remain deterministic.
21. As a User, I want later team members to receive preceding members' final text results, so that mixed Runtime Engines can still collaborate.
22. As a Workflow owner, I want all team members in one turn to share the same temporary Workspace, so that later members can inspect earlier file changes.
23. As a Workflow owner, I want the persistent Workspace to advance only after every member succeeds, so that a failed mixed-engine turn cannot leave partial files.
24. As a User, I want the final team member to remain the official response author, so that mixed execution does not introduce an extra synthesis invocation.
25. As a User, I want one overall timeout to cover the complete team turn, so that adding members cannot multiply the allowed execution time.
26. As a User, I want cancellation to stop the active member and skip remaining members, so that I retain control of a long mixed-engine turn.
27. As a User, I want retry to restart from the first frozen member, so that results are based on one coherent execution chain.
28. As a User, I want each team member to have isolated Runtime context, credentials, Extensions, and Native Session state, so that one Expert cannot inherit another Expert's environment.
29. As a User, I want team Native Session states to advance only when the complete turn succeeds, so that a failed later member cannot partially advance conversation continuity.
30. As a User, I want each Expert's warm Runtime definition to be isolated from other members, so that warm reuse does not become context sharing.
31. As a User, I want warm Runtime definitions to be reused for 30 minutes within the same frozen conversation and Expert identity, so that isolation does not require needless recreation.
32. As a User starting a Session without an Expert, I want execution to use the current Personal Settings default model and engine, so that basic use requires no extra selection.
33. As a User continuing a no-Expert Session, I want a Personal Settings change to apply to my next message, so that I can change defaults without a Session-level control.
34. As a User starting a Session with an Expert, I want that Expert configuration frozen by the first message, so that later Expert edits cannot silently alter the conversation.
35. As a User regenerating a response, I want the original ordered Response Snapshot reused, so that regeneration preserves the original execution identity.
36. As a User configuring a Workflow without an Expert, I want each new Run Conversation to freeze the then-current Personal Settings defaults, so that all follow-up Runs remain coherent.
37. As a User configuring a Workflow with an Expert or Expert Team, I want each new Run Conversation to freeze the current Expert configuration, so that subsequent edits affect only future conversations.
38. As a User, I want Sessions and Workflows to omit model and engine selectors, so that they cannot conflict with Personal Settings or Expert configuration.
39. As an API client, I want Session and Workflow execution requests to omit model and engine override fields, so that the API enforces the same single-source rule as the UI.
40. As a User viewing a response, I want to see the final stage's model and engine and expand all stage identities, so that I can audit what produced the answer.
41. As a User viewing a team execution, I want each stage to show its Expert, model, engine, status, elapsed time, Credit Consumption, and final text, so that failures and costs are attributable.
42. As a Workflow API client, I want ordered member-stage execution identities and results returned with overall status, so that I do not have to infer mixed execution from logs.
43. As a User, I want historical snapshots to retain their original shared model and engine semantics, so that a migration does not rewrite what past conversations mean.
44. As a User, I want old mutable Session and Workflow overrides ignored for new execution after migration, so that hidden legacy settings cannot compete with the new rules.
45. As a User, I want a Model Provider Connection that is still needed by defaults, Experts, or continuable snapshots to be protected from deletion, so that valid conversations do not lose their credentials.
46. As a User launching manual Workflow execution with an unavailable Expert, I want an immediate error before a Run is created, so that the history is not polluted by preventable attempts.
47. As a Workflow API client invoking an unavailable Expert, I want an explicit error before a Run is created, so that automation can distinguish configuration failure from execution failure.
48. As a User with a Scheduled Trigger, I want an unavailable Expert to create an explicit failed, uncharged Run, so that a missed schedule remains auditable.
49. As a User, I want queued work to revalidate its first stage before model invocation, so that a dependency change while waiting fails safely and without model charges.
50. As a User, I want Credit Balance checked before every team member invocation, so that each model call follows the same account rule.
51. As a User, I want completed team stages charged at their own frozen Model Credit Rates, so that mixed-model cost is accurate and auditable.
52. As a User, I want unstarted members to remain uncharged when Credits run out mid-turn, so that I pay only for actual Provider Model invocations.
53. As a User, I want a mid-team insufficient-Credits failure to roll back Workspace and staged Native Session state, so that charged partial work does not become committed product state.
54. As a User, I want a retry after adding Credits to be a new fully charged execution from the first member, so that retry semantics remain deterministic.
55. As an Administrator, I want execution-level model choices to remain private User content, so that stage auditability does not weaken ownership boundaries.
56. As a mobile User, I want Expert and team execution identities to remain readable without hiding required controls or status, so that the feature works outside desktop layouts.

## Implementation Decisions

- The Expert aggregate gains required Provider Model and Runtime Engine references. Completeness requires a non-empty Execution Instruction and both execution references.
- Expert availability is separate from completeness. A complete Expert becomes unavailable when its Provider Model, Model Provider Connection, Runtime Engine, or Runtime Model Compatibility is unusable.
- Expert create and update validate ownership, current availability, and Runtime Model Compatibility. Verified pairs are accepted, unverified pairs are accepted with a warning, and incompatible pairs are rejected.
- Expert Team remains an ordered list of Expert references with no team-level Execution Instruction, Extensions, Provider Model, or Runtime Engine.
- Session and Workflow public inputs remove Provider Model and Runtime Engine overrides. Non-empty legacy override fields from old clients are rejected rather than ignored at the current API boundary.
- Execution without an Expert resolves the Personal Settings default Runtime Engine and its default Provider Model. A Session resolves this for every new message; a Workflow resolves it only when a new Run Conversation begins.
- A new shared execution-planning Application seam resolves no Expert, single Expert, and Expert Team inputs into one ordered set of Execution Stage Snapshots. Session and Workflow paths use the same resolver.
- Each Execution Stage Snapshot contains an optional Expert identity, Provider Model identity, Model Provider Connection identity and version, Provider type, Model API Protocols, Endpoint, Runtime Engine, Execution Instruction, exact MCP Server snapshots, exact Skill revisions, and the frozen Model Credit Rate needed by that invocation.
- Common execution data remains outside the stages: Session or Workflow input, Personality, bounded conversation context, attachments, Workflow environment, overall timeout, Git Source, and Workflow temporary Workspace identity.
- A no-Expert execution has exactly one anonymous stage. A single Expert has exactly one Expert stage. An Expert Team has exactly two through ten stages in frozen member order.
- New Response Snapshots and Workflow Snapshots persist the ordered stage representation. Mixed teams have no synthetic top-level Provider Model or Runtime Engine.
- Response regeneration reuses the original ordered Response Snapshot. Follow-up Runs reuse the initiating Workflow Snapshot.
- The legacy snapshot decoder recognizes the prior schema and maps its shared top-level Provider Model and Runtime Engine across its historical Expert members without consulting current mutable Expert data. Historical JSON is not rewritten.
- An additive immutable database migration adds Expert Provider Model and Runtime Engine references, leaves existing rows nullable for migration, and treats missing values as Incomplete Expert state.
- The migration clears legacy mutable Session current-model and Workflow model/engine override values. Runtime behavior stops reading those values for new execution. Existing immutable snapshots remain untouched.
- The public generated contract, generated server bindings, OpenAPI, and generated frontend types change together. Generated files remain outputs of the existing generation commands.
- Runtime execution iterates over resolved stages. For every stage it loads that stage's Runtime configuration, Adapter, Model Provider credential version, Extensions, and isolated Runtime state.
- Runtime Adapters continue receiving one invocation at a time through the common Describe and Execute contract. Runtime brand differences remain inside Drivers; the platform orchestrator, not a Driver, owns Expert Team behavior.
- External Runtime commands remain executable-plus-argument arrays. Stage-specific User content and credentials never become shell fragments or command-line secrets.
- Warm Runtime identity includes owner, Session or Run Conversation, frozen Expert identity or anonymous-stage identity, Runtime Engine, and image digest. Different team members never share a warm execution context.
- Team stages mount the same per-turn Workflow temporary Workspace sequentially. Only complete team success merges it into the persistent Workspace.
- Native Session state is stored per frozen member. Each turn operates on temporary copies and atomically promotes all copies only after every member and final persistence step succeed.
- Rolling Summary, recent context, and preceding member final results remain the correctness boundary. Native Resume remains a conformance-gated optimization.
- The existing single overall team timeout, fail-fast ordering, cancellation behavior, final-member response, and whole-team retry behavior remain unchanged.
- Stage progress and terminal records add Provider Model and Runtime Engine execution identity, usage, and Credit Consumption. They do not expose private reasoning or raw Runtime events as primary UI content.
- The final response summary uses the final stage's execution identity. Detailed views and Workflow API responses expose the complete ordered stage list.
- Credit Balance is checked immediately before each stage's Provider Model invocation. Every started invocation settles measured usage or its frozen fallback independently.
- If Credits become insufficient before a later stage, the turn fails before that invocation. Completed stages remain charged, unstarted stages are uncharged, and Workspace plus staged Native Session changes are discarded.
- Manual and Workflow API execution with a currently unavailable Expert fails before creating a Run. Scheduled execution creates an explicit uncharged failed Run. Queued execution revalidates before its first Provider Model invocation.
- A Model Provider Connection cannot be deleted while referenced by a Personal Settings default, a mutable Expert, or a continuable frozen Session or Run Conversation. This spec does not overload deletion with a new disabled state.
- Expert and team catalog responses expose completeness, availability, selected model, selected Runtime Engine, and compatibility state needed by selectors and cards.
- Expert editing presents model and engine together. Session and Workflow surfaces present resolved execution identity as read-only metadata and contain no execution override controls.
- Desktop and mobile layouts preserve the same execution information order; mobile may stack it but cannot hide model, engine, compatibility, or availability.
- The accepted Expert-owned execution ADR supersedes only the earlier decision that all Expert Team members share one Provider Model and Runtime Engine. Platform-managed sequential orchestration and its other constraints remain accepted.

## Testing Decisions

- Tests assert externally observable planning, invocation, persistence, API, and UI behavior. They do not lock helper function names, internal loop shape, JSON marshal implementation, CSS structure, or Driver internals unrelated to their public contract.
- The primary seam is a shared Workspace Application execution planner. Table-driven tests pass owned Personal Settings, Expert or Expert Team state, compatibility, and legacy snapshots into this seam and assert the complete ordered stage plan or classified failure.
- Planner tests cover no-Expert Session resolution per message, no-Expert Workflow resolution per Run Conversation, single Expert resolution, mixed team ordering, required fields, owner isolation, verified/unverified/incompatible pairs, Incomplete Expert, Unavailable Expert, no fallback, connection versions, exact Extension revisions, and legacy snapshot decoding.
- Planner tests prove Session and Workflow use the same resolution rules and that mutable Expert or Personal Settings changes do not alter already-frozen snapshots.
- Existing Runtime Executor tests remain the orchestration seam. Fake Adapters record the selected Runtime Engine, model, Endpoint, protocols, credential reference, instruction, Extensions, workspace, and checkpoint for every stage.
- Runtime Executor tests cover mixed models and engines, strict member order, preceding-result handoff, shared temporary Workspace visibility, isolated stage credentials and Extensions, final-member response, one overall timeout, cancellation, fail-fast, and terminal persistence.
- Runtime Executor tests cover warm identity separation for different Experts using the same engine, warm reuse for the same frozen stage, image-digest changes, unavailable Runtime rejection, and configuration drift fail-closed behavior.
- Native Session tests cover independent member state, temporary state copies, atomic all-member promotion, failure and cancellation discard, retry from the preceding successful turn, and Rolling Summary correctness when Resume is unavailable.
- Credit tests cover per-stage balance checks, different frozen Model Credit Rates in one team, completed-stage settlement, unstarted-stage zero charge, insufficient Credits before a later member, Workspace rollback, Native Session rollback, retry charging, and scheduled uncharged failure before invocation.
- Repository integration tests cover the additive migration, nullable legacy Experts, new required Expert writes, cleared mutable overrides, new snapshot persistence, legacy snapshot reads, stage audit records, connection deletion restrictions, and aggregate-plus-event transactional commits.
- Generated HTTP contract tests cover required Expert model and engine fields, rejected incompatible pairs, removed Session and Workflow override fields, strict rejection of stale non-empty fields, availability errors, ordered stage metadata, and non-enumerating ownership behavior.
- Existing Vue page component tests cover Expert create/edit fields, suggested defaults with explicit confirmation, compatibility warnings, unavailable and incomplete cards, mixed team member summaries, removed Session and Workflow selectors, read-only response identity, stage detail expansion, localized errors, and responsive information ordering.
- Prior art includes the existing table-driven Domain validation tests, Runtime Executor recording Adapter and recording progress fakes, strict generated ProtoJSON HTTP binding tests, Expert editor tests, Session conversation tests, Workflow detail tests, and team fail-fast/Workspace rollback tests.
- Targeted Go package tests run first for the Domain/Application planner, repository, service contract, and Runtime Executor. The affected full gates then run: generated-output verification, all backend tests, backend build, frontend typecheck, frontend unit tests, and frontend production build.
- Migration and remote Provider integration skips are reported as skips, not passes. Runtime image, Linux sandbox, and production conformance remain required before claiming model-engine pairs or Native Resume behavior for deployed image digests.

## Out of Scope

- Team-level Provider Model or Runtime Engine overrides.
- Session-level or Workflow-level Provider Model or Runtime Engine selectors.
- Optional or inherited Expert execution configuration.
- Automatic fallback to another model, connection, protocol, Runtime Engine, Expert, or team member.
- Parallel Expert execution, dynamic routing, conditions, loops, arbitrary graphs, or free-form inter-agent conversation.
- A coordinator or extra synthesis model invocation after the final team member.
- Runtime-native subagent orchestration.
- Per-member Workflow Workspaces or merging partial Workspace changes after failure.
- Changing the existing two-to-ten Expert Team size, ordered execution, whole-team retry, or overall timeout rules.
- Reinterpreting or rewriting historical snapshot JSON with current Expert values.
- Introducing a disabled Model Provider Connection lifecycle; deletion remains restricted while references exist.
- Commit, Push, Review Branch, pull request, or merge request workflows.
- Claiming Runtime compatibility, Usage support, Native Resume, sandbox isolation, or production readiness without the required conformance evidence for exact image digests.

## Further Notes

- The current implementation stores one top-level Provider Model and Runtime Engine in execution snapshots and resolves one Runtime before iterating an Expert Team. This is an implementation baseline, not the target behavior.
- The current Session implementation also differs from the former product wording: it does not provide a complete Runtime switch path even though older requirements described one. Removing Session and Workflow execution selectors resolves that ambiguity instead of adding another selector.
- Historical snapshots are immutable evidence. Backward compatibility belongs in explicit schema decoding, not in backfilling from current mutable Experts.
- The model and engine pair is part of Expert configuration, but connection secrets remain protected versioned credentials and never enter ordinary snapshot JSON.
- Existing unrelated Credits and usage work in the worktree must be preserved. This feature integrates at the per-invocation Credit seam and must not overwrite or weaken the Credit Ledger decisions.
- Completion requires actual tests and acceptance evidence. Parsing new fields or constructing mixed stages is not evidence that a Runtime image can execute a given model-engine pair or safely Resume it.
