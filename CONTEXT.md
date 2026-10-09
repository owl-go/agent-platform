# Agent Workspace

A personal AI workspace for private conversations, reusable Workflows, Experts, Expert Teams, Skills, Connectors, and managed execution.

## People And Ownership

**Agent Workspace**:
The product in which an authenticated User creates private Sessions, configures Workflows and Experts, and runs Workflows through managed Runtime Engines.
_Avoid_: Coding Agent Platform, multi-agent system

**User**:
An authenticated person who exclusively owns their Sessions, Workflows, Expert Teams, Personal Settings, Credit Balance, Credit Ledger, and privately created Experts, Skills, and MCP Connectors. A User may also access Platform Resources and Department Resources within their current Identity Group memberships.
_Avoid_: Organization member, Team member, product role

**Administrator**:
An authorized User who manages accounts, read-only Identity Group synchronization, governance roles, Department budgets, Platform Resources, the platform-wide Model Catalog, Daily Credit Allocations, Model Credit Rates, Redemption Codes, and reasoned Credit Adjustments without access to private User-owned content or execution-level consumption. One immutable Bootstrap Administrator guarantees that the deployment cannot lose its final Administrator.
_Avoid_: Platform operator, Organization administrator, support user

**Resource Publisher**:
An authorized User who may create and maintain Department Resources only inside a Department where they currently hold Identity Group membership. The role does not grant Administrator access, access to another Department, or access to any User's private content.
_Avoid_: Administrator, resource owner bypass, global editor

**Registration Method**:
An Administrator-managed external identity route for scan sign-in and first-use creation of an ordinary User. It has its own availability and provider application configuration; it never grants governance roles or authorizes a Connector or Workflow Message Channel.
_Avoid_: Connector Authorization, Message Channel Account, password registration

**Registration Attempt**:
A short-lived, browser-bound exchange that ties one external identity proof to one sign-in request. Its WeChat one-time code identifies that exchange; confirmation can complete only that exchange and cannot be reused for another browser or User.
_Avoid_: User session, Connector Authorization, Message Channel Sender Pairing

**Identity Group**:
A read-only local projection of one group and its memberships from the configured enterprise identity source. A Group marked as a Department may scope shared resources and a daily Credit budget; the product never maintains a second writable organization tree.
_Avoid_: product-managed Team, role grant, local mailing list

**Department Resource**:
A shared resource bound to exactly one active Department. Department members may read it and Resource Publishers in that Department may maintain it; access is revoked when the Group or membership disappears from the latest identity synchronization. A disabled former custodian may be outside the current membership snapshot while an Administrator transfers only that Department's resources to a current Resource Publisher.
_Avoid_: Platform Resource, public resource, User-private resource

**Governance Audit Event**:
An append-only record of an Administrator governance mutation containing the actor, action, target identifier, required reason, bounded numeric metrics, and occurrence time. It never stores prompts, replies, filenames, private resource contents, credentials, or external tokens.
_Avoid_: Product Event, Runtime Event, private-content transcript

**Product Event**:
An append-only, privacy-bounded measurement record derived from a confirmed product transition. It contains only a stable anonymous User key, an optional anonymous subject key, an allowlisted event name, coarse state or duration attributes, and occurrence time; prompts, replies, filenames, Object Keys, external accounts, credentials, and signed URLs are forbidden.
_Avoid_: Runtime Event, audit transcript, request log, user content

## Credits And Usage

**Credit**:
A product usage unit available to one User and consumed by Provider Model usage; at a Model Credit Rate of 1.00, one Credit represents 10,000 input or output Tokens, while Image Credit Rates price successfully Generated Images directly. A User without available Credits cannot start another model execution.
_Avoid_: Currency, Provider charge, Token

**Credit Balance**:
The sum of one User's remaining Daily Credit Allocation and Redeemed Credit Balance. A completed execution may make it temporarily negative, preventing another execution until Credits become available again.
_Avoid_: Daily Credit Limit, Provider balance, Token balance

**Available Credit**:
The portion of one User's Credit Balance not withheld by active Image or Execution Credit Reservations and available to admit another execution. When any current Department membership has a daily Credit budget, Available Credit is capped by the smallest remaining applicable Department budget and the limiting Department is shown to the User.
_Avoid_: Credit Balance, reserved Credit, Provider balance

**Department Credit Budget**:
An optional daily aggregate admission limit for all current members of one Department. It counts settled Credit Consumption and active reservations for the Credit Day, is enforced transactionally before model or image execution, and never replaces a User's own Credit Balance.
_Avoid_: User Daily Credit Allocation, Provider budget, accounting invoice

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
A private, continuing text conversation owned by one User. Each response may use one Expert, one Expert Team, or no specialist; the selected specialist persists until changed. Each new response resolves the current Personal Settings execution configuration and freezes it for that response.
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
A reusable executable configuration that combines a name, goal, optional Expert or Expert Team, environment, API access, schedule, optional Workflow Message Channels, an optional Knowledge Selection, and one persistent Workspace. It is a single execution definition, not a visual graph or arbitrary DAG.
_Avoid_: Pipeline, visual DAG

**Workflow Queue**:
The persistent FIFO of queued Runs for one Workflow. It serializes access to that Workflow's Workspace while allowing Runs belonging to other Workflows to execute concurrently; its first-version limit is five queued Runs.
_Avoid_: User-wide execution queue, Run Conversation, Workspace lock

**Workflow Snapshot**:
The immutable copy of a Workflow's goal, initial execution stages, environment, and other initiating inputs used by one Run Conversation. Follow-up Response Snapshots preserve that context while recording each turn's specialist and resource selection. API Keys are referenced through protected versioned credentials rather than copied into the ordinary snapshot.
_Avoid_: Published Workflow, Workflow release

**Session Workflow Origin**:
The immutable, owner-scoped provenance link from one successful Session response to the Workflow and first validation Run created from it. It supports navigation and idempotent conversion; the Session and Workflow do not share later history.
_Avoid_: copied Session, shared conversation, Workflow version

**Execution Stage Snapshot**:
The immutable execution identity for one model invocation within a Response Snapshot or Workflow Snapshot, including its optional Expert and Team Member identities, Provider Model, Model Provider Connection version, API Protocol, Runtime Engine, structured Expert guidance, Skills, and Connectors. An execution without an Expert has one anonymous stage; an Expert Team has one ordered stage per member.
_Avoid_: Expert Stage result, mutable Expert, team Runtime Engine

**Workflow API Credential**:
The single API Key and encrypted, owner-viewable API Secret pair used only to exchange for a 72-hour Workflow Access Token. Regeneration immediately invalidates the previous pair and every token derived from it.
_Avoid_: User Token, Idempotency Key, model credential

**Scheduled Trigger**:
An optional hourly, daily, or weekly schedule that starts a Workflow from its fixed goal in a selected time zone.
_Avoid_: API call, Webhook, file watcher

**Workflow Message Channel**:
A User-owned configuration bound to one Workflow that accepts external messages under a Message Channel Audience and returns that Workflow's answers to their originating chat. Its authenticated account, reception scope, and enablement are separate from the Workflow's execution configuration and do not grant external participants a product User identity.
_Avoid_: Connector Authorization, Workflow API Credential, notification-only Webhook

**Message Channel Account**:
The authenticated bot or messaging-service identity used by one Workflow Message Channel, including its enterprise scope where applicable. It is distinct from an external sender identity and from a User's Connector Authorization.
_Avoid_: User, allowed sender, Connector Authorization

**Message Channel Audience**:
The explicit sender identities and permitted private or group chat scope for one Workflow Message Channel. An allowed group message also requires an allowed sender and a mention of the channel's bot; a display name or group membership alone grants no access.
_Avoid_: Identity Group, Department membership, anonymous public access

**Message Channel Sender Pairing**:
A temporary, one-use exchange that identifies a candidate external sender through an authenticated Message Channel Account. Recognition changes no permission until the owning User explicitly adds the candidate to the Audience and saves it; pairing is separate from channel validation and Workflow execution.
_Avoid_: Connector Authorization, channel enablement, automatic permission grant

**Message Channel Validation**:
A time-bounded proof that an allowed sender can deliver the expected validation message through the current channel configuration and receive its confirmation in the originating chat. It is required before enablement and is distinct from account authentication, connection health, Sender Pairing, and a Workflow Run.
_Avoid_: credential check, connection health, model validation Run

**Message Channel Inbox**:
The durable receipt of an external message that passed the channel's initial authentication and Audience checks, recording its identity and subsequent admission outcome. Receipt alone does not create a Run: validation messages, revoked access, and unsuccessful Workflow admission retain different outcomes.
_Avoid_: Workflow Queue, Run Conversation, Message Channel Delivery

**Message Channel Conversation**:
The link between one Workflow Message Channel's external conversation scope and one Run Conversation. Its scope includes the account and enterprise identity, sender, chat, and thread, preserving continuity without sharing history with another participant or channel.
_Avoid_: External Conversation, Session, Native Session

**Message Channel Response**:
An optional ongoing reply in the originating external chat that presents execution activity, a bounded public reasoning summary, a provisional answer, and eventually a terminal result or status where the messaging service supports it. A provisional answer is not a successful Run result, and the response never exposes private model reasoning or raw tool output.
_Avoid_: Message Channel Delivery, Runtime Event, private reasoning, completed Run result

**Message Channel Delivery**:
The record of returning an answer, bounded status, or validation confirmation to the originating external chat, with an outcome independent of any Run's execution outcome. Retrying a Delivery does not rerun the Workflow, and an uncertain send outcome requires explicit confirmation before a potentially duplicate resend.
_Avoid_: Runtime Event, Run retry, Artifact

**Run Conversation**:
A continuing conversation started by one manual, scheduled, API, Session conversion, or Workflow Message Channel trigger. It keeps the initiating Workflow Snapshot and contains one or more ordered Runs, so the User or an authorized external participant can follow up without losing the original goal, prior messages, or Workspace context.
_Avoid_: Session, one Runtime process, Run Event stream

**Run**:
One immutable execution turn inside a Run Conversation, with fixed input, one terminal result, the initiating Workflow context, and that turn's frozen specialist and resource selection. The first Run records the trigger; each follow-up creates another Run instead of reopening a terminal Run.
_Avoid_: Workflow, Run Conversation, Session response, Worker process

**User Action Wait**:
A non-terminal execution state in which a Session response or Run is paused until the User completes Connector authorization or approves a high-risk Connector command before a fixed deadline. No protected action executes without the required confirmation.
_Avoid_: queued execution, indefinite pause, automatic approval

**Execution Plan**:
The immutable, platform-generated objective, ordered Plan Steps, selected resource identities, bounded side-effect categories, and execution estimate shown for a qualifying Session response or manual Run. An explicitly requested Plan uses one separately metered Provider Model call to name task-specific steps and waits for a User decision; automatically required rule Plans use platform rules without that call and start without Plan confirmation. A requested Plan is confirmed, skipped only for a direct answer with no external operation, or cancelled as one versioned decision; no Plan replaces a high-risk Connector command approval.
_Avoid_: model chain-of-thought, Workflow definition, command approval

**Plan Step**:
One user-visible unit in an Execution Plan whose state is pending, running, completed, skipped, or failed. Worker-owned transitions project actual execution progress and are not inferred from generated prose.
_Avoid_: Runtime Event, Expert Stage Snapshot, hidden reasoning step

**Execution Activity**:
A bounded, redacted account of actual execution progress visible with a Session response or Run. It may contain public reasoning summaries and permitted activity details, but is distinct from an Execution Plan, retained Evidence, raw Runtime output, and private reasoning.
_Avoid_: Plan Step, Evidence, raw tool output, model chain-of-thought

**Deleted Workflow Record**:
The read-only name, Run history, and unexpired Artifacts retained after a Workflow and its Workspace are permanently deleted.
_Avoid_: Restorable Workflow, archived Workflow

## AI Applications

**AI Applications**:
The product area in which a User creates and uses durable AI capabilities and task-specific creation tools outside a continuing Session or Workflow. Its entries are Smart Assistants and Image Creation.
_Avoid_: AI Creation as a top-level area, Session, Workflow, generic AI tools

**Smart Assistant**:
A reusable, User-owned AI application for one scenario, combining a selected Provider Model, visible service rules, Frequently Asked Questions, an Answer Safety Policy, and optional Knowledge Selection and Expert or Expert Team references. One User may create many Smart Assistants; customer service is one scenario type rather than the entity name.
_Avoid_: AI Customer Service, Expert, Session, Workflow

**Smart Assistant FAQ**:
An ordered, User-authored question and Markdown answer attached to one Smart Assistant. A direct selection returns its safety-checked answer without a model invocation; uncertain free-text questions may be classified against enabled FAQs before Knowledge Base retrieval.
_Avoid_: Knowledge Document, canned Session message, generated answer

**Answer Safety Policy**:
The ordered, platform-enforced and Assistant-configurable boundary that decides whether a request may proceed to FAQ matching, Knowledge Base retrieval, or model answering. Platform safety categories cannot be disabled, and a rejected request receives the fixed localized refusal without exposing the matched category or sensitive source text.
_Avoid_: Prompt instruction, content filter toggle, moderation note

**Share Configuration**:
The per-Smart-Assistant public-use configuration containing an unpredictable share Token, an explicit non-empty HTTPS allowed-Origin list, a fullscreen or floating embed type, iframe dimensions, an initial floating-window display state, an independent chat icon, a data-processing acknowledgement, and revocation state. It allows free-text questions by default without a per-Assistant daily call cap and grants access only to the rendered Assistant surface and never exposes private credentials or internal Session identity.
_Avoid_: Public Session, API Key, User Access Token

**Publication Validation**:
A version-bound, server-recorded result proving that one Smart Assistant revision passed the platform publication checks for configuration, model availability, FAQ safety, Knowledge readiness, referenced resources, owner Credits, and Share Configuration. Any material Assistant edit invalidates the result; an enabled or shared Assistant must have a passing result for its current version.
_Avoid_: client-side checklist, permanent approval, production-provider evidence

**External Conversation**:
An anonymous conversation created through a Smart Assistant Share Configuration and kept separate from the owner's private Session list. Each accepted turn uses the Smart Assistant's current configuration while retaining its Share Token revision, answer source, and owner Credit settlement for audit.
_Avoid_: Session, visitor account, public chat transcript

**Assistant Conversation**:
An authenticated User's private conversation with a Smart Assistant, separate from a Workspace Session and retained as a complete audit transcript. A new conversation starts with the Assistant's welcome and enabled FAQs; each accepted turn uses the Smart Assistant's current configuration and only a bounded portion of prior messages as context.
_Avoid_: Session, External Conversation, temporary chat

**Image Creation**:
The task-specific image-generation tool nested under AI Applications. It owns no Smart Assistant and uses Image Generation Records for its submitted requests and results.
_Avoid_: AI Creation, Smart Assistant, Image Artifact

**Retrieval Provider**:
The optional, replaceable indexing and recall service used by Knowledge Base Ingestion and retrieval. Knowledge ownership, current permissions, Document Revisions, Knowledge Index Generations, Assistant bindings, and retained Knowledge Citations remain platform-owned.
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

**Attachment**:
An immutable reference to a User-uploaded file frozen onto one accepted conversation turn. It supplies input to that turn and remains distinct from a generated Artifact or a mutable Workspace file.
_Avoid_: Artifact, Workspace file, filename mention

**File Reference**:
A message's explicit reference to an attachment or Artifact from its own conversation, or to a file in its Workflow's Workspace. An accepted reference preserves the file content supplied with that message independently of later source changes or expiry.
_Avoid_: Mutable Workspace path, signed download URL, filename mention

**Knowledge Base**:
A logical collection of knowledge documents with one immutable ownership scope: private to one User, a Department Resource, or an enterprise-wide Platform Resource. Private content remains visible only to its owner; a Department Knowledge Base is readable by current Department members and maintainable by current Resource Publishers in that Department; a Platform Knowledge Base is readable by every authenticated User and maintainable only by its Administrator owner.
_Avoid_: Workspace, Artifact collection, shared file folder

**Knowledge Category**:
An optional first-level grouping inside one Knowledge Base. A Knowledge Document belongs to at most one Category, and may instead remain unclassified; Categories are not nested.
_Avoid_: Directory, tag, multi-parent folder

**Knowledge Document**:
A source supplied to a Knowledge Base as uploaded bytes or a captured web source. Within one Category, a normalized source identifies one logical document whose later uploads or refreshes create new revisions; the same source in another Category is a separate Knowledge Document rather than a multi-category assignment.
_Avoid_: Attachment, Workspace file, Artifact

**Document Revision**:
An immutable accepted source snapshot of a Knowledge Document. A later replacement creates a new revision; current retrieval uses the latest successfully ingested revision, while a frozen Knowledge Index Generation may retain an older Ready revision under current source permissions.
_Avoid_: Mutable file, overwrite, Runtime Snapshot

**Ingestion**:
The asynchronous processing of an accepted Knowledge Document Revision into searchable knowledge. Upload acceptance is independent of Ingestion completion; a revision moves through accepted, processing, ready, failed, or blocked, and only a ready revision is eligible for retrieval while failures retain the source for retry.
_Avoid_: Upload, synchronous import, Runtime execution

**Ingestion Job**:
A durable, idempotent task that performs one Knowledge Document Revision's extraction, OCR, chunking, and indexing outside the Runtime execution queue. It has a lease, retry history, and terminal failure state independent of the source bytes.
_Avoid_: Run, background goroutine, upload request

**Knowledge Selection**:
The set of Knowledge Bases chosen for retrieval by a Workflow or Smart Assistant. A Run freezes its selection in the initiating Workflow Snapshot, while each accepted Smart Assistant turn uses the Assistant's current selection under current source permissions.
_Avoid_: Runtime Engine setting, Conversation Selection, dynamic folder lookup

**Retrieval Context**:
A bounded set of authorized Knowledge Base excerpts returned for a retrieval preview, Run, or Smart Assistant answer, carrying source identity, location, and relevance. A Run uses its frozen Knowledge Selection and retains a redacted citation summary; the excerpts are not an Artifact or a replacement for the original document.
_Avoid_: Model memory, full document dump, Artifact

**Knowledge Index Generation**:
The platform's monotonically identified Ready state for a Knowledge Base after verified ingestion by an active Retrieval Provider. A queued Run freezes its immutable set of Ready Document Revisions; retained generations remain usable under current source permissions, while unavailable generations fail closed rather than being substituted with different content.
_Avoid_: Provider workspace, mutable search state, Workflow Snapshot

**Knowledge Citation**:
A bounded, permission-checked provenance record for a Retrieval Context excerpt, identifying its Knowledge Base, Category, Document Revision, source location, relevance, and safe display text. It remains auditable in Run history without becoming an Artifact or granting unconditional source access.
_Avoid_: Raw provider response, full document copy, download URL

**Evidence**:
A bounded, owner-visible execution fact retained with an Assistant Message or Run, derived only from a platform-controlled boundary such as Knowledge Retrieval or the CLI Connector Broker. It identifies a safe source, action, owning stage, and requested/succeeded/failed/not-used state; an optional Knowledge Citation may add provenance. Evidence never contains raw Tool Output, arguments, prompts, model responses, credentials, unrestricted external payloads, or private reasoning.
_Avoid_: Runtime log, model claim, Response Snapshot, Artifact

## Experts, Skills, And Connectors

**Platform Resource**:
An Expert, Skill, or MCP Connector owned by an Administrator and visible to every authenticated User in the platform section of its catalog. Administrator-created resources remain editable only by that Administrator; default Expert and Skill originals distributed with the platform are immutable and may be selected alongside private resources.
_Avoid_: shared User resource, public credential, CLI Connector Definition

**Expert**:
A reusable specialist profile with a display name, display-only Introduction, visible Expert Guidance, and selected Skills and Connectors. It does not select a Provider Model or Runtime Engine.
_Avoid_: Persona, Workflow

**Expert Guidance**:
The single authoritative, User-visible instruction document defining an Expert's capabilities, working method, output expectations, and constraints. Introduction remains separate display content rather than part of this guidance.
_Avoid_: hidden prompt, duplicated form instructions, Introduction

**Expert Package**:
A portable, credential-free definition of one Expert or Expert Team, including its profile, Expert Guidance, and any bundled Skills or profile images. Its external dependencies do not grant an external account authorization.
_Avoid_: Connector Package, credential bundle, Runtime image

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
A reusable specialist profile with an Introduction, a Team Lead, and named Team Members whose work the Team Lead coordinates for a task. Each execution uses an immutable snapshot of its selected team.
_Avoid_: User team, organization, visual workflow

**Team Lead**:
The role in one Expert Team responsible for interpreting the task, delegating work to selected Team Members, and producing the official response from their results.
_Avoid_: Administrator, User, final ordered member

**Team Member**:
A stably identified, named role in one Expert Team that references an Expert and has role-specific labels. Its member name, labels, and order may change without replacing its identity; the same Expert may be referenced by multiple Team Members with isolated execution contexts.
_Avoid_: Expert, User, organization member

**Member Label**:
A User-authored label describing one Team Member's responsibility within an Expert Team. It is distinct from the referenced Expert's Derived Expertise Tags.
_Avoid_: Derived Expertise Tag, Expert capability

**Subagent**:
A platform-managed execution of one Team Member in its own isolated execution context inside an Expert Team. It uses the execution configuration frozen for the current response or Run; Runtime-specific native subagent support is not required for this behavior.
_Avoid_: Expert selected alone, simulated persona, Runtime capability

**Expert Team Execution**:
A platform-controlled collaboration in which the Team Lead delegates work as the task requires and produces the official response. Members retain isolated execution contexts, and the platform bounds delegation, concurrency, and consumption.
_Avoid_: fixed-order chain, arbitrary agent graph, simulated personas

**Expert Snapshot**:
The immutable Expert or Expert Team definition used by a Session response or Run, including visible profile content, structured Expert guidance, Team Member roles, member order, and exact Skill and Connector revisions. A later specialist selection or source-profile edit does not change this snapshot.
_Avoid_: Current Expert, mutable team, execution configuration snapshot

**Incomplete Expert**:
A migrated or partially edited Expert missing required Introduction, Core Capability, Operating Procedure, or Output Standard content. It remains visible and editable but cannot be selected for a new Session or Run Conversation until completed.
_Avoid_: unavailable execution configuration, deleted Expert

**Connector**:
A selectable integration through which a Session response or Run accesses an external capability, with or without an Expert. A Connector is distributed as a versioned Connector Package and declares exactly one external access mode: MCP or CLI. User-specific authorization remains private to that User.
_Avoid_: Extension, Skill, Runtime Engine

**Connector Package**:
An immutable, versioned distributable containing `connector-meta.json`, `icon.svg`, exactly one mode manifest (`mcp.json` or `cli.json`), and at least one Skill directory with a required `SKILL.md`. It may enter the platform as an uploaded ZIP or be assembled through a guided creation flow, but both paths produce the same validated package contract. The package describes how its external capability is connected; its Skills explain how an Agent should use that capability. A package cannot declare both MCP and CLI modes.
_Avoid_: Skill-only package, Runtime image, credential bundle

**Connector Publication**:
The Administrator-managed catalog record that selects one active Connector Revision for a package source and declares it available or disabled. Publication creates neither a Connector Installation nor a Connector Authorization and owns no User credentials.
_Avoid_: Connector Installation, Connector Authorization, package upload

**Connector Installation**:
A User- or platform-scoped record that a validated Connector Package revision is available in the isolated installation area. Installation does not grant an external account authorization and does not imply that the Connector is connected.
_Avoid_: Connector Authorization, active Runtime process, package upload

**Connector Authorization**:
A User-private grant that lets one installed Connector access an external identity or service scope. The platform owns the encrypted credential boundary and revalidates the current grant before every external invocation; disconnecting it immediately blocks new invocations without deleting the Connector Installation.
_Avoid_: Connector Installation, permanent permission, shared platform credential

**Connector Revision**:
An immutable package revision identified by its Connector source, semantic version, package checksum, and exact runtime policy, separately referenced by a Connector Publication and each Connector Installation. Execution snapshots retain their selected revision and its companion Skills even when a Publication or Installation later changes its active revision.
_Avoid_: Mutable Connector configuration, authorization version, latest tag

**Connector Invocation Result**:
The platform-normalized success or failure envelope returned from an MCP tool or CLI command. It carries a request identity, structured data or a stable error category, retry guidance, and only redacted diagnostics.
_Avoid_: Raw process output, Provider response, Runtime event stream

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
A private User-owned or Administrator-created Platform Resource containing a required `SKILL.md` and optional scripts or resources, installed from a Git URL or uploaded archive. A Connector Package must carry at least one such Skill; a standalone Skill does not provide external access by itself. Scripts run only inside an isolated Runtime environment.
_Avoid_: Connector, Prompt, Runtime Engine

**Recommended Skill Offer**:
An Administrator-authored name and HTTPS Git source attached to a CLI Connector Definition. A User must explicitly install it, after which it becomes an ordinary User-owned Skill; enabling a Connector never installs or injects it implicitly.
_Avoid_: platform-owned Skill, hidden instruction, automatic installation

## Personal Configuration

**Personal Settings**:
A User's personality, default Runtime Engine, Runtime Engine Settings, language, and time zone. Its default Provider Model and Runtime Engine supply every new Session or Run Conversation's execution configuration, whether or not an Expert or Expert Team is selected.

**Platform Execution Default**:
The Administrator-managed Runtime Engine and Provider Model pair inherited by Users who have not opted into a personal execution override, never used as a silent fallback. Available compatible pairs can be saved directly, including those whose Provider or compatibility is unverified. Saving a default does not constitute verification or change verification status; incompatible pairs remain forbidden.
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
A User's preference for one Runtime Engine, including that engine's default Provider Model. It supplies the execution configuration frozen when a Session response or Run starts; Experts and Workflows do not override it.
_Avoid_: global default model, Personality model, fixed Session model

**Runtime Adapter**:
The platform boundary that presents one Runtime Engine through the common execution and event contract.
_Avoid_: Runtime Engine, model provider, CLI wrapper

**Runtime Capability**:
A behavior that a specific Runtime image can reliably provide only after its required conformance evidence passes.
_Avoid_: Parsed field, configuration flag, compatibility assumption
