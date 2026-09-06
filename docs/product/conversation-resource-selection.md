# Conversation Resource Selection

Status: implemented with local validation. Production and exact-image conformance remain separate evidence.

## Confirmed Scope

- Skill catalog cards expose `Go use` on hover. Clicking that action opens a new Session with the Skill selected, without sending a message. Clicking the card opens read-only details whose body is the package's `SKILL.md` content.
- Expert and Expert Team cards expose `Summon` on hover. Clicking that action opens a new Session with that Expert or Expert Team selected, without sending a message. Clicking the card opens read-only details, with edit and summon actions.
- Details use a side panel on desktop and a full-screen presentation on mobile. Expert details include visible structured guidance and associated Skills and Connectors; Expert Team details include its profile and ordered member roles.
- The Session composer and the existing follow-up composer in a Workflow Run Conversation expose a bottom `+` menu containing Add file, Experts, Skills, and Connectors. Workflow goal editing, initial manual execution, Scheduled Triggers, and API trigger inputs are outside this composer change.
- The composer supports `/` to select Skills and `@` to reference files. Explicitly selected Skills appear as removable tokens in the message body; the selected Expert or Expert Team and Connector icons appear in the bottom area.
- One Expert, one Expert Team, or no specialist may be selected. The User may switch during a conversation; the selection applies to the next sent message and remains selected for subsequent messages.
- Explicitly selected Skills apply to the message being sent and clear for the next message. Selected Connectors remain selected until changed. Skills and Connectors may be used without an Expert.
- A composer Connector switch controls selection for the current conversation. Account-level Connector Enablement and Authorization remain separate management actions.
- Each sent message preserves its actual specialist and resource configuration. Later selection changes do not change historical execution or its retry configuration. Personal Settings execution configuration remains frozen for the Session or Run Conversation.
- File suggestions include the current conversation's uploaded attachments and available generated Artifacts. Workflow follow-ups additionally expose files from that Workflow's Workspace in a clearly labeled group. Unavailable files show a reason. A selected reference must make the actual file available to that execution, rather than merely inserting its name.

## Resource Selection

- An Expert's own Skills remain available while that Expert is selected. Clearing explicit Skill tokens after a message does not clear Expert-owned Skills. Deduplicate the same Skill identity within an Execution Stage.
- An Expert's own Connectors are selected by default. The User may disable them for the conversation without editing the reusable Expert. Switching specialists replaces specialist-derived defaults while retaining the User's explicit Connector additions and exclusions; an explicit exclusion takes precedence over an inherited default.
- Explicitly added Skills and Connectors apply to every Team Member in the current turn. Each member otherwise receives only its own Expert's default resources; another member's default resources are not implicitly shared. The composer has no member-by-member assignment controls.
- A retained selection keeps its exact Expert, Expert Team, Skill, and Connector revisions. Editing a catalog resource does not silently refresh it in an existing selection. Explicit reselection resolves the latest available revision; a newly selected Skill token similarly resolves its current available revision.
- Current Connector Enablement and Authorization remain enforceable despite revision retention. An unavailable selection displays a recovery action and is never silently omitted from execution. Existing fail-closed tests, image compatibility, and command-approval requirements continue to apply.
- Workflow follow-up selections belong to their Run Conversation. They do not edit the Workflow definition or the defaults used by future manual, scheduled, or API triggers.

## Submission And Drafts

- After the backend accepts a message, clear its text, explicit Skill tokens, and current file selections for the next message. Retain the specialist and Connector choices.
- An upload or submission failure preserves the entire draft and its selections. A later execution failure or cancellation leaves the accepted input in conversation history and does not automatically refill the composer.
- Retry uses the historical execution's original input, resource revisions, and file contents rather than the current draft. This does not change where existing Session retry or Workflow rerun actions create their result.
- Keep a separate Conversation Draft for each Session and Run Conversation in the current browser, scoped to the owning User. Reloading or returning restores text, selections, and uploaded or historical file references. Local files that have not been uploaded require reselection after reload, with a visible explanation.
- Catalog launch actions create a new Session and preserve the preceding conversation's draft. They do not transplant an existing draft into the new Session or automatically send its content.

## File References

- The `@` picker lists attachments and available Artifacts belonging to the current conversation. Workflow follow-ups additionally list that Workflow's current Workspace files in a separate group. Ownership and conversation/Workflow scope must be validated on the server.
- An accepted message fixes the referenced bytes at submission time. Workspace edits while a Run waits in the queue do not alter that message's File References. This fixes the referenced copies, not the entire live Workflow Workspace.
- Save the referenced content with the message so expiry of the original Artifact does not invalidate an already accepted reference. Subsequent cleanup follows the owning conversation's data lifecycle; a signed download URL is never the persisted identity.
- New attachments and referenced files together are limited to ten distinct files per message and 100 MiB per file. Repeated mentions of the same file count once. Display enough source/path information to distinguish different files sharing a name.
- Reject submission with a specific recovery action if a selected file is already unavailable or outside its permitted scope. Preserve the draft. Materialization verifies immutable size and checksum and supplies read-only copies of the accepted references to execution.

## Interaction Details

- In both composers, toolbar icon buttons keep equal width and height on desktop and mobile. Send and stop each show one centered icon; a loading indicator replaces that icon while the action is pending without changing the button's shape.
- Use the exact Chinese catalog actions from the request: Skill `去使用`, Expert and Expert Team `召唤`. Card activation opens details; action-button activation launches a Session without also opening details. Actions remain available to keyboard and touch users without hover.
- `+` and `/` select from the same available Skill catalog. Multiple distinct Skill tokens and file references may appear at the cursor in the same message; tokens are individually removable without rewriting surrounding text.
- Open `/` and `@` suggestions at a token boundary, filter by the typed query, and allow arrow navigation, Enter to select, and Escape to dismiss. A selection-confirming Enter does not also send the message; IME composition never triggers send. URLs and email addresses typed as ordinary text remain intact.
- Expert and Expert Team entries are grouped in one specialist picker with an explicit no-specialist option. The selected specialist appears by icon and name at the bottom. Connector icons open a named selection switch and management entry as shown in the reference; authorizations remain in the existing management flow.
- Catalog Skill details render the installed revision's `SKILL.md` as content, not instructions to the platform. Links and Markdown must be rendered safely; opening details does not execute scripts or install dependencies.
- Historical execution remains associated with its actual specialist, resource revisions, and accepted files even after the next-message choices change. Changing or removing resources must not leave them executable through a reused Runtime context or native Resume.

## Revision Precedence

An Expert may retain Skill revision v1 while the User explicitly selects revision v2 of the same Skill. The explicit selection replaces the inherited revision for that turn, leaving one revision per resource identity per Execution Stage. Clearing the explicit Skill token restores the Expert's retained revision for subsequent messages. Apply the same explicit-over-inherited rule to Connector revisions while retaining explicit exclusions and current authorization checks.

## Acceptance Scenarios

- Catalog card details and action buttons are distinct on desktop, keyboard, and mobile; launch preselects the requested resource in a new Session and preserves the prior draft.
- Anonymous, Expert, and Expert Team executions receive the selected resources. Switching specialists affects the next message only, retains explicit Connector overrides, and preserves historical retries.
- Expert-owned Skills remain after explicit Skill tokens clear. Team defaults remain member-specific, while explicit resources reach every member. Explicit revisions override inherited revisions without changing the retained defaults.
- Catalog edits do not change retained revisions; explicit reselection refreshes them. Disabled Connectors and revoked authorizations remain unusable even through historical snapshots, warm containers, or native Resume.
- Rejected submissions preserve drafts. Accepted submissions clear turn-specific input; later failure or cancellation does not overwrite a new draft. Browser reload and navigation restore the correct User/conversation draft without mixing conversations.
- `/`, `@`, mouse, keyboard, touch, token removal, ordinary URL/email text, and Chinese IME composition work in both composers.
- Referenced attachments, Artifacts, and Workspace files reach execution as the accepted bytes. Cover another User's IDs, another conversation's files, unsafe paths, source expiry, queued Workspace changes, limits, duplicate references, and checksum mismatch.
- Workflow follow-ups preserve the initiating goal, environment, execution configuration, and success-only Workspace merge. Their selection changes do not leak into the Workflow definition or future trigger conversations.

Implementation validation must exercise the authenticated HTTP and Runtime-executor seams with injected dependencies, focused Vue page/composer behavior tests, and the affected existing regression suites. Run target Go tests, `make test`, `make build`, `make web-typecheck`, and `make web-build` as appropriate to the eventual implementation. Exact-image Linux/runsc conformance remains separate evidence; passing local tests cannot stand in for it.

## Current Evidence

The catalog launch/detail paths and shared Vue composer are implemented for Sessions and Workflow Run Conversation follow-ups. Backend selection revisions are stored by owner and conversation, each submission freezes its actual Stage resources, and accepted submissions retain only persistent selection. A temporary PostgreSQL 17 database exercises the complete migration chain, revision retention/reselection, scope rejection, accepted-message retention, historical retry, platform-history continuity, unavailable-specialist recovery, and Workflow follow-up isolation.

Focused tests cover per-member resource precedence and exclusion, authenticated selection HTTP metadata without secret/configuration exposure, installed Skill Markdown checksums and safe rendering, scoped immutable Artifact and Workspace references, source expiry, traversal/symlink rejection, draft isolation/reload, token removal, accepted/failed sends, slash selection, and Chinese IME. Runtime executor tests verify resource changes do not reuse the same warm state identity. Validation on 2026-09-06 passed `make generate`, `make test` with the temporary PostgreSQL DSN, `make build`, `make web-typecheck`, `make web-build`, and all 122 frontend tests across 19 files. Remote Object Storage tests that require credentials are not counted as integration evidence.

Desktop and 390px mobile browser checks use the real Vue components with injected fixture APIs, so they verify layout and interaction rather than deployed authentication or real Runtime execution. Dynamic selections deliberately use platform history and disable native Resume; this avoids reusing removed tools or guidance while exact-image changing-resource Resume remains unverified. No production deployment, production migration, remote Object Storage integration, or Linux + runsc conformance is asserted by these local checks.
