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
- Icon or avatar
- Short introduction
- Scenario type
- Service objective
- Response style and service rules
- Optional Knowledge Selection containing one or more Knowledge Bases
- Optional Expert or Expert Team selection
- Optional Digital Human selection
- Enabled or disabled state

The configuration must not contain hidden, unreviewable instructions. User-authored service rules remain visible in the editor and are included in the application snapshot used by a conversation.

The Assistant uses the User's existing Personal Settings execution configuration unless a future product decision adds an explicit application-level model policy. The first version does not add a separate Provider Model or Runtime Engine selector to the Assistant editor.

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

- Opening an Assistant provides a dedicated conversation entry point.
- A conversation uses the Assistant's selected Knowledge Bases, Expert or Expert Team, Digital Human, and visible service rules.
- The first accepted message freezes the effective Assistant configuration and the existing Session execution configuration.
- Later Assistant edits affect only new conversations.
- A conversation continues to use platform-owned history, Rolling Summary, Credit admission, Runtime cancellation, event ordering, and secret redaction from the existing Session contract.
- An Assistant without a Digital Human remains usable as text or voice according to the supported client surface.
- An Assistant cannot silently invoke another User's resources.

### 4.6 Deletion And Resource Conflicts

- Deleting a Smart Assistant requires confirmation.
- Deletion removes its mutable configuration and prevents new conversations.
- Historical conversations retain the immutable snapshot needed to render their identity and results.
- Deletion does not delete referenced Experts, Knowledge Bases, Skills, Connectors, or Digital Humans.
- If a future external channel or active session prevents deletion, the API returns an explicit conflict instead of silently detaching live work.

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
- APIs return safe structured errors and do not expose raw provider responses, private prompts, credentials, signed URLs, or internal object keys.
- Mobile layouts provide the same list, editor, preview, and conversation actions without requiring hover-only controls.

## 9. Out Of Scope For The First Version

- Public application marketplace or cross-User sharing
- External web embedding, public links, WeChat, DingTalk, Feishu, or other channel adapters
- Multi-channel unified conversation history
- Human-agent queues, ticketing, SLA rules, or agent seats
- Live-streaming Digital Human broadcasts
- Multiple Digital Humans collaborating in one conversation
- Custom Digital Human training or user-uploaded model weights
- Assistant-level independent Provider Model or Runtime Engine policies
- Application analytics beyond the existing conversation and execution history

## 10. Acceptance Boundary

Completion requires real browser-to-API closure for both ordinary User and Administrator-visible flows where applicable. At minimum, acceptance covers:

- AI Applications navigation, landing state, responsive tabs, and the `Image Creation` rename
- Smart Assistant create, edit, copy, list, search, scenario classification, enable, disable, delete, owner isolation, and incomplete-state handling
- Smart Assistant Knowledge Base, Expert, Expert Team, and Digital Human selection with validation and explicit unavailable-resource errors
- Smart Assistant conversation entry, configuration snapshot, resource revision snapshot, credit admission, cancellation, history, and later-edit isolation
- Digital Human create, edit, copy, preview, list, enable, disable, delete conflict, reuse across multiple Assistants, protected provider configuration, and snapshot behavior
- Image Creation route, old-route redirect, existing Image Generation behavior, existing Image Model administration, history, notifications, owner isolation, and credit settlement
- historical conversations remain readable after an Assistant or Digital Human is disabled or deleted, subject to existing retention and artifact rules
- no Assistant, Digital Human, or Image Creation API leaks credentials, private content, provider responses, internal object keys, or signed URLs

Real provider, Runtime, Digital Human, and external-channel claims remain unverified until their specific live or Conformance evidence is executed. A parser or configuration field is not evidence that a provider capability works in production.

## 11. Deferred Decisions

The following decisions must be made before implementing external delivery or live Digital Human providers:

1. Which initial Digital Human provider and protocol are supported?
2. Is live voice/video interaction part of the first release, or is the first release configuration and preview only?
3. Which external channels, if any, receive a Smart Assistant after the authenticated Web conversation is complete?
4. Does a Smart Assistant later need an explicit model policy independent of Personal Settings?
