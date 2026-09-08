# Generic Connector Authorization And Operations

## Problem Statement

Agent Workspace currently exposes Third-party CLI Connector authorization through Feishu-specific application, authorization, command, error, and frontend behavior. A User can be asked to authorize merely by selecting a Connector, before any protected capability is needed. When authorization is required during a conversation, the User may receive only an error code or instructions to leave the conversation and find another management screen. Authorization buttons and completed grants can fail to resume the operation, and raw command activity can appear where the User expects a concise business summary.

These failures are symptoms of a broader product boundary problem. Connector enablement, provider setup, external account authorization, operation approval, execution, and presentation are partly coupled to one package, one provider, and shell command text. Adding another CLI or an MCP Connector with authorization would require more provider-specific branches in the API, Worker, Runtime preparation, and frontend. README prose cannot safely define executable commands, permissions, credential handling, or UI behavior, while the current system does not provide a reviewed structured contract that tells the Agent how to use a Connector.

The platform needs one generic Connector model that covers CLI and MCP transports without sharing credentials between Connectors. Authorization must appear only when a conversation actually invokes a capability that needs it, and the resulting User action must remain visible and actionable in that conversation. Execution and security behavior must come from a frozen, machine-readable contract rather than provider names, raw errors, README text, or shell parsing.

## Solution

Introduce a versioned Connector Manifest that describes reviewed capabilities, structured inputs, User-visible operation semantics, Authorization Scheme, Connector Permissions, risk, Egress, execution limits, credential delivery, and idempotency. Creation or discovery produces a Manifest Draft. Schema validation, trusted-reference validation, transport Conformance, and Administrator review freeze an exact Resolved Connector Manifest for publication. An exact-version platform Profile is authoritative when one exists; otherwise a package Manifest may be used. Discovery output is never authoritative for missing security fields.

Use platform-registered Authorization Adapters for provider protocols. A Connector references an Authorization Scheme identifier but cannot ship executable authorization code. Each User and Connector has at most one active Connector Authorization. Connector Setup such as App ID and App Secret remains separate from account Authorization such as granted Permissions and Tokens, but both are private to that exact User and Connector and cannot be reused by another CLI or MCP Connector.

Selecting a Connector supplies the Agent with a compact Connector Capability Index and does not begin authorization. The Agent may request the frozen Capability Contract and reviewed Connector Usage Guide through a local, read-only `connector.describe` operation. The Agent invokes capabilities with structured input. The CLI Wrapper validates that input and renders a fixed argv array through a restricted declarative mapping; the MCP Gateway validates the same input model before invoking the frozen tool.

Immediately before a protected Connector Operation, the Connector Broker revalidates the frozen definition, enablement, Setup, Authorization, Permissions, risk, approval, Egress, and execution policy. Missing User work creates a typed Connector Action Requirement in the active conversation and pauses the same execution. The frontend renders only generic actions supplied by the platform, including opening a short-lived platform authorization URL, displaying or copying a code, checking status, and editing Setup. Successful verification resumes the same execution without another model invocation. High-risk CLI and MCP operations share one-use Connector Operation Approval.

CLI Wrapper and MCP Gateway emit typed Connector Operation events with business summaries. Conversations show concise requested, waiting, started, and terminal activity; raw argv and other implementation diagnostics remain in an explicit, Secret-redacted detail view. Recovery respects the frozen capability's idempotency policy. A write whose external outcome cannot be established becomes `outcome_unknown` and is not retried blindly.

Feishu becomes the first registered Authorization Scheme and exact-version Connector Profile under this model. Existing Feishu data and endpoints migrate in stages behind compatibility adapters. Once generic behavior and migration evidence pass, provider-specific branches are removed. Records that cannot be migrated safely require a new authorization rather than inferred credentials or silent fallback.

## User Stories

1. As a User, I want to browse an available Connector without authorizing it, so that I can understand its capabilities before granting access.
2. As a User, I want to select a Connector without immediately seeing an authorization prompt, so that setup does not interrupt an empty conversation.
3. As a User, I want authorization requested only when an operation needs a protected Permission, so that consent is tied to a concrete purpose.
4. As a User, I want a Connector capability that needs no Authorization to run without an authorization prompt, so that public operations remain frictionless.
5. As a User, I want a missing Setup prompt to appear in the conversation that needs it, so that I do not have to search the Connector catalog for recovery controls.
6. As a User, I want a missing Authorization prompt to appear in the conversation that needs it, so that I understand why execution paused.
7. As a User, I want an authorization card to identify the Connector, operation, requested Permissions, and external identity context, so that I can make an informed decision.
8. As a User, I want the authorization card to contain a working button and copyable link, so that popup behavior cannot leave me without a recovery path.
9. As a User, I want the authorization link to be short-lived and usable only once, so that an old conversation link cannot grant access later.
10. As a User, I want a verified provider callback to resume the same execution automatically, so that I do not have to send another model message.
11. As a User, I want “check status” or an equivalent recovery action when callbacks are unavailable, so that I can resume after completing authorization in another tab.
12. As a User, I want a completed Authorization to be recognized immediately, so that the platform does not repeatedly claim that I am unauthorized.
13. As a User, I want provider setup fields presented in a consistent form, so that different Connectors do not invent unrelated setup screens.
14. As a User, I want Setup Secrets to remain write-only, so that neither the Agent nor another User can read my App Secret.
15. As a User, I want each Connector to keep its own Setup and Authorization, so that authorizing one integration cannot expose credentials to another integration from the same provider.
16. As a User, I want at most one active external account per Connector, so that the account used by an operation is predictable.
17. As a User, I want credential refresh for the same external identity to preserve my conversation binding, so that routine Token rotation does not interrupt work.
18. As a User, I want an account change to require an explicit switch flow, so that existing conversations cannot silently start using another account.
19. As a User, I want a conversation bound to an old account to pause after that account is replaced, so that account switching is visible.
20. As a User, I want a request for additional Permissions to explain the new access and its purpose, so that authorization expansion is informed.
21. As a User, I want Permission expansion to require explicit consent, so that the platform cannot silently enlarge a grant.
22. As a User, I want a provider response for a different account during Permission expansion to enter the account-switch flow, so that it cannot replace my identity accidentally.
23. As a User, I want an expired Token refreshed automatically when safe, so that ordinary credential maintenance does not interrupt execution.
24. As a User, I want refresh failure reported as a clear action such as expired, revoked, or provider unavailable, so that I know what recovery is required.
25. As a User, I want a revoked Authorization to block future operations immediately, so that disconnected access cannot continue.
26. As a User, I want a high-risk operation to show the final external account before confirmation, so that I know which identity will act.
27. As a User, I want authorization resolved before high-risk operation approval, so that the approval card describes the real execution identity.
28. As a User, I want an approval to cover exactly one operation, target, and input, so that it cannot authorize later commands.
29. As a User, I want changed input or target to require a new approval, so that the approved action cannot be altered after consent.
30. As a User, I want the same approval experience for risky CLI and MCP operations, so that transport details do not change safety expectations.
31. As a User, I want rejection to return a structured cancellation to the current execution, so that the Agent can explain that no operation ran.
32. As a User, I want rejecting one operation to leave the Connector enabled, so that a single refusal does not erase my configuration.
33. As a User, I want an ignored action to expire at a fixed deadline, so that executions do not wait indefinitely.
34. As a User, I want expired links and approvals rejected, so that stale User actions cannot resume an obsolete operation.
35. As a User, I want retrying after expiry to create a new operation and revalidate current policy, so that changed permissions and risk are respected.
36. As a User, I want the conversation to show concise Connector activity summaries, so that implementation commands do not overwhelm the response.
37. As a User, I want to expand a summary when diagnostics are useful, so that detailed execution remains available on demand.
38. As a User, I want operation summaries to describe business actions such as searching chats or sending a message, so that status is understandable.
39. As a User, I want waiting, started, succeeded, failed, cancelled, timed out, and unknown outcomes distinguished, so that “completed” is not confused with “succeeded.”
40. As a User, I want duplicated terminal text suppressed, so that execution details do not repeat the Assistant response.
41. As a User, I want an interrupted read operation retried only when its policy says it is safe, so that recovery is predictable.
42. As a User, I want an interrupted write with an uncertain external result labeled `outcome_unknown`, so that the platform does not send it twice.
43. As a User, I want active conversations to retain their frozen Connector behavior, so that a catalog update cannot rewrite an in-progress interaction.
44. As a User, I want a new conversation to use the latest published Connector version, so that new work receives current behavior.
45. As a User, I want security-significant Connector changes to require review before protected use, so that an update cannot silently add access or risk.
46. As a User, I want an Administrator security disablement to block an unsafe frozen version, so that historical pinning cannot override an emergency control.
47. As an Agent, I want a compact Capability Index for selected Connectors, so that every model invocation does not include full integration documentation.
48. As an Agent, I want to retrieve the frozen Capability Contract and reviewed Usage Guide on demand, so that I can choose the right operation without reading mutable provider documentation.
49. As an Agent, I want structured capability inputs, so that I do not construct shell commands or provider-specific HTTP calls.
50. As an Agent, I want validation errors expressed against input fields, so that I can correct a request without parsing process output.
51. As an Agent, I want authorization and approval waits reported as structured execution state, so that I do not invent recovery instructions from error strings.
52. As an Agent, I want rejection, expiry, and unknown outcomes returned as stable codes, so that I can explain results accurately.
53. As an Administrator, I want Connector creation to generate a Manifest Draft, so that I do not have to author every field from scratch.
54. As an Administrator, I want to edit names, descriptions, localized operation text, and the reviewed Usage Guide, so that Users and Agents receive clear instructions.
55. As an Administrator, I want security fields sourced from a trusted Profile or package Manifest, so that ordinary form editing cannot weaken policy.
56. As an Administrator, I want a Draft with missing security fields blocked from publication, so that incomplete discovery cannot become executable policy.
57. As an Administrator, I want the exact package digest or remote-service revision bound to the Resolved Manifest, so that validation evidence applies to what runs.
58. As an Administrator, I want Authorization Scheme and Permission references validated before publication, so that unsupported grants fail during review.
59. As an Administrator, I want CLI argument mapping validated without executing a shell, so that structured input cannot become command injection.
60. As an Administrator, I want MCP Tool discovery to create only a Draft, so that remote schema drift cannot become trusted automatically.
61. As an Administrator, I want remote MCP schema drift to require a newly tested version, so that a published contract remains frozen.
62. As an Administrator, I want risk, Egress, credential delivery, and idempotency reviewed together, so that capability safety is complete.
63. As an Administrator, I want provider-specific Authorization implemented behind a platform registry, so that adding a Scheme does not add frontend branches.
64. As a Connector publisher, I want to provide a versioned Manifest that follows the platform schema, so that the platform can review my Connector without reverse engineering README prose.
65. As a Connector publisher, I want the platform to reject executable authorization hooks, so that all grants run through trusted code.
66. As a Connector publisher, I want a constrained declarative CLI mapping, so that my capability can accept structured input without an arbitrary script.
67. As an operator, I want Connector Operation events to use a versioned typed contract, so that backend and frontend releases can evolve safely.
68. As an operator, I want an unknown action type to fail closed with a compatibility message, so that an old client cannot perform an action it does not understand.
69. As an operator, I want provider continuation state encrypted on the server, so that Device Codes, OAuth State, and refresh context are not exposed to the Agent or browser.
70. As an operator, I want only one concurrent refresh per Authorization, so that simultaneous operations cannot race Token rotation.
71. As an operator, I want paused operation state persisted, so that an API or Worker restart does not discard an authorization flow.
72. As an operator, I want Connector policy revalidated after every User action and immediately before execution, so that stale approval cannot bypass a change.
73. As an operator, I want credentials materialized only for the owning Connector operation and cleaned idempotently, so that Secret lifetime is bounded.
74. As an operator, I want every materialized Secret added to exact-value redaction before process output is accepted, so that logs and events cannot leak credentials.
75. As an operator, I want minimal structured Connector audit records, so that authorization and execution decisions are traceable without storing private content.
76. As an operator, I want Tokens, Setup Secrets, authorization URLs, Headers, temporary credential files, and complete argv excluded from ordinary audit records, so that observability does not become a Secret store.
77. As an operator, I want migration to preserve existing record identities and encryption when safe, so that rollout does not disconnect valid Users unnecessarily.
78. As an operator, I want records that cannot be migrated reliably marked as requiring authorization, so that rollout fails closed.
79. As an operator, I want the Feishu implementation to use the same Conformance suite as later Authorization Schemes, so that the first migration proves the abstraction.
80. As an operator, I want provider-specific compatibility code removed after migration evidence passes, so that the generic model remains the only long-term path.

## Implementation Decisions

- Connector is the shared domain abstraction. CLI Connector and MCP Connector retain transport, ownership, catalog, and enablement differences while sharing Manifest, Permission, Authorization, Action Requirement, Operation, Approval, and audit semantics.
- Connector Enablement, Connector Setup, Connector Authorization, and Connector Operation Approval remain separate concepts and persistence lifecycles.
- Each User and Connector pair has at most one active Connector Authorization. Credentials and Setup are never shared across Connectors, even when those Connectors reference the same provider or Authorization Scheme.
- An owner-supplied static MCP transport Secret remains part of that MCP Connector's configuration and keeps the existing private or Platform Resource ownership semantics. It is not a User Connector Authorization. If a platform MCP Connector later requires a per-User external grant, that grant is a separate User-private Authorization bound only to that Connector.
- A Connector Authorization records one external identity, granted Connector Permission identifiers, lifecycle state, credential version, and protected provider credentials. Stable states and stable reason codes are separate fields.
- Supported Authorization lifecycle states include pending, active, invalid, and revoked. Supported reason codes include authorization_required, setup_required, permissions_missing, expired, refresh_failed, provider_denied, provider_unavailable, scheme_unavailable, account_selection_required, cancelled_by_user, and user_action_expired.
- Credential refresh for the same external identity updates the current Authorization version. A different identity requires an explicit account-switch flow and does not silently move existing Conversation Authorization Bindings.
- Connector Setup stores the User-private configuration needed to initiate a grant. The Authorization Scheme supplies a typed Setup schema with labels, sensitivity, validation, and reviewed help links.
- The platform defines an Authorization Adapter interface and trusted registry. The first implementation is compiled and dependency-injected with the platform release; dynamic code loading from Connector packages is prohibited.
- An Authorization Scheme owns its Setup schema, Permission Catalog, credential slot definitions, lifecycle behavior, and Adapter identifier. A Connector package may reference a registered Scheme but may not implement it.
- Connector Permissions are stable logical identifiers mapped by the Authorization Scheme to exact provider scopes. Capabilities reference Permission identifiers rather than arbitrary provider scope strings.
- A Connector Manifest is versioned and machine-readable. It declares capabilities, structured input schemas, localized capability names and operation phrases, Authorization Scheme, Permissions, risk, Egress, timeout and output limits, idempotency, transport mapping, and Connector Credential Delivery.
- Connector Credential Delivery is a trusted declarative mapping from the current Connector's credential slots to allowlisted environment variables, HTTP Headers, or temporary files. It cannot execute code, interpolate shell text, or reference another Connector's credentials.
- Connector creation produces a Manifest Draft. An exact platform Profile is authoritative when available for the exact package or remote-service revision; otherwise a package Manifest is the source. Discovery and README content are candidate inputs only and cannot fill or override trusted security fields silently.
- Publication requires schema validation, all trusted references to resolve, complete security semantics, transport Conformance against the exact immutable artifact or service revision, and Administrator review.
- A published Resolved Connector Manifest is immutable. A change creates a new version. Historical execution and active conversations keep their frozen version; new conversations resolve the latest published version.
- Permission, risk, Setup, Credential Delivery, or other security-significant changes place affected enablement in needs_review before protected execution. A platform security disablement may block future use of an old frozen version.
- MCP creation and testing may discover tools and generate a Manifest Draft. Remote schema drift fails closed and requires a new tested Manifest version. Discovery cannot infer Authorization, risk, Permissions, credential delivery, or idempotency.
- README remains human-facing package documentation. It is neither injected into the Agent nor interpreted as execution or security policy.
- Connector Usage Guide is reviewed, versioned, User-visible guidance associated with the Resolved Manifest. It may teach the Agent how to select and combine capabilities but cannot override the Manifest.
- Selecting a Connector injects only a compact frozen Connector Capability Index. A platform-local, read-only `connector.describe` operation returns the full Capability Contract and reviewed Usage Guide on demand without provider access, Authorization, or operation approval.
- The Agent invokes a capability using Connector identity, Capability identity, execution identity, and structured input. It does not submit arbitrary argv or an arbitrary command suffix.
- A CLI capability declares fixed executable and command segments plus a restricted mapping from validated input fields to flags or positional values. Unknown fields, shell constructs, command substitution, and free-form templates are rejected.
- An MCP capability sends validated structured input to the frozen tool contract. CLI and MCP use the same Connector Operation lifecycle above their transport adapters.
- Connector Operation lifecycle events include requested, waiting, started, and a terminal outcome of succeeded, failed, cancelled, timed_out, or outcome_unknown. Lifecycle completion and successful business outcome are distinct.
- Capability localization supplies names and short operation phrases. The platform generates consistent lifecycle and result sentences and applies current UI locale with a documented fallback while retaining the frozen localization map for historical rendering.
- CLI Wrapper and MCP Gateway emit typed Connector Operation events. Runtime command events do not define Connector business state, and frontend code does not reverse-parse shell commands.
- Raw argv and process diagnostics may appear only in an explicitly expanded, Secret-redacted diagnostic view. The default conversation activity is a concise business summary, and terminal details that duplicate the Assistant response are suppressed.
- The Connector Broker performs generic enablement, Setup, Authorization, Permission, risk, approval, Egress, input, timeout, and frozen-version checks before transport invocation and repeats mutable security checks immediately before execution.
- Selecting or opening a Connector never starts account Authorization. The first protected capability invocation with missing Setup, Authorization, or Permission creates a Connector Action Requirement in that conversation.
- Connector Action Requirement is a versioned discriminated contract containing frozen Connector and Capability identities, Operation identity, external identity summary when known, stable reason code, expiry, and permitted generic actions.
- Supported generic User actions include open_url, show_code, copy_value, check_status, and edit_setup. Frontend code renders these actions without provider identifiers or provider-specific components.
- A Connector Authorization Session exposes a short-lived, one-time platform URL rather than a provider URL generated by the Agent. It binds User, Connector, Conversation, Operation, requested Permissions, expiry, and cryptographically random state before redirecting to the provider.
- Provider callback verification or Adapter polling resumes the same persisted execution automatically. A User recovery signal such as check status requests verification without creating another model invocation.
- Paused execution persistence includes the frozen capability, validated structured input, Conversation Authorization Binding, Connector Action Requirement, and encrypted opaque provider continuation state. Plaintext credentials are never persisted in operation state.
- If Setup and Authorization are both missing, Setup is completed first and the same operation advances to Authorization. If Authorization and Approval are both required, Authorization establishes the actual external identity before Approval is shown.
- Connector Operation Approval covers CLI and MCP. It binds exactly one Operation ID, Resolved Manifest version, Authorization identity, target, User-visible input summary, input digest, expiry, and one-use nonce.
- Rejecting Setup, Authorization, or Approval returns cancelled_by_user to the same execution. It does not disable the Connector or revoke an unrelated existing Authorization.
- An expired User Action Wait ends the current execution with user_action_expired and invalidates its links, codes, OAuth State, and approval. A retry creates a new Operation and revalidates current state.
- Authorization Adapter may perform one concurrency-deduplicated Token refresh before protected execution. Persistent failure updates Authorization state and emits a structured recovery reason rather than a raw provider error.
- Automatic operation recovery follows the frozen capability idempotency policy. Reads may be retryable; writes require a stable provider idempotency key or reviewed reconciliation operation. An uncertain write becomes outcome_unknown and requires User judgment.
- Connector Audit Record contains only stable User, Connector, Manifest, Capability, Authorization, Permission, action, reason, result, timestamp, target-summary, and input-digest data needed for accountability.
- Provider continuation state remains encrypted server-side. Tokens, Setup Secrets, authorization URLs, credential delivery values, complete argv, Headers, temporary credential file content, and complete provider responses are excluded from ordinary audit records.
- Typed Connector contracts are explicitly versioned and evolve through additive compatible fields. Unknown action variants fail closed and produce a generic client-compatibility message.
- The first migration registers Feishu as a normal Authorization Scheme and an exact-version platform Profile for the supported CLI package. Provider-specific logic remains inside the Feishu Adapter rather than Workspace services, Worker branching, or frontend code.
- Migration is staged: introduce generic contracts and storage, adapt existing behavior, migrate records with stable identities and existing encryption where valid, switch reads and writes, gather regression evidence, then delete compatibility paths.
- The previous behavior that enables Feishu authorization while merely selecting the Connector is superseded. The previous allowance for multiple active Feishu account authorizations under one User application is superseded by one active Authorization per User and Connector.
- A migrated record whose ownership, identity, Permission, credential version, or encryption cannot be established is marked as requiring reauthorization. Migration never guesses missing security data.
- The initial delivery migrates the Feishu CLI path and establishes transport-neutral contracts. Additional providers use the same interfaces and Conformance suite; they do not require changes to the generic frontend flow.

## Testing Decisions

- Tests assert externally observable contracts rather than private helper structure, provider branch names, rendered implementation details, or exact internal call order.
- The primary and highest new seam is a shared Connector Broker Conformance suite. It accepts a frozen Manifest, fake Authorization Adapter, fake credential materializer, fake approval store, and recording transport, then verifies the same authorization, approval, lifecycle, recovery, and audit behavior for CLI Wrapper and MCP Gateway integrations.
- The Broker Conformance suite covers no-authorization capabilities, missing Setup, missing Authorization, missing Permissions, permission expansion, account mismatch, refresh success, refresh failure, revocation, provider unavailability, authorization callback, polling recovery, rejection, expiry, and immediate pre-execution revalidation.
- The same suite covers approval ordering, one-use approval consumption, changed-input rejection, changed-target rejection, expired approval, concurrent consumption, security disablement, and unknown action variants.
- The same suite covers restart while waiting, automatic safe recovery, idempotent read retry, idempotency-key write retry, non-idempotent outcome_unknown, cancellation, timeout, event sequence, exactly one terminal outcome, and event-publication failure.
- Existing CLI Wrapper contract tests remain the lower security seam for restricted argv rendering, executable allowlists, unknown-field rejection, Egress, Workspace, output, timeout, immutable bundle, approval digest, and execution-time revalidation.
- Authorization Adapter contract tests use a provider HTTP fake and verify Setup validation, exact requested scopes, state correlation, account identity, Permission mapping, refresh serialization, structured failure reasons, revocation, encrypted continuation handling, and no Secret values in errors.
- Manifest resolution tests cover source precedence, exact Profile matching, package Manifest fallback, draft-only discovery, missing security fields, unregistered Scheme, unknown Permission, invalid Credential Delivery, unsafe CLI mapping, MCP schema drift, immutable publication, and needs_review transitions.
- Workspace application tests cover owner isolation, one active Authorization per User and Connector, Setup and Authorization lifecycle separation, Conversation Authorization Binding, explicit account switching, User Action Wait persistence, rejection, expiry, and same-execution resume.
- Repository integration tests cover uniqueness constraints, optimistic versioning, transactional state and event commits, encryption AAD ownership, one-time nonce or state consumption, concurrent refresh, migration identity preservation, and fail-closed migration status.
- Runtime execution tests cover operation-scoped Secret materialization, exact-value redaction before event persistence, Connector-local Credential Delivery, no cross-Connector credential resolution, cleanup on success and every failure path, and absence of Secrets in command arguments.
- Frontend component tests cover a single generic Action Requirement renderer for open_url, show_code, copy_value, check_status, and edit_setup; compact button sizing; stable placement in the conversation; unknown-action fallback; expiry; and the absence of prompts when a Connector is merely selected.
- Frontend activity tests cover concise localized requested, waiting, started, and terminal summaries; expandable redacted diagnostics; succeeded versus completed wording; outcome_unknown; and duplicate terminal-response suppression.
- API contract tests cover versioned discriminated DTOs, stable reason and outcome codes, additive optional fields, non-enumerating ownership failures, one-time action completion, and unsupported-client behavior.
- Migration tests start from representative existing Feishu Setup, Authorization, attempt, and Approval rows. They prove valid data remains usable, invalid data becomes reauthorization-required, IDs and ownership remain stable, and no legacy ciphertext or Secret appears in migration logs.
- End-to-end browser coverage follows the User-visible path: select a Connector without a prompt, request a protected action, complete Setup if required, open the platform authorization link, complete a fake provider grant, approve the exact operation, resume the same response, and observe a concise terminal summary.
- End-to-end negative coverage includes denied authorization, expired link, account mismatch, missing Permission, revoked Token, provider outage, rejected operation, changed input after approval, Worker restart while waiting, and uncertain write outcome.
- Production Conformance must bind evidence to the exact Connector bundle digest, Runtime image RepoDigest, and platform Profile revision. Parser or unit-test support alone is not evidence that a Capability is available in production.
- Targeted package tests run before affected backend and frontend gates. The completed implementation must run Go formatting, affected Go package tests, the complete backend test and build gates, frontend typecheck and production build, and applicable Linux sandbox or production Conformance where the required environment exists.
- Missing Linux, gVisor, real-provider, or exact-image evidence is reported as unverified rather than passed. Skipped remote-provider tests are not counted as successful integration evidence.

## Out of Scope

- Loading executable Authorization Adapter code from Connector packages at runtime.
- A general third-party plugin host or separately deployed Authorization Adapter service in the initial delivery.
- Sharing Setup, Tokens, grants, or other credentials between CLI and MCP Connectors, even when they use the same provider.
- Supporting multiple simultaneously active external accounts for one User and Connector.
- Permanent, conversation-wide, or capability-wide approval of high-risk operations.
- Arbitrary shell commands, free-form argv suffixes, shell templates, Connector-provided credential scripts, or provider-specific frontend components.
- Treating README prose, CLI help output, model inference, or MCP discovery as authoritative security policy.
- Automatically retrying a write operation whose external outcome cannot be established.
- Automatically moving an existing conversation to a newly authorized external account.
- Rewriting immutable historical Message, Run, Snapshot, Artifact, or audit content to the new presentation model.
- Adding every provider or Connector during the Feishu migration. Later providers must implement the accepted generic contracts.
- Claiming production support for a Connector, Runtime image, or provider flow before exact-version Conformance evidence exists.
- Changing MCP Connector catalog ownership rules, platform-resource ownership, or the existing rule that platform MCP Secrets remain owned by their Administrator creator.

## Further Notes

- This spec uses the domain vocabulary in `CONTEXT.md`. Connector Authorization replaces the narrower CLI Connector Authorization term, and Connector Operation Approval replaces CLI-only Connector Command Approval for future behavior.
- This spec supersedes the product behavior that Feishu enablement automatically begins external setup or account authorization. Enablement remains explicit, while Setup and Authorization are requested lazily by the first protected capability.
- This spec supersedes the product behavior that allows multiple Feishu account authorizations under one User application. The accepted invariant is at most one active Connector Authorization for each User and Connector.
- Existing static MCP transport Secrets are distinct from external account Authorization. They retain their current Connector-owner semantics and are not made User-readable or copied into per-User Authorization records.
- Existing ADRs requiring frozen Connector revisions, platform Wrapper enforcement, conversation-level Connector selection, User-private authorization, one-use confirmation, and immutable history remain in force.
- The decision to generalize Authorization Scheme, Manifest, Action Requirement, and Operation semantics across CLI and MCP is architecturally significant and should be recorded in a dedicated ADR before the compatibility layer is removed.
- The local specification is the requested publication target for this synthesis. No issue-tracker item or triage label is created.
