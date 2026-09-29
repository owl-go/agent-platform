# AI Applications

Triage: `implemented`

## Outcome

Agent Workspace exposes `AI Applications` as the shared product area for:

1. `Smart Assistants` (`智能助手`), which are durable User-owned applications for constrained scenarios.
2. `Image Creation` (`图片创作`), which is the existing image-generation workbench under the AI Applications navigation.

Smart Assistants provide editable prompts, a validated `openai_chat` Provider Model, optional Expert or Knowledge Base bindings, ordered FAQs, answer-safety enforcement, durable Assistant Conversations, and optional anonymous iframe sharing. Image Creation retains its independent generation records, provider behavior, storage, Credits, and history.

## Execution Decisions

- Authenticated Assistant conversations use the dedicated Assistant Conversation model and remain separate from Workspace Sessions.
- Each conversation freezes the Assistant and selected Provider Model configuration needed for later execution and audit.
- FAQ direct answers do not invoke a model or consume Credits.
- Free text passes through safety checks, FAQ and scope classification, optional Knowledge Base retrieval, then model generation.
- Public iframe traffic uses visitor-scoped conversations and never appears in the owner's authenticated conversation history.
- Share Tokens are unpredictable and revocable; public access is protected by Origin, visitor, IP, Token, concurrency, and daily-limit controls.
- Image Creation keeps the `/ai-apps/image-creation` route while the deployed legacy route redirects to it.
- Knowledge retrieval is fail closed while no verified Retrieval Provider is active.

## Acceptance Evidence

Tests must cover navigation, Smart Assistant lifecycle and owner isolation, model validation, conversation snapshots, FAQ behavior, safety refusal, sharing controls, Credit accounting, Image Creation route compatibility, and absence of secrets or internal storage identifiers in public responses.

The detailed current requirements are in `docs/product/ai-applications-requirements.md`. Real provider, retrieval, Runtime, and external-channel behavior is not considered production-verified without its required live or Conformance evidence.
