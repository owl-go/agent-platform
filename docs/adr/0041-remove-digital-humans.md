# ADR 0041: Remove Digital Humans From AI Applications

Status: Accepted

Date: 2026-09-28

## Context

AI Applications previously included a Digital Human aggregate, CRUD and preview APIs, dedicated frontend routes, and an optional Smart Assistant reference. The feature described presentation configuration only; no verified live avatar, voice, or video provider completed the product loop. Keeping the dormant aggregate increased navigation, API, persistence, snapshot, and Assistant validation complexity without contributing to the supported Smart Assistant or Image Creation workflows.

The product decision is to remove Digital Human functionality completely rather than keep it hidden behind a feature flag.

## Decision

- AI Applications contains Smart Assistants and Image Creation only.
- Remove Digital Human domain types, repository methods, HTTP APIs, browser client methods, routes, pages, translations, and styles.
- Remove the optional Digital Human reference from Smart Assistant configuration and conversation snapshots.
- Add an immutable migration that removes the snapshot key, drops `smart_assistants.digital_human_id`, and drops `digital_humans`.
- Do not retain compatibility endpoints or a hidden configuration surface. Requests to former frontend paths use the normal catch-all route, and former APIs are unregistered.
- Preserve the historical migrations that originally created and evolved the schema so a new database can replay the full immutable migration sequence before applying the removal.

## Consequences

Existing Digital Human records and Assistant bindings are deleted when the migration runs. This data removal is intentional and irreversible without restoring a database backup. Smart Assistants, FAQs, conversations, sharing, Knowledge Base bindings, Image Creation, and their histories remain supported; only the obsolete Digital Human snapshot field is removed from historical Assistant conversation records.

Any future avatar, voice, or real-time presentation capability requires a new product decision, domain model, provider contract, privacy review, and production evidence. It must not silently revive the removed schema or API.
