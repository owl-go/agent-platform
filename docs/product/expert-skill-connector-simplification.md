# Expert, Skill, and Connector Simplification

Status: accepted for implementation

## Problem Statement

Users currently have to configure a Provider Model, Runtime Engine, one broad Execution Instruction, hand-authored tags, MCP Servers, and Skills inside every Expert. Skills and MCP Servers are also buried under Personal Settings as Extensions. This makes a reusable specialist profile responsible for execution infrastructure, discovery metadata, and integrations at the same time, and makes Expert Teams inherit unnecessary model and Runtime complexity.

Users need Experts to describe what a specialist knows and how it works, Skills and Connectors to be reusable resources in their own catalog, and execution configuration to remain a Personal Settings concern. They also need direct third-party CLI integrations such as the official Feishu CLI without allowing arbitrary Users or Runtime processes to install and execute ungoverned packages.

## Solution

The product exposes two top-level catalogs: `Experts` (Chinese: `专家`), containing Expert and Expert Team tabs, and `Skills & Connectors` (Chinese: `技能·连接器`), containing Skill and Connector tabs. The User-visible Extension concept is removed; the catalog is not renamed to Capability.

An Expert becomes an engine-independent specialist profile. Its Core Capability, Operating Procedure (shown as `工作流程`), Output Standard, and optional Cautions form the complete visible instruction injected for that Expert. Introduction remains display-only. Provider Model and Runtime Engine are resolved from Personal Settings and frozen when a Session or Run Conversation starts.

An Expert Team becomes an ordered set of stable Team Member roles. Each member has its own name and Member Labels, may reference the same Expert as another member, and executes in an isolated context. All members use the execution configuration frozen for the Session or Run Conversation.

Skills remain User-owned packages. Connectors have two ownership models: Users create private MCP Connectors, while Administrators create platform-wide CLI Connector Definitions for exact, verified npm packages. Users enable and authorize available CLI Connectors without gaining permission to create CLI Definitions. The official Feishu CLI is the first real CLI Connector.

High-risk CLI commands require a time-bounded, one-use approval from the owning User. A Session response or Run may enter a persisted `waiting_for_user` state and resume after approval without weakening the existing event, transaction, cancellation, workspace, credit, or secret boundaries.

## User Stories

1. As a User, I want Experts and Expert Teams grouped in one catalog, so that specialist profiles are easy to find.
2. As a User, I want Skills and Connectors grouped in a separate top-level catalog, so that reusable capabilities are not hidden in Personal Settings.
3. As a User, I want separate Skill and Connector tabs, so that instructions and executable integrations remain conceptually distinct.
4. As a User, I want the old Extension terminology removed, so that I do not have to understand a redundant umbrella concept.
5. As a User, I want to create an Expert without choosing a model or Runtime Engine, so that the Expert remains reusable across my execution defaults.
6. As a User, I want an Expert Icon with a stable default, so that Experts are visually distinguishable without managing image files.
7. As a User, I want an Expert Introduction, so that I can understand the profile before selecting it.
8. As a User, I want to describe an Expert's Core Capability, so that the specialist's area of competence is explicit.
9. As a User, I want to define an Expert's Operating Procedure, so that it follows a repeatable working method.
10. As a User, I want to define an Expert's Output Standard, so that its results have a predictable form and quality.
11. As a User, I want to record optional Cautions, so that known constraints and pitfalls are considered.
12. As a User, I want every injected Expert instruction to remain visible and editable, so that the platform does not hide role prompts from me.
13. As a User, I want the product to derive discovery tags from Core Capability, so that I do not maintain duplicate metadata.
14. As a User, I want tag generation failure not to block Expert editing or execution, so that catalog enrichment is never a hard dependency.
15. As a User, I want to search Experts by name, Introduction, and derived tags, so that I can find the right specialist quickly.
16. As a User, I want to select existing Skills and Connectors for an Expert, so that capability configuration is reusable.
17. As a User, I want to upload or install a Skill while editing an Expert, so that I can complete setup without losing context.
18. As a User, I want a newly created resource to enter my global catalog and be selected automatically, so that the Expert does not own a private duplicate.
19. As a User, I want a migrated incomplete Expert to remain visible and editable, so that no authored instruction is lost.
20. As a User, I want Personal Settings to supply execution configuration for every new Session or Run Conversation, so that model and Runtime selection has one owner.
21. As a User, I want a Session or Run Conversation to freeze its resolved execution configuration, so that later settings changes do not alter an active conversation.
22. As a User, I want an execution configuration problem reported when execution starts, so that infrastructure availability does not incorrectly mark an Expert as broken.
23. As a User, I want to create an Expert Team with an Icon, Introduction, and combined Core Capability summary, so that the team is understandable as a reusable profile.
24. As a User, I want to give each Team Member a role name, so that the same Expert can serve different purposes in one team.
25. As a User, I want to add Member Labels, so that each member's team-specific responsibility is explicit.
26. As a User, I want member role context injected visibly, so that role names and labels affect behavior rather than acting as decoration.
27. As a User, I want to reuse the same Expert in multiple Team Member positions, so that one specialist can review work it produced in an isolated pass.
28. As a User, I want each Team Member to have stable identity and isolated execution state, so that renaming or reordering cannot mix Native Sessions.
29. As a User, I want to reorder Team Members, so that the sequential collaboration reflects my intended workflow.
30. As a User, I want deletion of a referenced Expert rejected, so that an Expert Team is never silently rewritten.
31. As a User, I want Expert and Expert Team cards to omit model and Runtime details, so that the catalog focuses on specialist capability.
32. As a User, I want Expert cards to show Skill and Connector counts, so that I can understand attached capabilities at a glance.
33. As a User, I want team cards to show member roles and per-turn member count, so that team behavior and likely cost are visible.
34. As a User, I want to install Skills from a Git URL or ZIP, so that both maintained and local packages are supported.
35. As a User, I want a Skill update to affect only future Session and Run Conversation snapshots, so that history remains reproducible.
36. As a User, I want to create, test, edit, and delete private MCP Connectors, so that external MCP tools remain under my ownership.
37. As a User, I want an MCP Connector to pass its isolated test before selection, so that invalid configurations fail before execution.
38. As a User, I want resource deletion to show affected Experts and detach only mutable references, so that I understand the impact while historical snapshots remain intact.
39. As an Administrator, I want to create a CLI Connector Definition from an exact npm package and version, so that Users can access governed third-party CLIs.
40. As an Administrator, I want CLI packages built and verified outside User Runs, so that Runtime execution never performs an unbounded network installation.
41. As an Administrator, I want a CLI Definition to remain unavailable until its bundle and declared Runtime Digests pass Conformance, so that parsed metadata is not mistaken for supported capability.
42. As an Administrator, I want CLI Definitions to move through draft, building, testing, available, failed, and disabled states, so that publication is explicit.
43. As an Administrator, I want to define structured capability groups, identities, command patterns, risk, scopes, and Egress, so that a CLI package does not receive implicit unrestricted access.
44. As an Administrator, I want authentication chosen from built-in drivers, so that a CLI Definition cannot introduce arbitrary authentication scripts.
45. As an Administrator, I want upstream CLI schema changes reviewed before enabling a new version, so that new commands or scopes are not exposed automatically.
46. As a User, I want to browse an available CLI Connector before enabling it, so that discovery does not require authorization.
47. As a User, I want enabling the Feishu CLI to create one Feishu application for my account, so that I do not manually copy an App ID and App Secret.
48. As a User, I want repeated or concurrent enablement to return the same Feishu application, so that retries cannot create duplicates.
49. As a User, I want the Feishu application name shown exactly as returned by Feishu, so that the product does not claim an unsupported automatic rename.
50. As a User, I want a link to the Feishu developer console, so that I can rename the application manually if desired.
51. As a User, I want enablement to request only reviewed, non-business, review-free base permissions, so that initial access remains minimal.
52. As a User, I want business scopes requested only when enabling a capability or approving an operation, so that authorization follows least privilege.
53. As a User, I want to connect more than one Feishu account to my single Feishu application, so that I can use distinct external identities.
54. As a User, I want User and Bot execution identities distinguished, so that external actions clearly identify who performs them.
55. As a User, I want to choose User or Bot identity when approving an operation that supports both, so that actions are attributed correctly.
56. As a User, I want missing User scopes to start an explicit OAuth flow, so that User authority is never inferred from Bot authority.
57. As a User, I want missing Bot scopes or publication prerequisites to produce a direct recovery link, so that I can repair the application deliberately.
58. As a User, I want Connector credentials encrypted and write-only, so that neither another User nor an Administrator can inspect them.
59. As a User, I want expired tokens refreshed before execution, so that normal token rotation does not require editing an Expert.
60. As a User, I want failed refresh to mark an authorization invalid, so that execution cannot silently fall back to another identity.
61. As a User, I want to disconnect an authorization, so that future execution loses access immediately while historical results remain readable.
62. As a User, I want recommended CLI Skills installed as ordinary explicit Skills, so that the Connector never injects hidden instructions.
63. As a User, I want an Expert without a compatible recommended Skill to show a warning rather than fail, so that instructions remain my choice.
64. As a User, I want each high-risk CLI command to require approval, so that broad authorization does not imply consent for every action.
65. As a User, I want an approval to display Connector, identity, operation, target, and redacted parameters, so that I know exactly what I am approving.
66. As a User, I want one approval to authorize only one immutable command request, so that consent cannot be replayed for different parameters.
67. As a User, I want a Run to request approval more than once, so that multi-step work can proceed without granting permanent permission.
68. As a User, I want approval requests serialized within an Execution Stage, so that I can evaluate one high-risk action at a time.
69. As a User, I want to reject a command without necessarily cancelling the entire Run, so that the Runtime may offer a safer alternative.
70. As a User, I want an unanswered approval to time out, so that execution cannot wait forever.
71. As a User, I want to cancel while execution waits for me, so that I retain control of the Session or Run.
72. As a User, I want Scheduled and API Runs to expose the same waiting state, so that I may approve them from the product before their deadline.
73. As an API caller, I want Workflow credentials prohibited from approving commands, so that invocation authority cannot become User consent.
74. As a User, I want a waiting execution to resume from the same Runtime and temporary Workspace, so that approved work does not restart unpredictably.
75. As a User, I want waiting time excluded from the ordinary execution timeout, so that the confirmation deadline is the one visible source of truth.
76. As a User, I want actual model Usage charged even if a later approval expires, so that accounting remains based on work already performed.
77. As a User, I want revocation or Connector disablement rechecked immediately before command execution, so that a stale approval cannot bypass a new restriction.
78. As an Administrator, I want aggregate Connector health without User secrets or content, so that the platform can be operated without violating User ownership.
79. As a User, I want desktop and mobile interfaces for both catalogs and approval flows, so that capability management is not desktop-only.
80. As an operator, I want production evidence tied to exact Runtime and Connector bundle Digests, so that availability is based on tested artifacts.

## Implementation Decisions

- Replace the User-visible Expert/Extension structure with two top-level catalogs: Experts and Skills & Connectors. Personal Settings no longer embeds resource management.
- Use preset Profile Icons and background colors with stable defaults. User-uploaded profile images are not introduced.
- Replace Capability Introduction with display-only Introduction.
- Replace Execution Instruction with four explicit fields. Core Capability, Operating Procedure, Output Standard, and Cautions are assembled under fixed visible headings and are the complete Expert role instruction.
- Keep the interface label `工作流程`, but use `operating_procedure` in the domain and API so it cannot be confused with the Workflow aggregate.
- Require Expert name, Introduction, Core Capability, Operating Procedure, and Output Standard. Cautions, Skills, and Connectors are optional.
- Project up to five Derived Expertise Tags asynchronously from Core Capability by using the User's current Personal Settings default Provider Model. Tag generation is non-blocking, non-authoritative, not injected, and does not consume User Credits. A failed refresh retains the previous projection.
- Remove Provider Model and Runtime Engine from mutable Expert data and API contracts. Resolve the Personal Settings default Runtime and its Provider Model once at Session or Run Conversation creation and freeze the result in ordered Execution Stage Snapshots.
- A no-Expert execution, a single Expert, and all members of an Expert Team use the same frozen execution configuration for that Session or Run Conversation. Historical snapshots retain their original schema and model/Runtime semantics.
- Replace Expert Team's ordered Expert ID array with ordered Team Member records. Each record has a stable member ID, team-unique name, Expert reference, zero to five Member Labels, and position.
- Permit the same Expert reference in more than one Team Member. Isolate Native Session and Runtime context by stable member identity rather than Expert ID or display name.
- Inject Team Member name and labels as visible role context before that member's Expert guidance. Keep team Introduction and team Core Capability display-only.
- Reject deletion of an Expert referenced by any mutable Expert Team. Immutable snapshots do not block deletion.
- Rename MCP Server to MCP Connector at User-visible and new public API boundaries. User ownership, isolated testing, fixed stdio package version, Streamable HTTP restrictions, and snapshot behavior remain fail closed.
- Move public Skill and MCP routes to `/api/v1/skills` and `/api/v1/connectors/mcp`. Do not keep deprecated `/extensions` aliases.
- Continue selecting Skill and MCP Connector identities from mutable Experts. New Session and Run Conversation snapshots freeze their latest exact revisions; existing snapshots do not change.
- On Skill or MCP Connector deletion, show affected mutable Experts, detach those references transactionally after confirmation, and preserve immutable snapshots.
- Introduce an Administrator-owned CLI Connector Definition and User-private CLI Connector Enablement and Authorization records. Ordinary Users cannot create or edit CLI Definitions.
- Allow an Administrator to submit an exact npm package and version, but never a Shell install command. Choose the executable from npm `bin` metadata and authentication from built-in drivers.
- Build each CLI bundle in an isolated, credential-free builder; verify npm integrity and final SHA-256; store the immutable bundle in object storage; mount it read-only for a Run. A Run never performs package installation.
- Define CLI capabilities as structured identity, argv pattern, risk, scope, and Egress policy. Place a single common Wrapper in front of the real executable so the five Runtime Drivers do not duplicate enforcement.
- Freeze CLI Definition version, bundle digest, capability policy, and Authorization identity in the execution snapshot. Never freeze App Secrets or Tokens in ordinary snapshots.
- Resolve current encrypted credentials before every execution, materialize them only for that invocation, include their exact bytes in the common redaction set, and clean them idempotently.
- Make CLI availability specific to an exact CLI bundle digest and Runtime image RepoDigest. Configuration or parsing alone never enables a combination.
- Build and publish a new CLI version only after its generated schema is compared with the preceding reviewed command and scope set. Upstream additions remain unavailable until accepted explicitly.
- For Feishu, use the official fixed-version `@larksuite/cli` package and its app-registration/device flows. Enabling creates exactly one Feishu CLI Application per User and encrypts its returned App ID and App Secret.
- Treat the remote Feishu application name as provider-owned display metadata. Read and show the actual name and offer a developer-console link; do not claim or require automatic renaming.
- Allow multiple Feishu account authorizations under the one User application. Account Tokens are isolated; application permissions and Bot identity are shared by that User application.
- At enablement, request only a platform-reviewed subset of officially review-free scopes required for Connector identity and diagnostics, with no business-data access. Capability use requests the smallest additional scopes.
- Support User and Bot identities. A high-risk operation that supports both asks the User to choose during approval and verifies the corresponding scopes and application prerequisites before execution.
- Generate recommended Skills as explicit install offers. They become ordinary User-owned Skills and are never injected merely because a CLI Connector is enabled.
- Add `waiting_for_user` as a non-terminal Session response and Run state. Persist an approval request before exposing it, then transition back to generating/running after its one-use decision.
- Permit only the authenticated owning User to approve or reject. Administrator identity and Workflow API credentials have no approval authority.
- Serialize high-risk approval requests per Execution Stage. Bind every decision to a nonce and a digest of Connector, identity, executable, argv, target, policy version, and expiry.
- Keep the Runtime container, temporary Workspace, Workflow serialization lock, and Credit execution lease while waiting. Pause the ordinary execution timeout and apply the Definition's approval timeout, defaulting to five minutes and capped at fifteen minutes.
- A rejected or expired approval returns a structured CLI error to the Runtime; it does not force the overall Run terminal state. Settle any model Usage that occurred before the decision.
- Revalidate Definition, Enablement, Authorization, scopes, and policy immediately after approval and before starting the real process. Disablement or revocation closes pending requests and blocks execution.
- Preserve the event contract: Run ID matches, sequence starts at one and increases monotonically, exactly one terminal event exists, and no event is published after terminal state. User-action events are non-terminal.
- Migrate old Capability Introduction into Introduction and old Execution Instruction unchanged into Operating Procedure. Leave new Core Capability, Output Standard, and Cautions empty; migrated Experts remain incomplete until required fields are supplied.
- Stop reading legacy Expert Provider Model, Runtime Engine, and hand-authored tags for new execution. Retain columns during the compatibility period and keep historical snapshot readers unchanged.
- Give migrated Experts and Expert Teams default Profile Icons. Convert old team member order into stable Team Member records with generated IDs, Expert names as initial member names, and no Member Labels.
- Supersede the decision that Experts own Provider Model and Runtime selection. Retain sequential fail-fast team execution, final-member official response, shared temporary Workflow Workspace, success-only merge, and full-team retry.

## Testing Decisions

- Prefer externally visible behavior over storage or helper implementation details. Tests should assert ownership, accepted and rejected state transitions, snapshots, events, rendered API responses, and executable requests rather than private function calls.
- Use the authenticated Workspace HTTP service as the primary acceptance seam. Inject fake model, tag generator, npm builder, Feishu registration/OAuth, object store, Runtime, approval clock, and command process ports so complete flows remain deterministic and never contact external services.
- Keep focused Domain tests for Expert required fields, instruction assembly inputs, Derived Expertise Tag limits, Team Member identity and uniqueness, repeated Expert references, CLI Definition validation, capability patterns, and approval state transitions.
- Keep Repository contract tests for incremental migration, owner isolation, Administrator-only CLI Definition writes, one-application-per-User idempotency, encrypted credential versions, optimistic concurrency, Expert deletion conflicts, transactional detach, snapshot freezing, approval nonce consumption, timeout, cancellation, and terminal-event invariants.
- Extend existing execution planning tests so anonymous, single-Expert, and team stages all use the one Personal Settings execution configuration while retaining distinct Expert and Team Member identities.
- Extend Runtime Executor tests so only the four explicit Expert guidance fields are injected in order, Introduction and derived tags are omitted, member role context is visible, and each stage receives only its own frozen Skills and Connectors.
- Add a shared CLI Wrapper contract suite using a harmless fixture package. Cover exact package digest, executable discovery, allowed and rejected argv, argument mutation after approval, identity selection, Egress rejection, timeout, cancellation, output limits, structured errors, Secret canaries, Workspace boundaries, and revalidation after revocation.
- Reuse the existing recording/fake process seam; tests must never invoke a real third-party CLI, npm registry, browser, or external provider.
- Add Worker tests for running to waiting to running, multiple sequential approvals, rejection, expiry, cancellation, disabled User, process restart/reconcile, Credit lease heartbeat, and no events after terminal state.
- Add API tests proving that only the owning User can approve and that Administrator and Workflow API credentials cannot inspect or mutate User authorizations or approvals.
- Add Vue component/page tests for both catalog tab sets, Expert structured fields, inline resource creation, Team Member editing and reorder controls, Connector enablement, Feishu registration links, permission recovery, waiting cards, identity selection, expiry, mobile layout, and inaccessible actions.
- Run target Go package tests first, then `make test` and `make build`. Run `make web-typecheck` and `make web-build` for all Web changes.
- Extend Production Conformance to cover the exact Feishu CLI bundle and each claimed Runtime RepoDigest on Linux with `runsc`. Include non-root/read-only Rootfs, bundle read-only mount, approved and rejected commands, real authorization only when credentials are explicitly provided, Egress, cancellation, timeout, Secret redaction, and Workspace writes.
- Treat missing Feishu credentials, Docker, Linux, `runsc`, or provider access as skipped environment gates, never as passing evidence. CLI Connector availability remains false until the required evidence exists.

## Out of Scope

- User-created or User-uploaded CLI Connector Definitions.
- Arbitrary Shell installers, arbitrary executable paths, authentication scripts, or Wrapper scripts supplied through the API.
- Runtime installation from npm during a Session response or Run.
- Implicit installation or hidden injection of recommended Skills.
- User-uploaded Expert or Expert Team profile images.
- Expert-specific or Team Member-specific Provider Model and Runtime Engine overrides.
- Parallel Expert Team execution, routing, loops, coordinator synthesis, or Runtime-native subagent requirements.
- Permanent approval grants, Run-wide approval of a command class, or approval by Administrator and Workflow API credentials.
- Automatic Feishu application renaming.
- Claiming production readiness without exact bundle and Runtime Digest Conformance evidence.

## Further Notes

- The official Feishu CLI sources are [`larksuite/cli`](https://github.com/larksuite/cli) and [`@larksuite/cli`](https://www.npmjs.com/package/@larksuite/cli). It is an Agent-oriented direct CLI with structured output, schema introspection, User and Bot identities, application registration, and device authorization. Its npm installation wizard and local Keychain behavior are reference behavior only; managed execution uses the platform's bundle, credential, and sandbox boundaries.
- MCP classification depends on the invocation protocol, not the package manager. An `npx` process speaking MCP is an MCP Connector; an npm-installed executable invoked directly through argv is a Third-party CLI Connector.
- One Feishu application per Agent Workspace User means application-level scopes are the union needed by that User's enabled Feishu capabilities. Wrapper allowlists and account Tokens remain narrower execution boundaries.
- A waiting Scheduled or API Run may be approved only by the User from the authenticated product interface. Without that action, its command approval expires normally.
- ADR-0026 supersedes the Expert-owned execution decision and extends the platform-managed Connector boundary. Existing historical snapshots remain an explicit compatibility path rather than being rewritten.
