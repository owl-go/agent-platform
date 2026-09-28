# AI Applications Product Requirements

Status: proposed for implementation on 2026-09-19. This document defines the `AI Applications` product area and its first three entries: `Smart Assistants`, `Digital Humans`, and `Image Creation`.

## 1. Product Outcome

`AI Applications` is a top-level Agent Workspace area for creating and using durable, user-owned AI capabilities. It groups persistent scenario applications and task-specific creation tools without turning them into one shared database aggregate.

The first version contains:

1. `Smart Assistants` (`智能助手`)
2. `Digital Humans` (`数字人`)
3. `Image Creation` (`图片创作`)

The product must not use `AI Customer Service` (`AI 客服`) as the canonical entity name. Customer service is one `Smart Assistant` scenario type alongside pre-sales consulting, after-sales support, product guidance, enterprise knowledge questions, recruitment, training, and other user-defined scenarios.

`AI Applications` is distinct from the existing Agent Workspace areas:

- A `Session` is a continuing conversation.
- A `Workflow` is a reusable executable definition.
- An `Expert` is reusable specialist guidance.
- A `Knowledge Base` is a searchable source collection.
- A `Smart Assistant` packages a scenario, resources, interaction rules, and an entry point into a durable application.
- A `Digital Human` controls the presentation identity and interaction style used by one or more Smart Assistants.
- `Image Creation` is a task-oriented workbench and does not become a Smart Assistant or Digital Human.

## 2. Navigation And Information Architecture

The authenticated navigation exposes one `AI Applications` entry. Its focused surface contains three tabs or sub-entries:

```text
AI Applications
├── Smart Assistants
├── Digital Humans
└── Image Creation
```

The product uses these canonical paths:

```text
/ai-apps
/ai-apps/assistants
/ai-apps/assistants/:assistantId
/ai-apps/digital-humans
/ai-apps/digital-humans/:digitalHumanId
/ai-apps/image-creation
```

The former `/ai-creation/image-generation` route, if already deployed, redirects to `/ai-apps/image-creation`. The user-facing navigation label is `Image Creation` (`图片创作`); existing lower-level `Image Generation Record` data and APIs remain valid unless a later migration explicitly changes them.

The AI Applications landing surface may show summary cards for all three entries, but each entry keeps its own list, editor, lifecycle, permissions, and empty state. `AI Applications` is a product area, not a parent record that owns every child object.

## 3. Ownership And Privacy

- Every Smart Assistant and Digital Human is owned by exactly one authenticated User.
- User-owned Smart Assistants, Digital Humans, their configurations, and their conversations are private to that User.
- An Administrator cannot read or mutate User-owned application content in the first version.
- Administrator-created platform Experts, Skills, Connectors, and public Knowledge Bases may be selected where the existing resource rules allow it.
- Image Creation records, prompts, Reference Images, and Generated Images retain the existing owner-isolation rules.
- A later platform-template feature may introduce Administrator-created application templates, but it is out of scope for this version.

## 4. Smart Assistants

### 4.1 Definition

A `Smart Assistant` is a reusable, user-owned AI application for one scenario. It can answer questions, guide decisions, retrieve knowledge, or perform a constrained service interaction. It is not limited to customer support.

One User may create any number of Smart Assistants. Each Assistant has an independent identity, configuration, state, resource selection, and conversation history.

### 4.2 Scenario Types

The first version provides these optional scenario types:

- Customer consultation (`客服咨询`)
- Pre-sales advisor (`售前顾问`)
- After-sales support (`售后支持`)
- Product guide (`商品导购`)
- Enterprise knowledge assistant (`企业知识助手`)
- Recruitment assistant (`招聘助手`)
- Training assistant (`培训助手`)
- Custom (`自定义`)

Scenario type is a discovery and display classification. It does not change the execution contract or grant extra permissions.

### 4.3 Configuration

The minimum editable configuration is:

- Name
- Uploaded icon or avatar
- Short description
- Welcome message shown when a User opens the Assistant for the first time
- Scenario type
- Assistant prompt
- User-question pre-processing prompt
- One available Provider Model from a connection with a configured API Key and `openai_chat` protocol
- Response style
- Optional Knowledge Selection containing one or more Knowledge Bases
- Optional Expert or Expert Team selection
- Optional Digital Human selection
- Zero or more configured Frequently Asked Questions (FAQs)
- An Answer Safety Policy defining the assistant's service scope and non-overridable platform safety rules
- Optional public Share Configuration for the authenticated Web UI and iframe embedding
- Enabled or disabled state

The configuration must not contain hidden, unreviewable instructions. User-authored prompts remain visible in the editor and are included in the application snapshot used by a conversation. The former Service Objective, Answer Scope, and Operating Rules editor fields are not part of the current configuration surface.

The Assistant editor selects a Provider Model independently of the User's Personal Settings. Only available models whose connection has a configured API Key and supports `openai_chat` may be selected; the backend repeats this check on create, update, enable, and new conversation creation. The Assistant does not select a Runtime Engine. Existing Assistants without a saved model keep the Personal Settings default as a compatibility fallback only when that model also supports `openai_chat`; editing them requires an explicit selection.

### 4.4 Lifecycle And Operations

A User can:

- Create, edit, copy, enable, disable, and delete a Smart Assistant.
- Open an Assistant detail surface and start a new conversation.
- View conversations belonging to that Assistant.
- Search and filter the Assistant list by name and scenario type.

The initial lifecycle is:

```text
Draft -> Enabled -> Disabled
```

An incomplete or invalid Assistant remains editable but cannot start a new conversation. A disabled Assistant cannot accept new conversations; existing conversations retain their frozen snapshots and remain readable according to Session retention rules.

### 4.5 Conversation Execution

- Opening an enabled Assistant reopens its most recently created authenticated Assistant Conversation, or creates one when none exists; it never opens a Workspace Session. The User can explicitly create a new conversation and can always access saved Assistant Conversation history, even when only one conversation exists. The welcome appears as the first Assistant chat message with the uploaded Assistant icon; enabled FAQs appear beside that message as compact, labeled question shortcuts before the first User question. Later Assistant replies use the same left-aligned message and icon treatment, while User questions appear on the right.
- Selecting an FAQ returns its stored answer without model invocation. Free text is preprocessed by the configured Provider Model to classify an FAQ, reject an out-of-scope question with `对不起，我暂时无法回答此类问题`, or continue.
- A continuing question retrieves relevant results from the Assistant's Knowledge Bases, then streams the model's answer with those results; when no results are found, the model may answer directly.
- The conversation freezes the Assistant's selected Provider Model identifier and Assistant configuration on creation. Each model-backed turn resolves that Provider Model's current Model Provider Connection, including its Endpoint, protocol, model identifier, connection version, and protected current credential. Later Assistant edits affect only new conversations, except that enabled FAQ answers are read at answer time.
- Each accepted question first emits a thinking state. Only one turn can generate in a conversation; the User may stop it or create a new conversation. A new conversation does not erase the previous transcript.
- A failed Assistant reply remains in the transcript as a visually distinct error message, including any partial output already streamed. The chat composer keeps the growing question field and send or stop action in one focused input surface.
- The model receives at most the latest 10 completed turns, with a compressed summary if the context grows too long. Every turn, including failed or cancelled turns and partial output, remains in the owner's audit transcript independently of model context pruning.
- Each model stage uses Credit admission and settlement; provider credentials never enter snapshots or responses.
- An Assistant without a Digital Human remains usable as text or voice according to the supported client surface.
- An Assistant cannot silently invoke another User's resources.

### 4.6 Deletion And Resource Conflicts

- Deleting a Smart Assistant requires confirmation.
- Deletion removes its mutable configuration and prevents new conversations.
- Historical conversations retain the immutable snapshot needed to render their identity and results.
- Deletion does not delete referenced Experts, Knowledge Bases, Skills, Connectors, or Digital Humans.
- If a future external channel or active session prevents deletion, the API returns an explicit conflict instead of silently detaching live work.

### 4.7 Frequently Asked Questions

A Smart Assistant may contain any number of ordered Frequently Asked Questions. Each FAQ contains question text, a Markdown answer, display order, enabled state, and optional category, tag, or icon. FAQ import and export use question and answer columns; imported duplicate questions replace the previous answer.

An external or authenticated User can select an enabled FAQ and receive its stored answer directly. A direct FAQ answer does not invoke a Provider Model, retrieve a Knowledge Base, or consume Credits. FAQ Markdown is rendered through the existing safe Markdown boundary; raw HTML, scripts, and unsafe external content are not executable.

Free-text input follows the same ordered decision pipeline for every access surface:

```text
Safety pre-check
  -> model FAQ and scope classification
  -> direct FAQ answer or fixed out-of-scope refusal
  -> Knowledge Base retrieval when no FAQ matches
  -> grounded answer or model-only answer when no results exist
```

FAQ matching returns a stable FAQ identity and confidence. It never rewrites a matched FAQ answer. If an enabled Assistant changes an FAQ, the change affects new requests immediately; a response already returned to a User is not rewritten.

### 4.8 Answer Scope And Safety

Every Smart Assistant is protected by an Answer Safety Policy containing the platform safety categories and a fixed refusal message. The current Assistant editor does not expose a separate Answer Scope field; product safety remains enforced outside the user-authored prompt and cannot be weakened by the User.

Platform-level rules always reject requests involving sensitive political or military content, weapon construction, violent crime, dangerous operations, security bypasses, or other configured safety categories. The policy is evaluated before FAQ matching, Knowledge Base retrieval, or answer generation. A rejected request returns exactly the localized fixed message:

```text
暂时无法回答此类问题
```

The English equivalent is `I'm unable to answer that type of question at the moment.` The response does not identify the matched safety category, expose the rule, or return the sensitive source text.

Safety enforcement uses deterministic rule matching first and a lightweight safety classifier when needed. Deterministic checks do not consume Credits; a classifier invocation is a real model call and is charged to the Smart Assistant owner under the existing Credit contract. FAQ clicks that passed publication checks do not run a second classifier.

FAQ answers are safety-checked before they can be enabled. Knowledge Documents are safety-checked before indexing; a blocked source never enters a Ready retrieval generation. A later policy revision can disable an FAQ or remove a source from the active index without deleting its audit metadata.

The platform retains only a minimum safety audit record: Smart Assistant, time, access source, classification result, policy version, and Credit outcome. It does not retain the complete sensitive input in ordinary application logs or expose it to the Administrator as User content.

### 4.9 External Sharing And Iframe Embedding

A User may enable a Smart Assistant for anonymous public use through a Share Configuration. Sharing is independent per Assistant and exposes only the rendered Assistant UI, FAQ answers, grounded answers, and the Digital Human presentation when configured.

Share Configuration contains:

- Enabled or disabled state
- Unpredictable share Token and Token revision
- Optional HTTPS allowed-Origin list
- iframe width, defaulting to `100%`
- iframe height, defaulting to `600px`
- Generated iframe snippet
- Token regeneration and immediate revocation actions

Width accepts `100%` or a fixed pixel value from 320px through 1920px. Height accepts a fixed pixel value from 400px through 1600px. These values affect the generated snippet only; the public page remains responsive. Updating the values does not mutate already-copied snippets.

The generated embed resembles:

```html
<iframe
  src="https://example.com/embed/assistant/{share-token}"
  width="100%"
  height="600px"
  frameborder="0"
  allow="microphone"
></iframe>
```

The iframe provides FAQ shortcut buttons and free-text conversation. New external conversations freeze the Assistant's selected `openai_chat` Provider Model and use the same preprocessing, Knowledge Base, Credit, and answer-audit pipeline as authenticated Assistant conversations; older external Session conversations keep their original execution snapshot. It never exposes Provider Model, Runtime Engine, Expert, Knowledge Base internals, User Access Tokens, API Keys, internal Session IDs, Object Keys, signed URLs, or private application settings.

External visitors are anonymous and receive a short-lived Visitor ID represented only by a one-way hash in platform records. New free-text interactions create or continue a visitor-scoped Assistant Conversation, hidden from the owner's private Assistant Conversation history; pre-existing `External Conversation` records remain readable through their original Session path. The public conversation retains the Assistant and selected Provider Model snapshots, Share Token revision, answer source, and Credit settlement needed for audit.

Free-text external model calls consume the Smart Assistant owner's Credits. The owner may disable free-text questions or set an Assistant-level daily call limit; direct FAQ clicks do not consume model Credits. Platform-wide concurrency, IP, Visitor ID, and Share Token rate limits always apply and cannot be disabled by the owner.

Share Token validation is server-side. HTTPS embedding is required outside local development. The server emits an appropriate CSP `frame-ancestors` policy and validates `postMessage` origins; public access never relies on a client-only visibility flag. Disabling sharing or regenerating the Token immediately invalidates the old iframe.

## 5. Digital Humans

### 5.1 Definition

A `Digital Human` is a reusable presentation identity for an AI interaction. It describes who appears to the User and how that identity speaks or behaves; it does not own the business knowledge or answer logic.

One User may create any number of Digital Humans. A Digital Human may be referenced by multiple Smart Assistants.

```text
Digital Human A ──┬── Smart Assistant 1
                  ├── Smart Assistant 2
                  └── Smart Assistant 3
```

A Smart Assistant may omit a Digital Human and use a text-only or otherwise supported non-avatar presentation.

### 5.2 Configuration

The first version reserves these configuration concepts:

- Name
- Avatar or character asset
- Voice
- Language
- Expression or communication style
- Supported actions, gestures, or animations when supplied by the selected provider
- Scene or background when supplied by the selected provider
- Enabled or disabled state

Provider-specific secrets and credentials remain protected configuration. They are never returned in ordinary read APIs, browser payloads, conversation snapshots, logs, or generated artifacts.

The first version does not require a Digital Human to own an Expert, Knowledge Base, model, or conversation. Those belong to the Smart Assistant that uses it.

### 5.3 Lifecycle And Reuse

A User can create, edit, copy, preview, enable, disable, and delete a Digital Human.

- A disabled Digital Human cannot be selected for a new Assistant conversation.
- Existing conversations preserve the Digital Human identity and relevant configuration snapshot they started with.
- Editing a Digital Human affects only new conversations or explicit reselection.
- A Digital Human referenced by one or more Smart Assistants cannot be deleted silently. The product must either require detachment first or present an explicit detach-and-delete confirmation.
- Deleting a Digital Human never deletes or changes the Smart Assistant's knowledge, Expert, Skill, Connector, or conversation history.

### 5.4 Preview

Preview is a configuration check, not a production conversation. It may exercise the selected provider when required, but it must not create a Smart Assistant conversation, alter historical snapshots, or expose provider credentials. Provider charges, if any, must be made explicit before implementation of live preview.

## 6. Image Creation

### 6.1 Product Placement

`Image Creation` is the renamed user-facing entry for the existing image-generation workbench. It is now nested under `AI Applications`, but it remains a separate task-oriented tool rather than a Smart Assistant or Digital Human.

The workbench keeps its existing product behavior:

- Text-to-image and image-to-image requests
- Image Model selection and Administrator verification
- Prompt Optimization
- Ordered Reference Images
- Background execution and progress recovery
- Cancellation, partial success, and unknown provider outcome handling
- Regeneration, reuse settings, preview, download, ZIP, expiry, and history
- Image Credit Reservation and exact terminal settlement

The existing `Image Generation Record`, `Generated Image`, `Reference Image`, `Image Model`, `Image Credit Rate`, and `Image Credit Reservation` terms remain authoritative domain terms. Only the product-area label changes from `AI Creation` to `Image Creation`.

### 6.2 Boundary

- Image Creation does not bind to a Smart Assistant, Digital Human, Expert, or Knowledge Base.
- Image Creation does not create a Session, Workflow, or Run.
- Workflows, Scheduled Triggers, Workflow API credentials, and external image-generation APIs cannot start Image Creation in the first version.
- Image Creation may reuse an unexpired Generated Image as a Reference Image under its existing rules.

## 7. Cross-Object Relationships

```text
Smart Assistant
├── optional Knowledge Selection
├── optional Expert or Expert Team
├── optional Digital Human
└── Assistant conversations

Digital Human
└── reusable presentation identity for zero or more Smart Assistants

Image Creation
└── independent image-generation workbench and records
```

The semantic distinction is:

```text
Digital Human = who appears to the User
Smart Assistant = which scenario the application serves
Expert = what specialist guidance it uses
Knowledge Base = which source material it can retrieve
Image Creation = a separate tool for creating images
```

## 8. Shared Security And Execution Rules

- All long-running Assistant operations accept and propagate `context.Context` through the existing application and Runtime boundaries.
- User input is passed through controlled values, files, or standard input; no Assistant configuration is concatenated into shell commands.
- Secrets are materialized only for the single execution that needs them and pass through the existing exact-value redaction boundary.
- Assistant conversations use the existing Session execution and Credit contracts. The product does not add an untracked model invocation path.
- Resource revisions and Digital Human identity are frozen in the conversation snapshot. Later edits do not rewrite history.
- The FAQ, safety, and retrieval pipeline executes before any model invocation and records its source decision in the External Conversation or Session response snapshot.
- APIs return safe structured errors and do not expose raw provider responses, private prompts, credentials, signed URLs, or internal object keys.
- Mobile layouts provide the same list, editor, preview, and conversation actions without requiring hover-only controls.

### 8.1 Knowledge Retrieval

Knowledge Base source management accepts text and Markdown documents. A document revision is designed to move through `Accepted`, `Processing`, `Ready`, or `Failed`; only Ready revisions may participate in retrieval. Processing failure retains the source and permits a retry. The current deployment has no active Retrieval Provider, so new jobs remain unprocessed and selected knowledge fails closed rather than being treated as indexed.

The provider-neutral retrieval path is shared by Knowledge Base test search, Workflow Runs, and authenticated/new shared Smart Assistant conversations. Platform authorization and the latest Ready Document Revision must be checked before any candidate is returned or cited. No provider or embedding setting is exposed per Assistant or in the browser.

The application owns Knowledge Base, Document, Document Revision, permissions, Assistant binding, and retained Knowledge Citation records. A future provider may own chunk indexing and recall only after matching the shared ingestion, authorization, provenance, and Conformance contract.

When provider-backed ingestion is available, reindexing a Ready document creates a new immutable revision from its saved source. The previous Ready revision remains eligible while the new ingestion is pending or fails. A successful replacement advances the platform generation and makes older revisions ineligible. A Workflow Run frozen to an unavailable generation fails closed instead of substituting different content.

When retrieval produces no eligible excerpts, the Assistant may answer directly with its selected Provider Model, subject to its safety and scope policy; it must not imply that an ungrounded answer came from a Knowledge Base. A grounded response stores bounded citation metadata identifying the Knowledge Base, Document Revision, source location, relevance, and safe display text.

## 9. Out Of Scope For The First Version

- Public application marketplace or cross-User sharing
- WeChat, DingTalk, Feishu, or other channel adapters beyond the authenticated Web UI and iframe embed
- Multi-channel unified conversation history across different external adapters
- Human-agent queues, ticketing, SLA rules, or agent seats
- Live-streaming Digital Human broadcasts
- Multiple Digital Humans collaborating in one conversation
- Custom Digital Human training or user-uploaded model weights
- Assistant-level Runtime Engine policies
- Detailed product analytics beyond basic share usage, safety audit, and existing conversation/execution history

## 10. Acceptance Boundary

Completion requires real browser-to-API closure for both ordinary User and Administrator-visible flows where applicable. At minimum, acceptance covers:

- AI Applications navigation, landing state, responsive tabs, and the `Image Creation` rename
- Smart Assistant create, edit, copy, list, search, scenario classification, enable, disable, delete, owner isolation, and incomplete-state handling
- Smart Assistant Knowledge Base, Expert, Expert Team, and Digital Human selection with validation and explicit unavailable-resource errors
- Smart Assistant conversation entry, configuration snapshot, resource revision snapshot, credit admission, cancellation, history, and later-edit isolation
- Smart Assistant FAQ CRUD, ordering, safe Markdown rendering, direct-answer no-Credit behavior, deterministic and classifier-based FAQ matching, confidence thresholding, and immediate publication behavior
- Smart Assistant Answer Safety Policy, non-overridable platform categories, fixed localized refusal, FAQ/Knowledge Document publication checks, classifier charging, minimum safety audit, and no sensitive-content leakage
- Smart Assistant Share Configuration, unpredictable Token, Token rotation/revocation, allowed Origins, iframe width/height validation, generated snippet, anonymous Visitor ID, External Conversation isolation, owner Credit charging, and platform rate limits
- Digital Human create, edit, copy, preview, list, enable, disable, delete conflict, reuse across multiple Assistants, protected provider configuration, and snapshot behavior
- Image Creation route, old-route redirect, existing Image Generation behavior, existing Image Model administration, history, notifications, owner isolation, and credit settlement
- Knowledge Base text/Markdown source management, asynchronous lifecycle records, safety gating, permission-checked revision provenance, and fail-closed behavior while no Retrieval Provider is active; any replacement requires pinned-deployment end-to-end evidence
- historical conversations remain readable after an Assistant or Digital Human is disabled or deleted, subject to existing retention and artifact rules
- no Assistant, Digital Human, or Image Creation API leaks credentials, private content, provider responses, internal object keys, or signed URLs

Real provider, Runtime, Digital Human, and external-channel claims remain unverified until their specific live or Conformance evidence is executed. A parser or configuration field is not evidence that a provider capability works in production.

## 11. Deferred Decisions

The following decisions must be made before implementing external delivery or live Digital Human providers:

1. Which initial Digital Human provider and protocol are supported?
2. Is live voice/video interaction part of the first release, or is the first release configuration and preview only?
3. Which external channels, if any, receive a Smart Assistant after the authenticated Web and iframe surfaces are complete?
