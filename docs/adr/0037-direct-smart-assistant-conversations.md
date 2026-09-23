---
status: accepted
---

# Use one direct model pipeline for private and new public Smart Assistant conversations

Smart Assistant conversations select an available `openai_chat` Provider Model on the Assistant, freeze its connection snapshot when a conversation starts, and run the same preprocessing, FAQ, Knowledge Base, streaming, Credit, and audit logic without a Runtime Engine. New public conversations use that pipeline too, but are scoped to a Visitor hash and Share Token revision and excluded from the owner's private history; older external Session conversations retain their original execution snapshot for continuity. This supersedes the Session-reuse decision in ADR-0034: keeping public questions on a separate Session path would silently ignore the Assistant's model choice and diverge from its answer policy.
