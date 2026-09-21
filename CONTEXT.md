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
A product usage unit available to one User and consumed by Provider Model usage; at a Model Credit Rate of 1.00, one Credit represents 10,000 input or output Tokens, while Image Credit Rates price successfully Generated Images directly. A User without available Credits cannot start another model execution.
_Avoid_: Currency, Provider charge, Token

**Credit Balance**:
The sum of one User's remaining Daily Credit Allocation and Redeemed Credit Balance. A completed execution may make it temporarily negative, preventing another execution until Credits become available again.
_Avoid_: Daily Credit Limit, Provider balance, Token balance

**Available Credit**:
The portion of one User's Credit Balance not withheld by active Image Credit Reservations and therefore available to admit another execution. It is the balance shown in the product's compact account surfaces.
_Avoid_: Credit Balance, reserved Credit, Provider balance

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
The immutable two-decimal Credit amount charged to one model execution, calculated from measured text Tokens and a frozen Model Credit Rate or from successfully Generated Images and frozen Image Credit Rates.
_Avoid_: Token Usage, Provider cost, Session total

**Execution Credit Reservation**:
The temporary amount withheld from one User's Available Credit when a text Execution Stage starts, using that stage's frozen Model Credit Rate fallback. It limits concurrent Workflow admission and is released or settled with the stage; measured usage may exceed it and produce the existing negative Credit Balance.
_Avoid_: Image Credit Reservation, Credit Consumption, Provider prepayment

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
A reusable executable configuration that combines a name, goal, optional Expert or Expert Team, environment, API access, schedule, an optional Knowledge Selection, and one persistent Workspace. It is a single execution definition, not a visual graph or arbitrary DAG.
_Avoid_: Pipeline, visual DAG

**Workflow Queue**:
The persistent FIFO of queued Runs for one Workflow. It serializes access to that Workflow's Workspace while allowing Runs belonging to other Workflows to execute concurrently; its first-version limit is five queued Runs.
_Avoid_: User-wide execution queue, Run Conversation, Workspace lock

**Workflow Snapshot**:
The immutable copy of a Workflow's goal, initial execution stages, environment, and other initiating inputs used by one Run Conversation. Follow-up Response Snapshots preserve that context while recording each turn's specialist and resource selection. API Keys are referenced through protected versioned credentials rather than copied into the ordinary snapshot.
_Avoid_: Published Workflow, Workflow release

**Execution Stage Snapshot**:
The immutable execution identity for one model invocation within a Response Snapshot or Workflow Snapshot, including its optional Expert and Team Member identities, Provider Model, Model Provider Connection version, API Protocol, Runtime Engine, structured Expert guidance, Skills, and Connectors. An execution without an Expert has one anonymous stage; an Expert Team has one ordered stage per member.
_Avoid_: Expert Stage result, mutable Expert, team Runtime Engine

**Workflow API Credential**:
The single API Key and encrypted, owner-viewable API Secret pair used only to exchange for a 72-hour Workflow Access Token. Regeneration immediately invalidates the previous pair and every token derived from it.
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
A non-terminal execution state in which a Session response or Run is paused until the User completes Connector authorization or approves a high-risk Connector command before a fixed deadline. No protected action executes without the required confirmation.
_Avoid_: queued execution, indefinite pause, automatic approval

**Deleted Workflow Record**:
The read-only name, Run history, and unexpired Artifacts retained after a Workflow and its Workspace are permanently deleted.
_Avoid_: Restorable Workflow, archived Workflow

## AI Applications

**AI Applications**:
The product area in which a User creates and uses durable AI capabilities and task-specific creation tools outside a continuing Session or Workflow. Its first entries are Smart Assistants, Digital Humans, and Image Creation.
_Avoid_: AI Creation as a top-level area, Session, Workflow, generic AI tools

**Smart Assistant**:
A reusable, User-owned AI application for one scenario, combining visible service rules, Frequently Asked Questions, an Answer Safety Policy, and optional Knowledge Selection, Expert or Expert Team, and Digital Human references. One User may create many Smart Assistants; customer service is one scenario type rather than the entity name.
_Avoid_: AI Customer Service, Expert, Session, Workflow

**Smart Assistant FAQ**:
An ordered, User-authored question and Markdown answer attached to one Smart Assistant. A direct selection returns its safety-checked answer without a model invocation; uncertain free-text questions may be classified against enabled FAQs before Knowledge Base retrieval.
_Avoid_: Knowledge Document, canned Session message, generated answer

**Digital Human**:
A reusable, User-owned presentation identity describing an AI interaction's avatar, voice, language, and supported expression or scene settings. A Digital Human does not own business knowledge or answer logic and may be referenced by multiple Smart Assistants.
_Avoid_: Smart Assistant, Avatar-only upload, Expert

**Answer Safety Policy**:
The ordered, platform-enforced and Assistant-configurable boundary that decides whether a request may proceed to FAQ matching, Knowledge Base retrieval, or model answering. Platform safety categories cannot be disabled, and a rejected request receives the fixed localized refusal without exposing the matched category or sensitive source text.
_Avoid_: Prompt instruction, content filter toggle, moderation note

**Share Configuration**:
The per-Smart-Assistant public-use configuration containing an unpredictable share Token, optional allowed Origins, iframe dimensions, rate limits, and revocation state. It grants access only to the rendered Assistant surface and never exposes private credentials or internal Session identity.
_Avoid_: Public Session, API Key, User Access Token

**External Conversation**:
An anonymous conversation created through a Smart Assistant Share Configuration and kept separate from the owner's private Session list. It freezes the Assistant, Digital Human, Share Token revision, FAQ or retrieval source, and owner Credit settlement needed for audit.
_Avoid_: Session, visitor account, public chat transcript

**Image Creation**:
The task-specific image-generation tool nested under AI Applications. It owns no Smart Assistant or Digital Human and uses Image Generation Records for its submitted requests and results.
_Avoid_: AI Creation, Smart Assistant, Image Artifact

**Retrieval Provider**:
The replaceable indexing and recall boundary used by a Knowledge Base, initially backed by PostgreSQL full-text and `pgvector` search. It does not own User permissions, Document Revisions, Assistant bindings, or retained Knowledge Citations.
_Avoid_: Knowledge Base, RAG application, Provider Model

**Image Model**:
An Administrator-managed image-generation configuration containing one exact model identifier, independent API Endpoint, and write-only API Key. A User selects an available Image Model and supplies the size, quality, format, background, and output count for an Image Generation Record.
_Avoid_: Model Provider Connection, Runtime Engine default, Provider Model type, User-owned model

**Image Generation Record**:
An immutable, User-owned record of one submitted text-to-image or image-to-image request and its generated results. Regeneration creates another Image Generation Record for the full original output count instead of replacing any original result.
_Avoid_: Session response, Run, mutable generation job

**Reference Image**:
An image supplied as an ordered input to an image-to-image request, either by new upload or by selecting an unexpired Generated Image owned by the same User.
_Avoid_: prompt-only input, mutable external URL, cross-User image

**Prompt Optimization**:
An explicit Image Creation action that expands the current editable image prompt using the Administrator-configured model identifier, independent API Endpoint, write-only API Key, and optimization instruction. It is charged from measured input and output Tokens, replaces the draft prompt after success, and does not submit an Image Generation Record.
_Avoid_: Model Provider Connection, automatic image generation, hidden prompt rewrite, Runtime Engine default

**Generated Image**:
An immutable image produced as one result of an Image Generation Record and retained with that record for 90 days.
_Avoid_: Artifact, attachment, temporary preview

**Image Credit Rate**:
The platform-fixed 50 Credit amount charged for one successfully Generated Image, independent of Image Model, size, quality, format, or background.
_Avoid_: Model Credit Rate, Provider price, batch price

**Image Credit Reservation**:
The temporary, source-preserving Credit amount atomically withheld from one User's Available Credit when an Image Generation Record is submitted, based on its frozen Image Credit Rate and requested output count. It belongs to the submission Credit Day and releases only through terminal settlement without carrying expired Daily Credits forward.
_Avoid_: Credit Consumption, provider prepayment, daily allocation

## Files And Results

**Workspace**:
The persistent directory and file tree owned by one Workflow and reused across Runs serialized by its Workflow Queue. A Run changes a temporary copy and merges it only on success.
_Avoid_: Session, Artifact, per-Run sandbox

**Git Source**:
The optional single Git repository cloned into an empty Workspace root from Workflow Git Settings. It uses public HTTPS, HTTPS account/password (or token), or SSH private-key authentication and a fail-closed allowlist of local Git configuration.
_Avoid_: Repository Binding, Source Control Provider, Review Branch

**Artifact**:
An immutable final deliverable generated by one successful Session response or added or changed by one successful Run, stored separately from a Workflow's mutable Workspace. A final Artifact path is removed from the persistent Workflow Workspace after capture; intermediate files remain available for later Runs. Final text or JSON remains part of its Session or Run Conversation and is not an Artifact.
_Avoid_: Run result, Workspace file, Run Event, temporary output

**File Reference**:
A message's explicit reference to an attachment or Artifact from its own conversation, or to a file in its Workflow's Workspace. An accepted reference preserves the file content supplied with that message independently of later source changes or expiry.
_Avoid_: Mutable Workspace path, signed download URL, filename mention

**Knowledge Base**:
A logical collection of knowledge documents owned by one User or created by the Administrator as a Platform Resource. A User-owned Knowledge Base is private; an Administrator-created Knowledge Base is explicitly private to the Administrator or public, and a public one is readable, searchable, and downloadable by every authenticated User without mutation rights.
_Avoid_: Workspace, Artifact collection, shared file folder

**Knowledge Category**:
An optional first-level grouping inside one Knowledge Base. A Knowledge Document belongs to at most one Category, and may instead remain unclassified; Categories are not nested.
_Avoid_: Directory, tag, multi-parent folder

**Knowledge Document**:
A source supplied to a Knowledge Base as uploaded bytes or a captured web source. Within one Category, a normalized source identifies one logical document whose later uploads or refreshes create new revisions; the same source in another Category is a separate Knowledge Document rather than a multi-category assignment.
_Avoid_: Attachment, Workspace file, Artifact

**Document Revision**:
An immutable accepted source snapshot of a Knowledge Document. A later replacement creates a new revision; only the latest successfully ingested revision participates in retrieval while older revisions remain part of the document's history.
_Avoid_: Mutable file, overwrite, Runtime Snapshot

**Ingestion**:
The asynchronous processing of an accepted Knowledge Document Revision into searchable knowledge. Upload acceptance is independent of Ingestion completion; a revision moves through accepted, processing, ready, failed, or blocked, and only a ready revision is eligible for retrieval while failures retain the source for retry.
_Avoid_: Upload, synchronous import, Runtime execution

**Ingestion Job**:
A durable, idempotent task that performs one Knowledge Document Revision's extraction, OCR, chunking, and indexing outside the Runtime execution queue. It has a lease, retry history, and terminal failure state independent of the source bytes.
_Avoid_: Run, background goroutine, upload request

**Knowledge Selection**:
The set of one or more Knowledge Bases configured on a Workflow for retrieval. A Run freezes the selection in its Workflow Snapshot so later Workflow edits do not change that Run's intended knowledge scope.
_Avoid_: Runtime Engine setting, Conversation Selection, dynamic folder lookup

**Retrieval Context**:
A bounded set of source excerpts returned for one Run from its frozen Knowledge Selection, including provenance such as Knowledge Base, Category, document, source location, and a relevance score. Its redacted citation summary is retained in Run history, but it is not an Artifact or a replacement for the original document.
_Avoid_: Model memory, full document dump, Artifact

**Knowledge Index Generation**:
The coherent searchable generation produced from the ready revisions in a Knowledge Base at a point in time. A queued Run freezes the generation it is allowed to query, while a later generation affects only later Runs or an explicit retry.
_Avoid_: AnythingLLM workspace, mutable search state, Workflow Snapshot

**Knowledge Citation**:
A bounded, permission-checked provenance record for a Retrieval Context excerpt, identifying its Knowledge Base, Category, Document Revision, source location, relevance, and safe display text. It remains auditable in Run history without becoming an Artifact or granting unconditional source access.
_Avoid_: Raw provider response, full document copy, download URL

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
A fail-fast collaboration in which the platform schedules every Subagent in Team Member order. Each Subagent receives the current task, bounded conversation context, attachments, and all preceding Subagent results, then executes using the shared frozen execution configuration. The final member produces the official response and retry restarts the whole collaboration.
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

**CLI Connector Definition**:
An Administrator-owned, platform-wide definition of one Third-party CLI's icon, name, capability description, immutable installation source, derived executable contract, capabilities, authentication, and execution policy. Users may use but never create or modify it.
Administrator deletion retains a disabled historical record, removes catalog visibility and mutable Expert bindings, and revokes User access without changing frozen snapshots.
_Avoid_: CLI authorization, MCP Connector, arbitrary package command

**CLI Connector Authorization**:
A User-private account authorization under one enabled CLI Connector. It records the authorized external identity and protected, versioned credentials without exposing them to the Administrator or an Expert Snapshot.
_Avoid_: CLI Connector Definition, Connector Enablement, shared platform credential

**CLI Connector Enablement**:
A User's activation of one available CLI Connector Definition before selecting or using it. It is distinct from each external account authorization under that Connector.
_Avoid_: CLI Connector Authorization, Expert selection, Run

**Connector Command Approval**:
A time-bounded User decision required before one high-risk CLI Connector command executes. Approval is specific to the displayed Connector, identity, operation, and target; expiry or rejection prevents that command from running.
_Avoid_: Connector authorization, permanent permission, implicit consent

**MCP Connector**:
A private User-owned or Administrator-created Platform Resource reached through Streamable HTTP or started as a fixed-version `npx` or `uvx` stdio process inside an isolated Runtime environment.
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
