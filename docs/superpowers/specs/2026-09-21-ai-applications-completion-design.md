# AI Applications Completion Design

**Date:** 2026-09-21

**Status:** Proposed for implementation

**Source requirements:** `docs/product/ai-applications-requirements.md`

## Goal

Complete the first-version AI Applications product area so that Smart Assistants, Digital Humans, Image Creation, knowledge retrieval, authenticated conversations, and anonymous iframe sharing satisfy the documented acceptance boundary without creating a parallel execution system.

## Current boundary

The repository already contains the initial AI Application domain, persistence, HTTP handlers, frontend routes, FAQ matching, basic knowledge search, image workbench, and public embed scaffold. The completion work must preserve the existing Agent Workspace contracts:

- Session and Workflow execution remain the source of truth for model admission, credits, cancellation, event ordering, rolling history, and secret redaction.
- AI Applications own configuration and snapshots, but do not own a second Runtime or model invocation queue.
- Knowledge Base permissions and retrieval remain separate from Smart Assistant configuration.
- Digital Human configuration is presentation metadata only and never becomes a provider credential or hidden prompt.
- User-owned application data remains owner-scoped; Administrator access is limited to explicitly administrator-owned configuration such as embedding settings and image models.

## Completion design

### 1. Canonical routes and product shell

Make `/ai-apps` the only canonical product surface with the three required entries. Keep `/ai-creation/image-generation` as a redirect to `/ai-apps/image-creation`, remove the duplicate route definition, and ensure route metadata keeps the active AI Applications navigation state. Add route tests for canonical paths, legacy redirects, unknown paths, and nested detail pages.

The landing shell remains a router-backed tab surface. It may show summary information, but child entities keep independent list, editor, lifecycle, ownership, and empty states.

### 2. Smart Assistant lifecycle

Extend the Smart Assistant domain with explicit validation and lifecycle operations:

- scenario classification uses the eight documented values while preserving `custom`;
- create, edit, copy, enable, disable, and delete are owner-scoped and version-checked;
- invalid or incomplete assistants remain editable but cannot start conversations;
- list supports owner-scoped name search and scenario filtering;
- delete requires an explicit confirmation at the HTTP/UI boundary and preserves historical session snapshots;
- FAQ CRUD includes edit, ordering, enable/disable publication checks, and safe Markdown rendering.

Copy creates a new independent assistant with copied visible configuration and FAQs, never copying share tokens, external conversations, session identity, or provider secrets. Enabling validates required references and published FAQ/document safety state before changing the lifecycle state.

### 3. Digital Human lifecycle

Add explicit enabled/disabled state and versioned lifecycle operations for Digital Humans:

- create, edit, copy, enable, disable, preview, and delete;
- a disabled human cannot be selected for a new assistant conversation;
- delete returns a conflict containing the referencing assistant identities unless the caller supplies an explicit detach-and-delete confirmation;
- copy excludes provider credentials and historical conversation identity;
- preview validates configuration without creating a Session, charging credits, or exposing secrets.

The first implementation treats preview as a deterministic configuration check because no live Digital Human provider has been selected in the requirements. Provider-specific live preview remains explicitly unverified.

### 4. Assistant conversation execution

Connect the existing assistant-session binding to the current Session execution path. A new assistant conversation creates a normal platform Session with an immutable assistant snapshot. The first accepted message freezes that snapshot and the current Personal Settings execution configuration. Later assistant edits affect only new conversations.

The execution path must use the existing Session admission and worker boundaries. It may prepend visible assistant configuration as structured execution context, but it must not create a second model client or bypass credits, cancellation, events, history, or redaction. The assistant pipeline is:

1. platform safety pre-check;
2. deterministic FAQ match;
3. bounded retrieval from the frozen Knowledge Selection;
4. grounded context passed through the existing model execution;
5. safe no-grounding response when no eligible source exists.

Direct FAQ answers do not invoke a model or consume credits. Model-backed answers do. The response snapshot records the assistant identity, resource revisions, retrieval citation metadata, and decision source without storing secrets or full sensitive safety input.

### 5. Knowledge ingestion and retrieval closure

Keep the existing `KnowledgeRepository` and embedding provider seams, but close the documented lifecycle:

- document acceptance, processing, ready, failed, and retry states;
- safety gating before indexing;
- asynchronous ingestion job and idempotent retry;
- PostgreSQL full-text fallback and pgvector search when configured;
- versioned embedding generations with atomic activation and previous-ready fallback;
- bounded citations containing base, document revision, source location, score, and safe display text;
- safe no-grounding response instead of returning raw chunks as the final answer.

The AI Application layer owns permissions, document metadata, revisions, assistant bindings, and citations. The retrieval implementation owns only indexing and recall.

### 6. Share and iframe closure

Complete the authenticated and anonymous share flows:

- server-side token validation and immediate revocation on disable/rotation;
- strict width/height and HTTPS allowed-origin validation;
- public visitor one-way hashing, separate External Conversations, owner credit charging, and token/visitor/IP rate limits;
- FAQ responses without credit consumption;
- model-backed external responses through the same Session worker path;
- conversation polling/history isolation and safe CSP/frame-ancestor headers;
- no private Session ID, Provider Model, Runtime, Knowledge Base internals, signed URL, object key, or credential in public payloads.

### 7. Image Creation placement

Preserve the existing image-generation data and credit contracts while completing the product rename and route redirect. Add route and UI regression coverage for text-to-image, image-to-image, prompt optimization, background progress, cancellation, regeneration, downloads, expiry, owner isolation, and credit settlement using the existing tests rather than introducing a new image subsystem.

## Verification strategy

Each phase adds focused tests before implementation changes:

- domain table tests for state transitions, validation, copy, ownership, and conflict behavior;
- repository tests for version checks, deletion references, snapshots, and migration behavior;
- HTTP tests for route/method/status/error contracts and secret-safe payloads;
- frontend tests for responsive navigation, forms, confirmation flows, empty/loading/error states, and keyboard-accessible actions;
- existing full backend tests, frontend tests, typecheck, production build, and deployment health checks;
- browser-to-API acceptance evidence for ordinary User and Administrator flows where applicable.

Claims about live Digital Human providers, real embedding providers, or production Runtime capabilities remain unverified until their specific live or Conformance checks run.

## Delivery order

1. Canonical routes, Smart Assistant lifecycle, and Digital Human lifecycle.
2. FAQ editing/publication and safe Markdown behavior.
3. Knowledge ingestion generations, citations, and no-grounding behavior.
4. Assistant Session execution and snapshot isolation.
5. Anonymous share execution and iframe acceptance.
6. Image Creation route/placement regression closure and full browser acceptance.

Each phase must leave the repository buildable and independently testable. The work must be integrated through `main_temp` before deployment and must not merge the ongoing `codex/connector-platform-p0` branch.
