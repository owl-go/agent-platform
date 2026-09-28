---
status: superseded by ADR-0037
---

# Use platform-owned Assistant sessions with layered safety

Smart Assistants reuse the existing Session execution and message pipeline while recording the Assistant identity, resource revisions, FAQ decision, and Answer Safety Policy in the conversation snapshot. Direct FAQ selections return a safety-checked stored answer without a model invocation; free text passes through safety enforcement, FAQ classification, and grounded retrieval before normal model execution. Public iframe traffic uses a separate anonymous External Conversation identity rather than entering the owner's private Session list, while the Assistant owner remains responsible for model Credits and platform rate limits protect anonymous access.

The safety boundary is deliberately outside the prompt: platform categories cannot be disabled, Assistant owners can only narrow their service scope, sensitive requests receive a fixed localized refusal, and FAQ/Knowledge Document content is checked before publication or indexing. This keeps safety, billing, and private-session isolation enforceable even when the model or retrieval provider changes.
