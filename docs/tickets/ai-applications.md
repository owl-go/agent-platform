# AI Applications

Triage: `ready-for-agent`

## Problem Statement

Agent Workspace currently has a complete image-generation workbench under the legacy `AI Creation` area, but it has no unified home for durable user-owned AI applications. Users cannot create scenario-specific Smart Assistants, configure reusable Digital Humans, publish an Assistant through an iframe, define direct-answer FAQs, or constrain answers to an explicit safety and business scope.

The product also has no grounded-answer path for Assistant questions. A free-text question needs to prefer a reviewed FAQ answer, then retrieve bounded evidence from a selected Knowledge Base, and refuse safely when the question is sensitive, outside scope, or unsupported by available knowledge. The current codebase has no Assistant, Digital Human, FAQ, Share Configuration, External Conversation, or Knowledge Base retrieval entities, while existing Session, Expert, Credit, and image-generation seams should remain the platform boundaries.

## Solution

Add an `AI Applications` product area containing three entries:

1. `Smart Assistants` (`智能助手`): durable user-owned applications for a scenario such as customer consultation, pre-sales, after-sales, product guidance, enterprise knowledge, recruitment, or training.
2. `Digital Humans` (`数字人`): reusable presentation identities containing avatar, voice, language, expression, and scene settings. One Digital Human may be referenced by multiple Smart Assistants.
3. `Image Creation` (`图片创作`): the existing image-generation workbench moved under AI Applications. Existing Image Generation records, APIs, credits, and provider behavior remain valid; only the product area and frontend route are renamed.

Extend the existing Session pipeline rather than creating a second private conversation engine. An Assistant-created Session freezes the Assistant, Digital Human, Expert or Expert Team, Knowledge Selection, FAQ decision, Answer Safety Policy, and execution configuration. External iframe traffic uses an isolated anonymous `External Conversation` identity and never appears in the owner's private Session list.

Use the following answer pipeline for both authenticated Assistant conversations and external iframe conversations:

```text
Safety pre-check
  -> deterministic FAQ matching
  -> lightweight FAQ classifier when deterministic matching is uncertain
  -> direct FAQ answer when confidence reaches the configured threshold
  -> Knowledge Base retrieval when no FAQ matches
  -> grounded answer or fixed safe refusal
```

Direct FAQ answers do not invoke a model or consume Credits. Classifier and answer-model invocations consume the owning User's Credits. Sensitive or out-of-scope requests return the fixed localized refusal `暂时无法回答此类问题` and do not reveal the matched category or source text.

The first retrieval implementation supports text and Markdown documents using PostgreSQL full-text/keyword matching plus `pgvector`. The retrieval engine is hidden behind replaceable `EmbeddingProvider` and `RetrievalProvider` boundaries so a later Ragflow or other RAG adapter can replace indexing and recall without changing ownership, permissions, snapshots, citations, or Assistant behavior.

Smart Assistants may be shared anonymously through a per-Assistant Share Configuration. The configuration includes a revocable unpredictable Token, optional HTTPS allowed Origins, iframe width and height, generated embed code, owner-controlled free-text enablement and daily call limit, and platform-enforced IP, Visitor ID, Token, and concurrency limits. FAQ clicks remain free of model Credits; external model calls are charged to the Assistant owner.

## User Stories

1. As a User, I want an AI Applications entry in the main navigation, so that Smart Assistants, Digital Humans, and Image Creation have one predictable home.
2. As a User, I want separate Smart Assistant, Digital Human, and Image Creation tabs, so that durable applications and task-specific creation tools remain understandable.
3. As a User, I want to create multiple Smart Assistants, so that each business scenario has its own configuration and history.
4. As a User, I want to name and describe a Smart Assistant, so that I can recognize its purpose from the catalog.
5. As a User, I want to classify an Assistant as customer consultation, pre-sales, after-sales, product guidance, enterprise knowledge, recruitment, training, or custom, so that discovery does not depend on the entity being called customer service.
6. As a User, I want to define an Assistant service goal, so that it has a clear purpose beyond a generic model prompt.
7. As a User, I want to define visible operating rules and response style, so that the Assistant's behavior is reviewable and predictable.
8. As a User, I want to bind an Expert or Expert Team optionally, so that a scenario can reuse existing specialist guidance without requiring it.
9. As a User, I want to bind one or more Knowledge Bases, so that the Assistant can answer from selected sources.
10. As a User, I want to bind an optional Digital Human, so that I can choose a visual and voice presentation without coupling it to the Assistant's knowledge.
11. As a User, I want incomplete Assistants to remain editable but unavailable for new conversations, so that incomplete setup cannot start execution.
12. As a User, I want to create, edit, copy, enable, disable, and delete an Assistant, so that I can manage its lifecycle.
13. As a User, I want to search and filter my Assistants, so that multiple applications remain manageable.
14. As a User, I want to start a Session from an Assistant, so that existing conversation streaming, attachments, cancellation, retry, Rolling Summary, and Credits continue to work.
15. As a User, I want an Assistant-created Session to freeze the Assistant configuration, so that later edits do not change historical conversations.
16. As a User, I want historical conversations to retain the Assistant and Digital Human identity, so that old responses remain understandable after deletion or disabling.
17. As a User, I want to create multiple Digital Humans, so that different scenarios can have different presentation identities.
18. As a User, I want to edit an avatar, voice, language, expression style, and scene settings, so that the Digital Human can represent a chosen role.
19. As a User, I want to preview a Digital Human configuration without creating a production conversation, so that I can validate presentation settings safely.
20. As a User, I want to reuse one Digital Human across multiple Assistants, so that I do not duplicate shared identity settings.
21. As a User, I want deletion of a referenced Digital Human rejected with the referencing Assistants listed, so that one deletion cannot silently change several applications.
22. As a User, I want to create, edit, reorder, enable, disable, and delete FAQ entries, so that common questions have stable reviewed answers.
23. As a User, I want an FAQ answer to support safe Markdown, so that formatted content is readable without allowing executable HTML or scripts.
24. As a User, I want a direct FAQ click to return the stored answer immediately, so that standard answers do not invoke a model or consume Credits.
25. As a User, I want free text that clearly matches an FAQ to receive the same stored answer, so that wording variations still get a reviewed response.
26. As a User, I want uncertain FAQ matching to use a lightweight classifier and confidence threshold, so that weak matches do not return the wrong canned answer.
27. As a User, I want a non-FAQ question to search only the Assistant's selected Knowledge Bases, so that answers stay within the configured source scope.
28. As a User, I want grounded answers to include safe document citations, so that I can understand where an answer came from.
29. As a User, I want a question without sufficient evidence to receive a safe no-grounding response, so that the Assistant does not invent facts.
30. As a User, I want to define the Assistant's allowed business scope, so that it does not present itself as a general-purpose authority.
31. As a User, I want to add narrower custom blocked topics, so that the Assistant can follow business-specific restrictions.
32. As a User, I want platform safety categories to remain non-overridable, so that Assistant configuration cannot weaken political, military, weapon, violence, dangerous-operation, or security-bypass protections.
33. As a User, I want sensitive or out-of-scope questions to return exactly the fixed refusal, so that the product does not expose moderation details or sensitive source text.
34. As a User, I want FAQ answers checked before enablement, so that the Assistant cannot publish a prohibited canned answer.
35. As a User, I want Knowledge Documents checked before indexing, so that blocked source content never participates in retrieval.
36. As an Administrator, I want safety policy revisions to disable affected FAQ entries or remove affected retrieval sources, so that a policy update takes effect without deleting audit metadata.
37. As a User, I want direct rule checks to be free and classifier checks to use the normal Credit contract, so that accounting reflects actual model work.
38. As a User, I want to enable public sharing for one Assistant without sharing every Assistant, so that exposure is an explicit per-application decision.
39. As a User, I want sharing to generate an unpredictable Token, so that public access cannot be guessed from an Assistant ID.
40. As a User, I want to rotate or revoke a Share Token, so that old embedded copies stop working immediately.
41. As a User, I want to restrict embedding to an HTTPS Origin list, so that I can control where my Assistant appears.
42. As a User, I want to set iframe width and height, so that the generated embed fits my website layout.
43. As a User, I want width to support `100%` or bounded pixels and height to support bounded pixels, so that embeds remain usable and safe.
44. As a User, I want generated iframe code to include my current dimensions, so that I can paste a complete integration snippet.
45. As a User, I want public visitors to see FAQ buttons and free-text chat, so that the embed supports both fast answers and follow-up questions.
46. As an external visitor, I want to use the iframe anonymously, so that I do not need an Agent Workspace account to ask a public Assistant.
47. As an external visitor, I want a short-lived browser conversation, so that refreshes can preserve context without exposing another visitor's history.
48. As a User, I want external conversations separated from my private Session list, so that public traffic cannot mix with personal work.
49. As a User, I want external model calls charged to my Credits, so that sharing has explicit ownership and cost semantics.
50. As a User, I want to disable external free-text questions or set a daily call limit, so that public sharing cannot consume unlimited Credits.
51. As a User, I want FAQ clicks to remain free of model Credits even in an iframe, so that common answers are inexpensive to share.
52. As a Platform Administrator, I want Token, IP, Visitor ID, concurrency, and request-rate limits, so that public embeds cannot be abused.
53. As a User, I want public UI responses to omit Provider Models, Runtime Engines, Experts, Knowledge Base internals, credentials, internal IDs, Object Keys, and signed URLs, so that sharing does not leak private implementation details.
54. As a User, I want Image Creation nested under AI Applications, so that the product-area rename does not make existing image records disappear.
55. As a User, I want the old image-generation route to redirect to the new Image Creation route, so that existing bookmarks continue to work.
56. As a User, I want existing Image Generation records, Image Models, Prompt Optimization, SSE progress, cancellation, regeneration, history, and Image Credit settlement unchanged, so that the rename does not change image behavior.
57. As a User, I want to upload text and Markdown Knowledge Documents, so that the first retrieval release has a small, reliable source boundary.
58. As a User, I want document processing states Accepted, Processing, Ready, and Failed, so that I know when a document can participate in retrieval.
59. As a User, I want a failed document to retain its source and support retry, so that transient ingestion failures do not require re-uploading.
60. As a Platform Administrator, I want embedding configuration to be protected and versioned, so that vector rebuilds are reproducible and credentials remain private.
61. As a Platform Administrator, I want PostgreSQL full-text and pgvector retrieval in the first release, so that the system can run locally without a separate hosted RAG service.
62. As a Platform Administrator, I want a replaceable retrieval seam, so that Ragflow or another RAG provider can replace indexing and recall later.
63. As a User, I want embedding generation changes to create a new index generation, so that the current Ready index remains live during rebuilds.
64. As a User, I want failed vector rebuilds to leave the previous index active, so that retrieval does not become unavailable during migration.
65. As a User, I want citations to retain Knowledge Base, Document Revision, source location, relevance, and safe display text, so that grounded answers are auditable without exposing storage internals.

## Implementation Decisions

- The product surface is `AI Applications`; its child entries are Smart Assistants, Digital Humans, and Image Creation.
- Smart Assistant and Digital Human are separate User-owned aggregates. A Digital Human is reusable by multiple Assistants and owns no business knowledge or answer logic.
- Smart Assistant conversations reuse the existing Session execution and message pipeline. A Session stores the Assistant identity and freezes Assistant, Digital Human, Expert, Knowledge Selection, FAQ, safety, and execution configuration at its first accepted message.
- External iframe traffic uses an External Conversation identity and never enters the owner's private Session list. It reuses the existing message and model execution pipeline after share, rate, safety, FAQ, and retrieval checks.
- Smart Assistant configuration includes visible service goal, operating rules, response style, scenario type, FAQ entries, Answer Safety Policy, optional resource bindings, and optional Share Configuration.
- FAQ direct answers are stored Markdown responses and do not invoke a model or consume Credits. Free-text FAQ classification may invoke a lightweight classifier and follows the normal owner Credit contract.
- Safety is enforced before FAQ matching, Knowledge Base retrieval, or answer generation. Platform safety categories are non-overridable; Assistant owners can only narrow scope.
- FAQ and Knowledge Document safety checks run before enablement/indexing. A policy revision can disable an FAQ or remove a source from the active retrieval generation without deleting audit metadata.
- Public sharing uses an unpredictable revocable Token, optional HTTPS Origin allowlist, bounded width/height, generated iframe code, anonymous Visitor IDs, owner Credit charging, and platform-wide abuse limits.
- Image Creation changes the user-facing product area and frontend route while preserving existing Image Generation APIs, records, provider adapters, storage, Credits, and backend compatibility.
- Knowledge retrieval supports text and Markdown document revisions, asynchronous processing states, PostgreSQL full-text/keyword retrieval, PostgreSQL `pgvector`, versioned Embedding Providers, and a replaceable Retrieval Provider.
- The platform owns Knowledge Base permissions, document revisions, chunk provenance, Assistant bindings, and Knowledge Citations. Retrieval providers own only indexing and recall.
- Embedding index generations activate atomically after a complete rebuild. A failed rebuild leaves the previous Ready generation active.
- Local PostgreSQL and pgvector are sufficient for the first deployment; hosted RAG services are not required.
- Digital Human providers, live voice/video, external channels other than iframe, public application marketplace, human-agent queues, and advanced analytics remain outside this implementation.

## Testing Decisions

- Tests verify externally observable behavior at domain, application, repository, HTTP, and browser seams; they should not assert private implementation details such as SQL query shape or classifier prompt wording.
- Smart Assistant domain tests cover required fields, Draft/Enabled/Disabled transitions, FAQ ordering and validation, safety policy invariants, Share Configuration width/height bounds, Token rotation, and Digital Human reference conflicts.
- Repository tests cover owner isolation, multiple Assistants and Digital Humans per User, Digital Human reuse, Assistant snapshots, FAQ persistence, Share Token revocation, External Conversation separation, document revision states, vector generation activation, and failed rebuild rollback.
- Application and HTTP tests cover FAQ direct-answer no-Credit behavior, classifier charging, safety refusal, Knowledge Base retrieval, grounded citations, no-grounding refusal, public share authorization, allowed Origins, iframe dimensions, anonymous Visitor IDs, owner Credit charging, rate limits, and private Session isolation.
- Existing Session tests are extended to verify Assistant-bound Session creation, frozen Assistant snapshots, later-edit isolation, and preserved cancellation/retry/streaming behavior.
- Existing image-generation tests are updated for the new frontend route and label while retaining old backend API compatibility and old-route redirect coverage.
- Frontend tests cover AI Applications navigation, Assistant and Digital Human lists/editors, FAQ interactions, share snippet generation, iframe dimension validation, safety refusal rendering, citation rendering, Image Creation route migration, and mobile layout.
- PostgreSQL integration tests run the actual full-text and pgvector retrieval path when the local database extension is available; otherwise unit tests cover provider contract behavior and report the missing integration environment rather than treating it as conformance.
- Security tests assert that Access Tokens, Provider API Keys, internal Session IDs, Object Keys, signed URLs, raw sensitive prompts, and raw provider responses never appear in public iframe responses, ordinary logs, or artifacts.

## Out of Scope

- WeChat, DingTalk, Feishu, or other external channel adapters beyond authenticated Web and iframe embedding
- Public application marketplace, cross-User sharing, or platform templates
- Live Digital Human voice/video providers, streaming broadcasts, custom model training, or user-uploaded model weights
- Human-agent queues, tickets, SLA rules, seat management, or automatic transfer to a human
- Automatic FAQ generation, bulk FAQ import, semantic FAQ authoring UI, or FAQ answer rewriting
- PDF, web capture, OCR, or other Knowledge Document sources beyond text and Markdown
- Hosted Ragflow integration in the first release; the Retrieval Provider seam is required, but the first provider is local PostgreSQL and pgvector
- Assistant-level independent Provider Model or Runtime Engine policies
- Detailed product analytics beyond basic share usage, safety audit, and existing conversation/execution history

## Further Notes

- The authoritative product terms are `AI Applications`, `Smart Assistant`, `Digital Human`, `Image Creation`, `Smart Assistant FAQ`, `Answer Safety Policy`, `Share Configuration`, `External Conversation`, and `Retrieval Provider`.
- The fixed refusal message is intentionally generic: `暂时无法回答此类问题` in Chinese and `I'm unable to answer that type of question at the moment.` in English.
- The existing lower-level Image Generation terms remain authoritative for persistence and provider behavior even though the navigation label becomes Image Creation.
- The first implementation should preserve the highest existing seams: Session execution for conversations, Credits for model admission and settlement, object storage for protected assets, PostgreSQL for ownership and snapshots, and a provider boundary for retrieval.
- This specification is stored locally because the request explicitly allowed local publication instead of creating a remote issue.
