# Domain Docs

The root [CONTEXT.md](../../CONTEXT.md) is the canonical glossary for Agent Workspace. This document routes domain work to the current code and authoritative behavior documents; package names, storage records, and transport DTOs are not automatically new domain concepts.

## Required reading

- Read the root glossary before naming or changing domain concepts. Keep its definitions about meaning, ownership, and relationships; provider protocols, fields, migrations, and deployment status belong in technical documents or dated evidence.
- Read the relevant ADRs under `docs/adr/` before changing an established architectural decision.
- Follow the product and technical document routing in [AGENTS.md](../../AGENTS.md); a target capability in a specification is not evidence that the capability is implemented. Inspect the Application callers and adjacent tests as well as the Domain types.

## Code and behavior routes

These routes were checked against the code on 2026-10-04. They establish where to inspect behavior, not production acceptance.

| Domain concern | Code entry | Authoritative behavior |
|---|---|---|
| User, Administrator, Identity Group and ownership | [Account Domain](../../backend/internal/biz/account/domain/account.go) | [Product ownership and governance](../product/agent-workspace-requirements.md), [ADR 0040: identity groups](../adr/0040-read-only-enterprise-groups-and-department-resources.md) |
| Credits, reservation and settlement | [Credits Domain](../../backend/internal/biz/credits/domain/model.go), [execution snapshots](../../backend/internal/biz/workspace/domain/execution.go) | Product Credits rules and [service architecture](../technical/service-architecture.md) |
| Session, Workflow, Run, Attachment, Artifact and Execution Activity | [Workspace Domain](../../backend/internal/biz/workspace/domain/model.go), [Conversation Selection](../../backend/internal/biz/workspace/domain/conversation.go), [Execution Plan](../../backend/internal/biz/workspace/domain/execution_plan.go) | Product conversation/execution rules and [Runtime contract](../technical/runtime-adapter.md) |
| Message Channel Account, Audience, Conversation and Delivery | [Message Channel Domain](../../backend/internal/biz/workspace/domain/message_channel.go), [Application admission](../../backend/internal/biz/workspace/application/message_channels.go) | [Message channel design](../technical/workflow-message-channels.md) |
| Message Channel Sender Pairing, Validation, Inbox and Response | [Sender Pairing](../../backend/internal/biz/workspace/application/message_channel_sender_pairing.go), [Response lifecycle](../../backend/internal/biz/workspace/application/message_channel_response.go), [Inbox/Delivery persistence](../../backend/internal/data/workspace/gormrepo/message_channels.go), [public preview](../../backend/internal/data/workspace/runtimeexecutor/channel_response_sink.go) | Message channel design; temporary pairing and optional transport capabilities are not independent User-owned aggregates |
| Knowledge Selection, Revision, Ingestion, Generation and Citation | Workspace Domain, [permission-checked retrieval](../../backend/internal/knowledgebase/retrieval/search.go), [provider wiring](../../backend/internal/wiring/knowledgebase/providers.go) | [Knowledge design](../technical/knowledge-base-rag.md), [ADR 0042](../adr/0042-ragflow-knowledge-retrieval.md) |
| Expert, Expert Team and frozen execution identity | Workspace Domain and execution snapshots | Product specialist rules and [ADR 0037: execution defaults](../adr/0037-refresh-execution-defaults-for-conversation-turns.md) |
| Connector Package, Revision, Publication, Installation and Authorization | [Connector Domain](../../backend/internal/biz/workspace/domain/connector_package.go), [command approval](../../backend/internal/biz/workspace/domain/approval.go) | [Connector design](../technical/connector-platform.md), [ADR 0036](../adr/0036-connector-packages-and-unified-lifecycle.md), [ADR 0039](../adr/0039-separate-connector-authorization-protocol-from-lifecycle.md) |
| Smart Assistant, FAQ, safety, publication and conversations | [AI Application Domain](../../backend/internal/biz/aiapplication/domain/model.go), [Assistant Conversation](../../backend/internal/biz/aiapplication/domain/conversation.go), [External Conversation](../../backend/internal/biz/aiapplication/domain/external.go) | [AI Applications requirements](../product/ai-applications-requirements.md), [ADR 0037: Assistant conversations](../adr/0037-direct-smart-assistant-conversations.md) |
| Image Creation and Generated Image | [AI Creation Domain](../../backend/internal/biz/aicreation/domain/model.go) and Credits Domain | [Image generation design](../technical/image-generation.md) |

## Vocabulary

Use glossary terms exactly as defined in `CONTEXT.md`. Use Connector for the external capability; Extension is a legacy code/UI name, not a separate domain concept. Distinguish a Message Channel Account from a permitted sender, Message Channel Sender Pairing from Message Channel Validation, Message Channel Response from Delivery, and Connector Publication from Installation and Authorization.

## Relationship checks

Use concrete scenarios when changing these boundaries:

- A sender has been recognized by a pairing code, but the owner has not saved the Audience. Recognition must not grant access; a connected account still needs receive-and-reply validation before channel enablement.
- A Run succeeded, but its external reply has an uncertain send outcome. Delivery recovery must not rerun the Workflow or silently assume that no message was sent; dynamic progress is not the terminal result.
- A queued Run freezes a Knowledge Index Generation, then a document is replaced or access is revoked. Retained revisions preserve the frozen content choice, while current source permissions still govern its use. RAGFlow is the current optional provider implementation, not the owner of those permissions.
- A Connector Publication changes revision while a User already has an Installation and a frozen execution snapshot. Publication, Installation, Authorization, and the snapshot have separate identities and lifecycles; catalog availability does not authorize an external account.

Inspect the corresponding [Audience/isolation tests](../../backend/internal/biz/workspace/domain/message_channel_test.go), [pairing tests](../../backend/internal/biz/workspace/application/message_channel_sender_pairing_test.go), [response tests](../../backend/internal/biz/workspace/application/message_channel_response_test.go), [Connector lifecycle tests](../../backend/internal/biz/workspace/domain/connector_package_test.go), and [Knowledge generation integration tests](../../backend/internal/data/workspace/gormrepo/knowledge_provider_integration_test.go). This documentation update did not execute those tests or create new live acceptance evidence.

## Decisions

If a proposed change contradicts an accepted ADR, call out the conflict explicitly and create a superseding ADR rather than silently diverging.
