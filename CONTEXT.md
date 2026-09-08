# Agent Workspace

A personal AI workspace for private conversations, reusable Workflows, Experts, Expert Teams, Skills, Connectors, and managed execution.

## People And Ownership

**Agent Workspace**:
The product in which an authenticated User creates private Sessions, configures Workflows and Experts, and runs Workflows through managed Runtime Engines.
_Avoid_: Coding Agent Platform, multi-agent system

**User**:
An authenticated person who exclusively owns their Sessions, Workflows, Expert Teams, Personal Settings, Credit Balance, Credit Ledger, and privately created Experts, Skills, and MCP Connectors. Every User may also select Administrator-created Platform Resources, the platform-wide Model Catalog, and available CLI Connectors.
_Avoid_: Organization member, Team member, product role

**Administrator**:
The single bootstrap identity that manages User accounts, Platform Resources, the platform-wide Model Catalog, Daily Credit Allocations, Model Credit Rates, Redemption Codes, and reasoned Credit Adjustments without access to private User-owned content or execution-level consumption.
_Avoid_: Platform operator, Organization administrator, support user

## Credits And Usage

**Credit**:
A product usage unit available to one User and consumed by Provider Model usage; at a Model Credit Rate of 1.00, one Credit represents 10,000 input or output Tokens. A User without available Credits cannot start another model execution.
_Avoid_: Currency, Provider charge, Token

**Credit Balance**:
The sum of one User's remaining Daily Credit Allocation and Redeemed Credit Balance. A completed execution may make it temporarily negative, preventing another execution until Credits become available again.
_Avoid_: Daily Credit Limit, Provider balance, Token balance

**Daily Credit Allocation**:
A User-specific amount of expiring Credits restored at the start of each calendar day in that User's configured time zone. Unused daily Credits do not carry forward and are consumed before redeemed Credits.
_Avoid_: Daily Credit Limit, recurring Credit Grant, rolling allowance

**Redeemed Credit Balance**:
The persistent Credits a User has received by redeeming Redemption Codes. They remain available across daily boundaries and are consumed after Daily Credits.
_Avoid_: Daily Credit Allocation, payment balance

**Redemption Code**:
A platform-issued, globally single-use code with a fixed Credit value and optional expiry. A successful redemption adds persistent Credits to the redeeming User.
_Avoid_: Workflow Access Token, coupon, payment

**Model Credit Rate**:
The platform-managed input multiplier, output multiplier, and missing-Usage fallback that determine Credit Consumption for a Provider type, Model API Protocol, and exact Provider Model identifier.
_Avoid_: Provider price, User model setting, single model multiplier

**Credit Adjustment**:
An Administrator's reasoned addition to or subtraction from one User's persistent Credit Balance, preserved as an immutable account record.
_Avoid_: Balance overwrite, Redemption Code, Daily Credit Allocation

**Credit Ledger**:
The immutable chronological record of one User's daily allocations, redemptions, adjustments, and Credit Consumption. The current Credit Balance and daily usage are projections of this record.
_Avoid_: Mutable balance, Runtime Event stream, Provider invoice

**Credit Day**:
The calendar day used for one User's Daily Credit Allocation and daily consumption, bounded by midnight in that User's effective Personal Settings time zone.
_Avoid_: Rolling 24-hour window, UTC day, billing cycle

**Credit Consumption**:
The immutable two-decimal Credit amount charged to one completed, failed, or cancelled model execution from its measured input and output Tokens and frozen Model Credit Rate.
_Avoid_: Token Usage, Provider cost, Session total

## Conversations

**Session**:
A private, continuing text conversation owned by one User. Each response may use one Expert, one Expert Team, or no specialist; the selected specialist persists until changed. It freezes its Personal Settings execution configuration when the first message starts.
_Avoid_: Workflow Run, Coding Task, runtime process

**Response Snapshot**:
The immutable ordered Execution Stage Snapshots for one Session response or Run, preserved for its retry. It records no API Key and preserves the specialist, resource revisions, and execution identity of every model invocation shown with the resulting Agent response.
_Avoid_: Workflow Snapshot, current Session model, Native Session

**Conversation Draft**:
One User's unsent message and selections for a particular Session or Run Conversation. Its explicit Skills and file selections belong to the next message, while its specialist and Connector choices persist for subsequent messages until changed.
_Avoid_: Workflow definition, accepted message, Response Snapshot

**Conversation Selection**:
An immutable, owner-and-conversation-scoped revision of the selected specialist, member-specific default resources, explicit resource additions, and Connector exclusions. Drafts reference its opaque identity; accepted messages freeze its merged execution stages and retain a new revision without explicit Skills for the next turn.
_Avoid_: Expert definition, Personal Settings, Runtime configuration

**Archived Session**:
A read-only Session hidden from the active list until the User cancels its archive state.
_Avoid_: Deleted Session, completed Run

**Native Session**:
An opaque conversation identity maintained by one Runtime Engine to continue a product Session, including across Provider Model changes that keep the same Runtime Engine. It is an optional, conformance-gated optimization and never replaces platform-owned history.
_Avoid_: Session, Run, rolling summary

**Rolling Summary**:
The platform-maintained bounded summary that preserves Session continuity when history is too large, the Runtime Engine changes, or native Resume is unavailable.
_Avoid_: Agent Memory, user-authored note, Runtime checkpoint

## Workflows And Execution

**Workflow**:
A reusable executable configuration that combines a name, goal, optional Expert or Expert Team, environment, API access, schedule, and one persistent Workspace. It is a single execution definition, not a visual graph or arbitrary DAG.
_Avoid_: Pipeline, visual DAG

**Workflow Snapshot**:
The immutable copy of a Workflow's goal, initial execution stages, environment, and other initiating inputs used by one Run Conversation. Follow-up Response Snapshots preserve that context while recording each turn's specialist and resource selection. API Keys are referenced through protected versioned credentials rather than copied into the ordinary snapshot.
_Avoid_: Published Workflow, Workflow release

**Execution Stage Snapshot**:
The immutable execution identity for one model invocation within a Response Snapshot or Workflow Snapshot, including its optional Expert and Team Member identities, Provider Model, Model Provider Connection version, API Protocol, Runtime Engine, structured Expert guidance, Skills, and Connectors. An execution without an Expert has one anonymous stage; an Expert Team has one ordered stage per member.
_Avoid_: Expert Stage result, mutable Expert, team Runtime Engine

**Workflow API Credential**:
The single API Key and write-only API Secret pair used only to exchange for a short-lived Workflow Access Token. Regeneration immediately invalidates the previous pair and every token derived from it.
_Avoid_: User Token, Idempotency Key, model credential

**Scheduled Trigger**:
An optional hourly, daily, or weekly schedule that starts a Workflow from its fixed goal in a selected time zone.
_Avoid_: API call, Webhook, file watcher

**Run Conversation**:
A continuing conversation started by one manual, scheduled, or API Workflow trigger. It keeps the initiating Workflow Snapshot and contains one or more ordered Runs, so the User can follow up without losing the original goal, prior messages, or Workspace context.
_Avoid_: Session, one Runtime process, Run Event stream

**Run**:
One immutable execution turn inside a Run Conversation, with fixed input, one terminal result, the initiating Workflow context, and that turn's frozen specialist and resource selection. The first Run records the trigger; each follow-up creates another Run instead of reopening a terminal Run.
_Avoid_: Workflow, Run Conversation, Session response, Worker process

**User Action Wait**:
A non-terminal execution state in which a Session response or Run is paused until the User completes Connector authorization or approves a high-risk Connector operation before a fixed deadline. Verification resumes the same execution automatically; a User recovery signal may request another verification when provider callbacks or polling are unavailable. Rejection resumes that execution with a structured cancelled_by_user result, while expiry ends it with user_action_expired and invalidates every stale action link.
_Avoid_: queued execution, indefinite pause, automatic approval

**Deleted Workflow Record**:
The read-only name, Run history, and unexpired Artifacts retained after a Workflow and its Workspace are permanently deleted.
_Avoid_: Restorable Workflow, archived Workflow

## Files And Results

**Workspace**:
The persistent directory and file tree owned by one Workflow and reused across serialized Runs. A Run changes a temporary copy and merges it only on success.
_Avoid_: Session, Artifact, per-Run sandbox

**Git Source**:
The optional single Git repository cloned into an empty Workspace root from Workflow Git Settings. It uses public HTTPS, HTTPS account/password (or token), or SSH private-key authentication and a fail-closed allowlist of local Git configuration.
_Avoid_: Repository Binding, Source Control Provider, Review Branch

**Artifact**:
An immutable file generated by one successful Session response or added or changed by one successful Run, shown directly with that Agent response and separately from a Workflow's mutable Workspace. Final text or JSON remains part of its Session or Run Conversation and is not an Artifact.
_Avoid_: Run result, Workspace file, Run Event, temporary output

**File Reference**:
A message's explicit reference to an attachment or Artifact from its own conversation, or to a file in its Workflow's Workspace. An accepted reference preserves the file content supplied with that message independently of later source changes or expiry.
_Avoid_: Mutable Workspace path, signed download URL, filename mention

## Experts, Skills, And Connectors

**Platform Resource**:
An Expert, Skill, or MCP Connector created by the Administrator and visible to every authenticated User. It appears only in the platform section of its catalog, remains editable only by the Administrator, and may be selected by Users alongside their private resources.
_Avoid_: shared User resource, public credential, CLI Connector Definition

**Expert**:
A reusable specialist profile with an Icon, display name, display-only Introduction, visible structured guidance, and selected Skills and Connectors. Its Core Capability, Operating Procedure, Output Standard, and Cautions form its injected guidance; it does not select a Provider Model or Runtime Engine.
_Avoid_: Persona, Workflow

**Profile Icon**:
A preset symbol and background color that visually identify an Expert or Expert Team. It always has a default and is not a User-uploaded image.
_Avoid_: Avatar file, Artifact, attachment

**Introduction**:
The display-only summary of an Expert or Expert Team. It is never injected into model instructions.
_Avoid_: Core Capability, hidden prompt

**Core Capability**:
The visible explanation of the work an Expert is qualified to perform, or the combined abilities of an Expert Team. An Expert's Core Capability contributes to its injected guidance; an Expert Team's Core Capability is display-only.
_Avoid_: Introduction, Expertise Tag

**Operating Procedure**:
The visible, User-authored steps an Expert follows when performing work. The product interface labels it `工作流程`, while the domain name distinguishes it from the executable Workflow aggregate.
_Avoid_: Workflow, hidden prompt

**Output Standard**:
The visible, User-authored requirements for the form and quality of an Expert's result.
_Avoid_: Artifact format, hidden prompt

**Cautions**:
Optional visible, User-authored constraints and pitfalls an Expert must consider while working.
_Avoid_: platform policy, hidden prompt

**Derived Expertise Tag**:
A rebuildable, system-derived label projected from an Expert's Core Capability for discovery and display. It is not User-authored guidance.
_Avoid_: User-authored tag, Core Capability, managed taxonomy

**Expert Team**:
A reusable named profile with an Icon, Introduction, display-only Core Capability, and an ordered list of Team Members. The same Expert may fill more than one distinctly named Team Member role, and each execution uses an immutable snapshot of its selected team.
_Avoid_: User team, organization, visual workflow

**Team Member**:
A stably identified, named role in one Expert Team that references an Expert and has role-specific labels. Its member name, labels, and order may change without replacing its identity; the same Expert may be referenced by multiple Team Members with isolated execution contexts.
_Avoid_: Expert, User, organization member

**Member Label**:
A User-authored label describing one Team Member's responsibility within an Expert Team. It is distinct from the referenced Expert's Derived Expertise Tags.
_Avoid_: Derived Expertise Tag, Expert capability

**Subagent**:
A platform-managed execution of one Team Member in its own isolated execution context inside an Expert Team. It uses the execution configuration frozen for the Session or Run Conversation; Runtime-specific native subagent support is not required for this behavior.
_Avoid_: Expert selected alone, simulated persona, Runtime capability

**Expert Team Execution**:
A fail-fast collaboration in which every Subagent receives the current task, bounded conversation context, attachments, and all preceding Subagent results, then executes in Team Member order using the shared frozen execution configuration. The final member produces the official response and retry restarts the whole collaboration.
_Avoid_: Parallel fan-out, arbitrary agent graph, coordinator synthesis

**Expert Snapshot**:
The immutable Expert or Expert Team definition used by a Session response or Run, including visible profile content, structured Expert guidance, Team Member roles, member order, and exact Skill and Connector revisions. A later specialist selection or source-profile edit does not change this snapshot.
_Avoid_: Current Expert, mutable team, execution configuration snapshot

**Incomplete Expert**:
A migrated or partially edited Expert missing required Introduction, Core Capability, Operating Procedure, or Output Standard content. It remains visible and editable but cannot be selected for a new Session or Run Conversation until completed.
_Avoid_: unavailable execution configuration, deleted Expert

**Connector**:
A selectable integration through which a Session response or Run accesses an external capability, with or without an Expert. A User may privately own an MCP Connector, while an Administrator creates platform-wide MCP and Third-party CLI Connectors; User-specific CLI authorization remains private to that User.
_Avoid_: Extension, Skill, Runtime Engine

**Connector Manifest**:
A versioned, machine-readable declaration of one Connector's capabilities, structured inputs, User-visible operation semantics, Authorization Scheme, required permissions, risk, network destinations, and execution limits. It may describe behavior but cannot supply executable authorization logic.
_Avoid_: README, Connector Usage Guide, arbitrary command script

**Resolved Connector Manifest**:
The reviewed, validated Connector Manifest bound to one exact Connector Definition and its immutable package or remote-service revision. Publication requires schema and trusted-reference validation, transport conformance evidence, and Administrator review. An exact platform Profile is authoritative when present; otherwise the package Manifest is used, while automatically discovered command metadata remains a draft input. Existing conversations retain their frozen version, while new conversations use the latest published version after any required User review of security-semantic changes.
_Avoid_: generated Manifest draft, current upstream README, mutable catalog description

**Connector Usage Guide**:
Versioned, User-visible instructions that help an Agent choose and combine a selected Connector's structured capabilities. A selected Connector contributes a compact capability index, while the Agent retrieves the frozen full guide through a platform-local read-only description operation.
_Avoid_: README, hidden prompt, Skill, security policy

**Connector Operation**:
One invocation of a structured capability through a selected Connector, bound to its frozen Definition, execution identity, Authorization Binding, reviewed input, and optional target. Its lifecycle and outcome are reported through a versioned, typed contract by the CLI Wrapper or MCP Gateway rather than inferred from Runtime command text. Recovery follows the frozen capability's idempotency policy; an interrupted operation whose result cannot be established becomes outcome_unknown instead of being retried blindly.
_Avoid_: arbitrary shell command, Agent message, Connector Operation Approval

**Connector Capability Index**:
The compact, frozen list of structured capabilities supplied to an Agent when a Connector is selected. It identifies available operations and the local description action without embedding the full Connector Usage Guide in every model invocation.
_Avoid_: Connector Usage Guide, README, dynamically discovered tool list

**CLI Connector Definition**:
An Administrator-owned, platform-wide definition of one Third-party CLI's icon, name, capability description, immutable installation source, derived executable contract, capabilities, authentication, and execution policy. Users may use but never create or modify it.
Administrator deletion retains a disabled historical record, removes catalog visibility and mutable Expert bindings, and revokes User access without changing frozen snapshots.
_Avoid_: CLI authorization, MCP Connector, arbitrary package command

**Connector Authorization**:
A User-private grant from an external identity to exactly one enabled Connector, covering an explicit set of permissions and protected, versioned credentials. At most one Authorization is active for each User and Connector. Credentials are never shared with another Connector, including a Connector that uses the same provider or Authorization Scheme. Permission expansion requires explicit User consent; an authorization response for a different external identity enters the account-switch flow instead of replacing the identity silently. Its stable lifecycle state is separate from a structured reason explaining required action, provider failure, expiry, or revocation.
_Avoid_: CLI Connector Definition, Connector Enablement, Connector Operation Approval, shared platform credential

**Connector Setup**:
User-private configuration required by one Connector before its Authorization Scheme can request an external account grant, such as that User's App ID and App Secret for the Connector. The Authorization Scheme provides a structured Setup schema so a missing Setup can be completed from a generic setup_required Action Requirement in the conversation. Setup and Authorization have separate lifecycles and protected records, but neither may be reused by another Connector.
_Avoid_: Connector Authorization, platform-wide provider credential, shared OAuth application

**Connector Credential Delivery**:
A trusted, declarative mapping from one Connector's own Authorization credential slots to the exact environment variables, HTTP headers, or temporary files accepted by that Connector's frozen execution transport. It cannot reference another Connector's Authorization, execute code, or expose plaintext credentials to the Agent.
_Avoid_: shared provider credential, arbitrary environment template, Authorization Adapter

**Connector Authorization Binding**:
A Conversation Selection's stable reference to the single active Connector Authorization under that same Connector when its first protected capability is invoked. Credential refresh and permission expansion for the same external identity preserve the binding, while account replacement does not silently move existing conversations. No Authorization from another Connector can satisfy the binding.
_Avoid_: encrypted Token, account discovery, per-command account guess

**Connector Enablement**:
A User's activation of one available Connector before selecting or using it. It is distinct from Connector Setup and external account Authorization. Selecting or opening an enabled Connector does not request authorization; the first actual capability that requires missing Permissions creates a Connector Action Requirement in that conversation.
_Avoid_: Connector Authorization, Expert selection, Run

**Connector Authorization Scheme**:
A platform-trusted authorization protocol referenced by a Connector, defining its Setup schema, Permission Catalog, credential slots, and how external grants, refresh, revocation, and execution credentials behave. Its registered Adapter emits only supported generic User actions such as opening a platform URL, showing or copying a code, checking status, or editing Setup. A Connector package may select a supported scheme but cannot supply executable authorization logic.
_Avoid_: Connector Authorization, arbitrary authentication script, credential template

**Connector Permission**:
A stable authorization capability from one Connector Authorization Scheme that maps a User-visible meaning to exact provider scopes. Connector capabilities reference Permission identities rather than supplying arbitrary provider scopes.
_Avoid_: Provider scope string, Connector capability, Connector Operation Approval

**Connector Action Requirement**:
A versioned, typed, time-bounded description of the Connector-related User action needed to resume an execution, including the frozen Connector and capability identities, external identity, stable reason code, and permitted generic actions. Sessions, Run Conversations, and the global action inbox present the same requirement without parsing command, provider error text, or provider-specific identifiers in the frontend. An unsupported action type fails closed with a generic client-compatibility message.
_Avoid_: raw error message, authorization popup, Connector-specific frontend event

**Connector Authorization Session**:
A short-lived, one-time platform flow that binds one User, Connector, Conversation, Connector Operation, requested Permissions, and random state to an external authorization redirect. Its Action Requirement exposes a platform URL as both a button and copyable link; a verified callback resumes the same execution without exposing provider parameters to the Agent.
_Avoid_: Provider authorization URL generated by an Agent, permanent authorization link, Connector Authorization

**Connector Operation Approval**:
A time-bounded User decision required before one high-risk Connector Operation executes through CLI or MCP. Approval is specific to exactly one displayed Connector Operation, Authorization identity, structured input summary, target, and input digest; it is never reused by a later operation. Expiry or rejection prevents that operation from running. When Authorization is also required, the platform establishes the actual external identity before requesting approval.
_Avoid_: Connector authorization, permanent permission, implicit consent

**Connector Audit Record**:
A minimal, Secret-redacted record of a Connector authorization action, approval, or operation containing stable identities, frozen versions, Permission and reason codes, result, timestamps, target summary, and input digest. Provider continuation data remains encrypted server-side, while Tokens, Setup Secrets, authorization URLs, credential delivery values, and complete command arguments never enter ordinary audit records.
_Avoid_: Runtime transcript, plaintext credential, full provider response

**MCP Connector**:
A private User-owned or Administrator-created Platform Resource reached through Streamable HTTP or started as a fixed-version `npx` or `uvx` stdio process inside an isolated Runtime environment. It shares the Connector Manifest, Permission, Authorization, and Action Requirement model while retaining its existing ownership and enablement rules.
_Avoid_: API Endpoint, Skill, Third-party CLI

**Third-party CLI**:
An Administrator-created Connector installed from an exact npm package or a validated ZIP package and exposed as a direct command inside an isolated Runtime environment without using the MCP protocol. Availability is restricted to Runtime image Digests with the required conformance evidence.
_Avoid_: MCP Connector, arbitrary host command, Runtime Engine

**Feishu CLI Application**:
The single Feishu developer application created for one User when that User enables the Feishu CLI Connector. Its App ID and App Secret are shared by that User's Feishu CLI authorizations, while account tokens remain isolated per authorization.
_Avoid_: CLI Connector Definition, one application per Expert, platform-wide Feishu application

**Skill**:
A private User-owned or Administrator-created Platform Resource containing a required `SKILL.md` and optional scripts or resources, installed from a Git URL or uploaded archive. Scripts run only inside an isolated Runtime environment.
_Avoid_: Connector, Prompt, Runtime Engine

**Recommended Skill Offer**:
An Administrator-authored name and HTTPS Git source attached to a CLI Connector Definition. A User must explicitly install it, after which it becomes an ordinary User-owned Skill; enabling a Connector never installs or injects it implicitly.
_Avoid_: platform-owned Skill, hidden instruction, automatic installation

## Personal Configuration

**Personal Settings**:
A User's personality, default Runtime Engine, Runtime Engine Settings, language, and time zone. Its default Provider Model and Runtime Engine supply every new Session or Run Conversation's execution configuration, whether or not an Expert or Expert Team is selected.
_Avoid_: Organization policy, Expert configuration, Workflow settings

**Personality**:
One of the gentle-professional, direct-efficient, lively-friendly, or custom communication styles, optionally refined by a User-authored explanation.
_Avoid_: Expert, model system prompt, display name

**Model Provider Connection**:
A platform-wide named connection containing a provider type, Endpoint, and write-only API Key. Only the Administrator manages connections; every User may select their available Provider Models.
_Avoid_: Model Profile, Provider Model, Runtime Engine

**Model API Protocol**:
The wire contract exposed by a Model Provider Connection, such as OpenAI Responses, OpenAI Chat Completions, or Anthropic Messages. It is distinct from the provider brand, and one connection may expose more than one protocol.
_Avoid_: Model Provider, Endpoint, Runtime Adapter

**Provider Model**:
A platform-wide model identifier under one Model Provider Connection, loaded from its `/models` API, the platform-maintained provider defaults, or explicit Administrator configuration. The platform does not classify model types; every available Provider Model is selectable subject to Runtime Model Compatibility.
_Avoid_: Model Profile, provider connection, Runtime Engine

**Runtime Model Compatibility**:
The verified, unverified, or incompatible relationship between one Provider Model's API Protocol and one Runtime Engine. An unverified relationship warns but does not prevent selection; an incompatible invocation fails explicitly.
_Avoid_: Provider Model availability, Runtime Capability, provider verification

**Runtime Engine**:
The selected Claude Code, Codex, Hermes, OpenClaw, or PI Agent engine that generates a Session response or executes a Run.
_Avoid_: Provider Model, Expert, sandbox, Worker

**Runtime Engine Setting**:
A User's preference for one Runtime Engine, including that engine's default Provider Model. It supplies the execution configuration frozen when a Session or Run Conversation starts; Experts and Workflows do not override it.
_Avoid_: global default model, Personality model, fixed Session model

**Runtime Adapter**:
The platform boundary that presents one Runtime Engine through the common execution and event contract.
_Avoid_: Runtime Engine, model provider, CLI wrapper

**Runtime Capability**:
A behavior that a specific Runtime image can reliably provide only after its required conformance evidence passes.
_Avoid_: Parsed field, configuration flag, compatibility assumption
