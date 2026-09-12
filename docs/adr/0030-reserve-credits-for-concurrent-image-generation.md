---
status: accepted
---

# Reserve Credits for concurrent Image Generation

Image Generation may run concurrently with the User's Workflow-scoped Runtime-backed model queues because every image request has a known maximum Credit cost. Submission atomically creates a source-preserving Image Credit Reservation for the full requested output count, removes that amount from Available Credit for every other admission, and assigns it to the submission Credit Day even when execution crosses midnight; terminal settlement charges only validated Generated Images and releases the remainder to its original source without carrying an expired Daily Credit Allocation forward.

This narrows ADR-0024's former User-wide serialization decision without weakening its no-Credits boundary. Runtime-backed invocations are serialized within each Workflow because their Token cost is not known in advance; different Workflows may execute concurrently. The Image Generation workbench permits only one active batch per User and rejects a second submission until the first becomes terminal.
