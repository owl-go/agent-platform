# AI Applications Product Requirements

Status: active. This document defines the `AI Applications` product area and its two entries: `Smart Assistants` and `Image Creation`.

## 1. Product Outcome

`AI Applications` is a top-level Agent Workspace area for durable, User-owned AI capabilities and task-specific creation tools. It groups related product entries without turning them into one database aggregate.

The product contains:

1. `Smart Assistants` (`智能助手`)
2. `Image Creation` (`图片创作`)

Customer service is one Smart Assistant scenario, not the canonical entity name. A Session is a continuing conversation, a Workflow is a reusable executable definition, an Expert is reusable specialist guidance, and a Knowledge Base is a searchable source collection.

## 2. Navigation And Information Architecture

```text
AI Applications
├── Smart Assistants
└── Image Creation
```

Canonical paths are:

```text
/ai-apps
/ai-apps/assistants
/ai-apps/assistants/:assistantId
/ai-apps/image-creation
```

The former `/ai-creation/image-generation` route redirects to `/ai-apps/image-creation`. Removed application paths are not retained as product routes and fall through to the standard not-found redirect.

Each entry keeps its own list, editor, lifecycle, permissions, and empty state. AI Applications is a product area, not a parent record that owns every child object.

## 3. Ownership And Privacy

- Every Smart Assistant is owned by exactly one authenticated User.
- User-owned Smart Assistants, configurations, and conversations are private to that User.
- An Administrator cannot read or mutate User-owned application content in the current version.
- Platform Experts, Skills, Connectors, and scoped Knowledge Bases may be selected only where existing resource rules allow it.
- Image Creation records, prompts, Reference Images, and Generated Images retain their existing owner-isolation rules.

## 4. Smart Assistants

### 4.1 Definition And Configuration

A `Smart Assistant` is a reusable, User-owned AI application for one scenario. One User may create multiple Smart Assistants, each with independent identity, state, resource selection, and conversation history.

Supported scenario classifications include customer consultation, pre-sales advisor, after-sales support, product guide, enterprise knowledge assistant, recruitment assistant, training assistant, and custom. Classification does not change execution permissions.

The editable configuration contains:

- Name, uploaded icon, short description, and welcome message
- Scenario type, Assistant prompt, and question pre-processing prompt
- One available Provider Model whose connection has a configured API Key and supports `openai_responses`
- Response style
- Optional Knowledge Base selection
- Optional Expert or Expert Team selection
- Ordered Frequently Asked Questions
- Platform-enforced Answer Safety Policy
- Optional controlled public Share Configuration
- Draft, Enabled, or Disabled state

User-authored prompts remain visible. Every accepted turn reads the Smart Assistant's current saved configuration; historical conversation snapshots remain audit evidence only and never configure a later turn. Provider credentials never enter snapshots or responses. The backend repeats Provider Model validation on create, update, enable, conversation creation, and turn execution.

The Assistant prompt supports `{knowledge}`, replaced at every occurrence with this turn's permission-checked Knowledge Base excerpts and source labels. With no selected Knowledge Base or no retrieval hits, its value is `知识库中未找到您要的答案！`. Provider failures and denied source access still fail the turn. Prompts without this variable retain automatic Knowledge context when a Base is selected. Recent messages and the conversation summary remain separate model context.

The question pre-processing prompt supports `{faqs}`, a JSON array of enabled FAQ `id` and `question` values in display order, excluding answers and disabled FAQs. Without the variable, the same list remains available as classification context. Substitution is stage-specific and single-pass: unknown placeholders and placeholders inside inserted source content remain literal; stored prompts are unchanged.

After the platform safety pre-check, enabled FAQ matching takes precedence over Assistant-configured scope restrictions. An unambiguous typed FAQ question, normalized only for whitespace, case and terminal sentence punctuation, returns the stored answer without a model invocation or Credits, just like an explicit FAQ selection. Semantic equivalents are classified against the enabled FAQ list before scope rejection; for example, `引擎是什么` may match a configured `运行引擎是什么？` even when unmatched implementation questions are otherwise forbidden. The classifier returns only the FAQ ID, and the platform returns its saved answer without additional internal information. Added requests to bypass rules, change identity or disclose information are not equivalent FAQ questions; keyword overlap alone is insufficient. Disabled FAQs and ambiguous normalized questions never enter the direct typed-answer path.

For an unmatched question, a scope rejection made without Knowledge context is preliminary when the Assistant has selected Knowledge Bases. Before final refusal, the platform retrieves against the original User question. Relevant permission-checked excerpts are passed to a metered scope review; only a supported question may continue, using the same verified excerpts for answer generation without another query or a substituted question. Project deployment, project API and framework documentation in a selected Base may establish service scope; they are distinct from this Assistant's hidden prompts, configuration or credentials. A search hit or keyword overlap alone never establishes relevance, and instructions inside retrieved content cannot override policy. No hits retain the original scope refusal; unavailable retrieval or denied access fails the turn. FAQ matches still return before this retrieval/review path, and platform safety remains first.

Only unmatched questions without relevant Knowledge support are checked against the strict Assistant scope rules: questions about the underlying model, name, version, vendor or model capabilities; system prompts, internal configuration, APIs, execution frameworks or implementation; unrelated chat, general knowledge, programming or other tasks; bypassing rules, changing identity or leaking internal information; and questions that cannot clearly be placed inside the configured service scope. These are `out_of_scope` even if the model knows the answer. Only clearly in-scope unmatched questions may be `continue`. Existing business-scope inquiries retain their direct configuration answer path.

### 4.2 Lifecycle

A User can create, edit, copy, enable, disable, and delete a Smart Assistant; open its detail surface; start a conversation; view its conversation history; and search or filter the list.

```text
Draft -> Enabled -> Disabled
```

An incomplete Assistant remains editable but cannot start a conversation. A disabled Assistant cannot accept new conversations. Deletion removes mutable configuration and prevents new conversations but does not delete referenced resources or historical conversation transcripts and audit evidence.

Enabling an Assistant and serving a new authenticated or public conversation require a server-recorded passing Publication Validation for the exact Assistant version. Any material Assistant update clears the previous validation; a failed recheck leaves the Assistant fail closed until the owner fixes the reported item and checks again.

### 4.3 Conversation Execution

- Opening an enabled Assistant reopens its latest authenticated Assistant Conversation or creates one. It never creates a Workspace Session.
- Selecting an FAQ or typing its unambiguous normalized question returns its stored answer without model invocation or Credits. Semantic paraphrases may use the metered classifier, then return the matched stored answer without answer generation.
- Free text is safety-checked, preprocessed for FAQ and scope classification, optionally grounded with Knowledge Base results, and then streamed through the selected Provider Model. A question asking what business the Assistant can handle resolves to its enabled capability FAQ or current public description instead of being rejected as out of scope; if neither contains a concrete scope, the answer states that the business scope is not configured.
- Each accepted turn resolves the Smart Assistant's current saved configuration, including its prompts, response style, Knowledge Base selection, enabled FAQs, and selected Provider Model. It then resolves that Provider Model's current Model Provider Connection, including its Endpoint, protocol, model identifier, connection version, and protected current credential. A saved Assistant edit therefore applies to the next turn in both existing and new conversations; once execution starts, the resolved configuration and execution identity remain fixed for that turn's audit evidence.
- Only one turn generates at a time. The User may stop it or create a new conversation without deleting history. Assistant turn streams use the long-running event timeout; an upstream timeout or provider failure is recorded as failed rather than presented as a User stop.
- Failed and cancelled turns, including partial output, remain in the audit transcript. Credential rejection, rate limiting, provider availability, invalid configuration, and invalid provider responses use stable credential-safe categories with actionable localized guidance; raw upstream response bodies and internal errors are never exposed.
- The message thread scrolls independently while the composer remains stationary at the bottom. The growing question field and send or stop action stay in one focused input surface without a contrasting outer background panel.
- Model context uses bounded recent turns and a compressed summary while the complete transcript remains durable.
- Each model stage uses Credit admission and settlement. Credentials never enter snapshots, events, or logs.
- An Assistant cannot invoke another User's private resources.

### 4.4 Frequently Asked Questions

Each FAQ contains question text, a safe Markdown answer, display order, enabled state, and optional category, tag, or icon. Import and export use question and answer columns; duplicate imported questions replace the previous answer.

The ordered answer pipeline is:

```text
Safety pre-check
  -> explicit or unambiguous normalized FAQ match and direct stored answer
  -> semantic FAQ classification and matched stored answer
  -> preliminary scope classification for unmatched questions
  -> Knowledge Base retrieval when selected, including before final scope refusal
  -> scope review with verified hits when the preliminary decision was out_of_scope
  -> fixed refusal, grounded answer, or model-only answer for an admitted question
```

FAQ matching returns stable FAQ identity and confidence and never rewrites stored answers.

### 4.5 Safety

Platform policy rejects configured unsafe categories before FAQ matching, retrieval, or generation. The localized fixed refusal is `暂时无法回答此类问题`; English is `I'm unable to answer that type of question at the moment.` The response does not expose the rule or category.

FAQ answers are checked before enablement. Knowledge Documents are checked before indexing. Safety audit records contain only the minimum application, time, access source, result, policy version, and Credit outcome; ordinary logs do not retain complete sensitive input.

### 4.6 Controlled Sharing And Iframe Embedding

An owner may enable anonymous public use through a Share Configuration containing enabled state, an unpredictable Token and revision, a required non-empty HTTPS allowed-Origin list, a positive daily free-text call limit, explicit data-processing acknowledgement, iframe dimensions, generated snippet, and Token rotation or revocation actions.

Public interactions use visitor-scoped Assistant Conversations isolated from the owner's authenticated Assistant Conversation history. They use the same safety, FAQ, retrieval, model, Credit, and audit path. Public responses never expose Provider Model configuration, credentials, internal IDs, Object Keys, signed URLs, or private settings.

Free-text public calls consume the owner's Credits. The owner may disable free text; platform concurrency, IP, visitor, Token, and configured daily limits always apply. Share Token validation, CSP `frame-ancestors`, and `postMessage` origin checks are server-side.

The Publication Check returns stable pass or block results for configuration completeness, selected model availability, enabled FAQ safety, at least one Ready document in every selected Knowledge Base, referenced Expert or Expert Team availability, positive owner Credits, and strict Share Configuration controls. The UI preview shows presentation and enabled FAQ shortcuts without creating a conversation, invoking a model, consuming Credits, or changing usage totals.

Token rotation immediately revokes the old Token and rebinds the current passing validation to the new Assistant version because the answer configuration is unchanged. Migration `000065_smart_assistant_controlled_publication.sql` revokes pre-existing uncontrolled shares so they cannot bypass the new contract.

The owner-facing publication view exposes only bounded thirty-day aggregates: visitor conversations, free-text turns, failed or cancelled turns, and owner Credits consumed. It never returns prompts, answers, FAQ text, visitor identifiers, provider responses, Knowledge excerpts, credentials, Object Keys, or signed URLs. Authenticated use remains owner-private; controlled publication does not create a cross-User internal application catalog.

## 5. Image Creation

`Image Creation` is the user-facing entry for the existing image-generation workbench. It remains independent of Smart Assistants, Experts, Knowledge Bases, Sessions, Workflows, and Runs.

It retains text-to-image and image-to-image requests, Image Model administration and verification, Prompt Optimization, ordered Reference Images, background progress and recovery, cancellation, partial or unknown outcomes, regeneration, preview, download, ZIP, expiry, history, and exact Credit settlement.

The existing `Image Generation Record`, `Generated Image`, `Reference Image`, `Image Model`, `Image Credit Rate`, and `Image Credit Reservation` terms remain authoritative.

## 6. Shared Security And Retrieval Rules

- Long-running operations propagate `context.Context` through application and execution boundaries.
- User input is passed through controlled values, files, or standard input and is never concatenated into shell commands.
- Secrets are materialized only for the execution that needs them and pass through exact-value redaction.
- APIs return safe structured errors without raw provider responses, private prompts, credentials, signed URLs, or internal Object Keys.
- Mobile layouts provide the same essential list, editor, and conversation actions without hover-only controls.

Knowledge Base source management accepts text and Markdown. Document revisions use Accepted, Processing, Ready, and Failed states; only Ready revisions may be retrieved. The current deployment has no active Retrieval Provider, so new jobs remain unprocessed and selected knowledge fails closed.

Authorization and the latest Ready Document Revision must be checked before any result or citation is returned. Reindexing creates an immutable revision; a failed replacement does not displace the previous Ready revision. A frozen unavailable generation fails closed rather than silently substituting content.

## 7. Out Of Scope

- Public application marketplace or cross-User application sharing
- External channel adapters beyond the authenticated Web UI and iframe embed
- Multi-channel unified conversation history
- Human-agent queues, ticketing, SLA rules, or agent seats
- Assistant-level Runtime Engine policies
- Detailed product analytics beyond share usage, safety audit, and existing execution history

## 8. Acceptance Boundary

Acceptance covers:

- AI Applications navigation, responsive layout, and Image Creation route migration
- Smart Assistant create, edit, copy, list, search, enable, disable, delete, owner isolation, and incomplete-state handling
- Smart Assistant Provider Model and resource validation
- Current Assistant configuration on every new turn, per-turn execution evidence, Credit admission, cancellation, and durable conversation history
- FAQ CRUD, ordering, safe Markdown, import/export, direct-answer no-Credit behavior, and publication checks
- Answer Safety Policy, fixed localized refusal, classifier charging, minimum audit, and no sensitive-content leakage
- Version-bound Publication Validation, strict Origin/daily limit/data acknowledgement, fail-closed migration, non-executing preview, aggregate-only statistics, Token rotation and revocation, visitor isolation, owner Credit charging, and rate limits
- Image Creation administration and generation behavior, history, notifications, owner isolation, and settlement
- Knowledge source lifecycle, safety gating, permission-checked revision provenance, and fail-closed behavior while no Retrieval Provider is active
- Historical Assistant conversations remain readable after an Assistant is disabled or deleted
- No Assistant or Image Creation API leaks credentials, private content, provider responses, internal Object Keys, or signed URLs

Real provider, Runtime, retrieval, and external-channel claims remain unverified until their specific live or Conformance evidence is executed. A parser or configuration field is not proof that a provider capability works in production.
