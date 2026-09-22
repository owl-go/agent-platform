# AI Applications Completion Design

**Date:** 2026-09-21

**Status:** Proposed for implementation

**Source requirements:** `docs/product/ai-applications-requirements.md`

## Goal

Complete the first-version AI Applications area so Smart Assistants, Digital Humans, Image Creation, knowledge retrieval, authenticated conversations, and anonymous iframe sharing satisfy the documented acceptance boundary without creating a parallel Runtime or model-invocation queue.

## Design constraints

- Existing Session and Workflow execution remain the source of truth for model admission, credits, cancellation, event ordering, history, and redaction.
- AI Applications own configuration and immutable conversation snapshots, not a second model client or worker queue.
- Knowledge Base permissions and retrieval remain separate from Smart Assistant configuration.
- Digital Human configuration is presentation metadata only; provider credentials never enter ordinary payloads, snapshots, logs, or artifacts.
- User-owned application data remains owner-scoped; the Administrator manages only explicitly administrator-owned configuration such as embedding settings and image models.

## Completion design

### Canonical product shell

`/ai-apps` is the canonical surface with Smart Assistants, Digital Humans, and Image Creation entries. `/ai-creation/image-generation` redirects to `/ai-apps/image-creation`; duplicate route definitions are removed. Route tests cover canonical paths, redirects, nested detail pages, and unknown paths.

### Smart Assistant lifecycle

Add documented scenario values and validation; owner-scoped, version-checked create, edit, copy, enable, disable, and delete operations; name/scenario list filtering; incomplete-state handling; explicit delete confirmation; FAQ edit/order/publication operations; and safe Markdown rendering. Copy duplicates visible configuration and FAQs but never share tokens, sessions, external conversations, or secrets. Enabling validates referenced resources and published safety state.

### Digital Human lifecycle

Add enabled/disabled state and versioned create, edit, copy, enable, disable, preview, and delete operations. Disabled humans cannot be selected for new conversations. Delete returns an explicit reference conflict unless detach-and-delete is confirmed. Copy excludes credentials and historical identity. Preview is a deterministic configuration check until a real provider is selected.

### Assistant execution

Connect assistant-session bindings to the existing Session path. A new assistant conversation creates a normal Session and freezes the assistant snapshot and current Personal Settings on the first accepted message. The ordered pipeline is safety pre-check, deterministic FAQ, bounded retrieval, grounded context through the existing model execution, or a safe no-grounding response. FAQ answers do not consume credits; model-backed answers do. Snapshots retain identity, resource revisions, retrieval citations, and decision source without sensitive input or secrets.

### Knowledge and sharing

Complete accepted/processing/ready/failed/retry ingestion, safety gating, asynchronous idempotent jobs, FTS/pgvector fallback, versioned embedding generations with atomic activation, bounded citations, and safe no-grounding behavior. Complete public token revocation, origin and size validation, visitor/IP/token limits, external conversation isolation, owner credit charging, polling, CSP, and model-backed iframe responses through the same Session worker path.

### Image Creation

Preserve existing image generation, credit, cancellation, history, and ownership contracts while completing the product rename, legacy redirect, and browser regression coverage.

## Delivery order

1. Routes and Smart Assistant/Digital Human lifecycle.
2. FAQ editing/publication and safe Markdown.
3. Knowledge ingestion generations, citations, and no-grounding behavior.
4. Assistant Session execution and snapshot isolation.
5. Anonymous sharing and iframe execution.
6. Image Creation placement and full browser acceptance.

Every phase must leave the repository buildable and tested, integrate through `main_temp`, and must not merge `codex/connector-platform-p0`.
