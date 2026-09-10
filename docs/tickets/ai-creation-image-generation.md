# AI Creation Image Generation

## Problem Statement

Agent Workspace does not provide a dedicated place for Users to create images. Users cannot submit a text prompt or ordered Reference Images, select an Administrator-approved Image Model, control image options, optimize a prompt, follow durable progress, or manage private results. Existing Sessions and Workflows are conversational or task execution surfaces, and their Runtime Engines, Artifacts, and serialized execution rules are not an appropriate substitute for direct image generation.

Administrators also cannot configure and verify image-specific model capabilities or set complete Image Credit Rates. Reusing ordinary Provider Model settings would blur ownership between Runtime-backed text execution and direct Images API calls, while charging only after completion would allow concurrent work to overcommit a User's Credits.

The product needs a private, responsive Image Generation workbench under a new AI Creation area. It must make model capabilities, Credit reservation, cancellation, partial results, uncertain provider outcomes, storage lifetime, and regeneration behavior explicit without exposing prompts, images, credentials, or provider responses in logs or to other Users.

## Solution

Add AI Creation as a top-level product area and place Image Generation beneath it. The workbench supports text-to-image and image-to-image requests, Administrator-managed Image Models, up to ten ordered Reference Images, explicit Prompt Optimization with a User-selected text model, compatible size and quality options, one to four results, output format, background, and a visible maximum Credit estimate.

Submit each request as an immutable, User-owned Image Generation Record. Reserve its maximum Image Credits atomically, execute it as durable Worker work through a direct image-provider port outside Runtime Engines, stream progress over SSE, validate and privately store each returned image, and settle only validated results. Keep the active request running when the User leaves the page, allow Stop while all other controls are locked, and represent success, partial success, failure, cancellation, and an unknown provider outcome as distinct terminal states.

Show results in the right pane on desktop and below settings on mobile. Users can preview, download individually or as a ZIP, reuse an unexpired result as a Reference Image, reuse valid settings, regenerate the complete original batch as a new record, inspect newest-first history, and delete an entire terminal record. Retain input and output bytes for 90 days; deletion immediately clears private content and identifying metadata without refunding Credits.

## User Stories

1. As a User, I want AI Creation to appear as a top-level navigation entry, so that task-specific generative tools have a clear home outside Sessions and Workflows.
2. As a User, I want Image Generation beneath AI Creation, so that I can find the image workbench predictably.
3. As a User, I want the Image Generation route to remain visible when no Image Model is available, so that I understand why generation cannot start.
4. As a User, I want setup guidance when no Image Model is available, so that I know an Administrator must configure one.
5. As an Administrator, I want no-model guidance to link me to Image Generation settings, so that I can complete setup directly.
6. As an Administrator, I want to manage Image Models in dedicated Image Generation settings, so that image capabilities are configured separately from Runtime-backed text execution.
7. As an Administrator, I want an Image Model to reuse an existing Model Provider Connection, so that endpoints and API Keys remain managed in one place.
8. As an Administrator, I want each Image Model to bind one exact provider model identifier and Images protocol, so that requests are reproducible and capabilities are explicit.
9. As an Administrator, I want to configure supported generation modes, sizes, qualities, output formats, backgrounds, and defaults, so that Users can select only valid options.
10. As an Administrator, I want known provider models to start from versioned capability templates, so that setup is fast without silently assuming unsupported options.
11. As an Administrator, I want to narrow a known capability template but not expand it, so that configuration remains fail closed.
12. As an Administrator, I want custom compatible Image Models to use explicit option allowlists, so that nonstandard endpoints can be supported safely.
13. As an Administrator, I want every material Image Model edit to invalidate verification, so that stale test evidence cannot enable changed behavior.
14. As an Administrator, I want to live-test the current Image Model revision before enabling it, so that available models have verified credentials, options, and output bytes.
15. As an Administrator, I want template updates to require review, retest, and rate confirmation, so that new platform knowledge cannot silently change an enabled model.
16. As an Administrator, I want complete Image Credit Rates for every allowed size and quality pair, so that every selectable request has a deterministic maximum cost.
17. As an Administrator, I want an explicit zero Image Credit Rate to be valid, so that free image combinations are intentional rather than inferred from missing configuration.
18. As an Administrator, I want to enable, disable, and delete Image Models, so that User selection reflects the currently supported catalog.
19. As an Administrator, I want deletion of an Image Model with active records rejected, so that in-flight work retains a valid frozen configuration.
20. As an Administrator, I want completed records to retain frozen model metadata after model deletion, so that history remains understandable.
21. As an Administrator, I want to choose which existing text Provider Models may optimize prompts, so that Prompt Optimization uses an approved catalog.
22. As a User, I want to select an available Image Model, so that I control which configured model creates my images.
23. As a User, I want Image Models grouped automatically by Model Provider Connection, so that I can distinguish similarly named models without a manual group setting.
24. As a User, I want my most recent valid model and generation options remembered, so that repeated visits require less setup.
25. As a User, I want unsupported remembered options replaced by valid defaults, so that catalog changes do not leave the form in an invalid state.
26. As a User, I want to choose text-to-image mode, so that I can create images from a prompt alone.
27. As a User, I want to choose image-to-image mode, so that I can guide creation with visual inputs.
28. As a User, I want to upload one to ten ordered Reference Images, so that their sequence can carry intentional meaning to the selected provider.
29. As a User, I want PNG, JPEG, and WebP Reference Images up to 20 MiB and 64 million decoded pixels, so that common inputs work within safe limits.
30. As a User, I want the platform to validate actual image bytes rather than trust filenames, so that malformed or spoofed files are rejected safely.
31. As a User, I want Reference Image bytes and embedded metadata preserved, so that the provider receives the original validated input.
32. As a User, I want a disclosure that embedded metadata is sent to the selected provider, so that I can make an informed privacy choice.
33. As a User, I want to remove or reorder temporary Reference Images before submission, so that the frozen request matches my intent.
34. As a User, I want to reuse an unexpired Generated Image as a Reference Image, so that I can iterate visually without downloading and uploading it again.
35. As a User, I want a prompt of up to 10,000 Unicode characters, so that I can describe detailed composition, style, and constraints.
36. As a User, I want to select an Administrator-approved Prompt Optimization model, so that I control which text model expands my prompt.
37. As a User, I want Prompt Optimization to replace the editable prompt only after success, so that a failed optimization does not destroy my text.
38. As a User, I want optimizing a prompt not to generate images, so that I can review and edit the result before spending Image Credits.
39. As a User, I want one-step undo after Prompt Optimization, so that I can restore the immediately preceding prompt.
40. As a User, I want Prompt Optimization to be free of Image Generation Credits, so that I can refine a prompt before deciding whether to submit an image request.
41. As a User, I want over-limit optimized text rejected without changing my prompt, so that the form remains valid.
42. As a User, I want to select only size, quality, format, and background combinations supported by the chosen Image Model, so that invalid requests fail before provider work.
43. As a User, I want to request one to four images, so that I can choose between a single result and a small batch.
44. As a User, I want PNG, JPEG, or WebP output when supported, so that results fit my intended downstream use.
45. As a User, I want transparent background disabled for JPEG, so that the UI cannot express an impossible combination.
46. As a User, I want quality controls but no content-review selector, so that the workbench exposes useful image settings without pretending provider safety policy is optional.
47. As a User, I want to see the maximum Image Credit reservation before submission, so that I understand the batch's admission cost.
48. As a User, I want submission to reserve the full requested batch cost atomically, so that concurrent model work cannot overcommit my Credits.
49. As a User, I want Available Credit to exclude active Image Credit Reservations, so that the balance used for admission is accurate.
50. As a User, I want the avatar to show Available Credit and the Credit panel to separate balance, reserved amount, available amount, daily remainder, and redeemed balance, so that withheld Credits are understandable.
51. As a User, I want unused redeemed Credits returned after settlement, so that I pay only for validated Generated Images.
52. As a User, I want unused same-day Daily Credits returned after settlement, so that an incomplete batch does not consume unused entitlement.
53. As a User, I want unused Daily Credits from an expired submission Credit Day not to reappear, so that reservation release cannot carry daily allowance forward.
54. As a User, I want one non-terminal Image Generation Record at a time, so that active image work is clear and bounded.
55. As a User, I want a second submission rejected without creating a record or reservation, so that duplicate clicks cannot start duplicate batches.
56. As a User, I want Image Generation to run concurrently with my serialized Session or Workflow text work, so that image creation does not unnecessarily block other product areas.
57. As a User, I want all generation controls and Prompt Optimization locked while a record is active, so that its frozen request cannot change during execution.
58. As a User, I want Stop to remain available during generation, so that I can end work I no longer want.
59. As a User, I want an accepted Stop to ignore outputs that arrive later, so that cancellation has a clear boundary.
60. As a User, I want images durably validated before Stop to remain available and charged, so that completed provider work is not lost or falsely free.
61. As a User, I want generation to continue after I leave the page, so that navigation or a closed tab does not cancel durable work.
62. As a User, I want progress updates over a reconnectable stream and an authoritative record read, so that transient connection loss does not corrupt status.
63. As a User, I want an in-product completion Toast and AI Creation unread marker, so that I can discover a finished background batch.
64. As a User, I want pre-dispatch abandoned work reclaimed, so that a Worker restart does not strand a safe-to-retry request.
65. As a User, I want post-dispatch uncertain work marked outcome unknown instead of retried blindly, so that a Worker restart cannot duplicate provider generation.
66. As a User, I want success, partial success, failure, cancellation, and outcome unknown shown distinctly, so that I understand what happened and which images were accepted.
67. As a User, I want invalid, malformed, oversized, wrong-format, or wrong-dimension provider output rejected before storage and charging, so that only valid Generated Images consume Credits.
68. As a User, I want one result shown prominently and two to four results shown in a responsive grid, so that the gallery matches the batch size.
69. As a User, I want full-screen preview with previous and next navigation, so that I can inspect every result closely.
70. As a User, I want each result to show its frozen model, options, time, and Credit Consumption, so that the output remains attributable.
71. As a User, I want to download one image, so that I can use a selected result directly.
72. As a User, I want to download all available results as a ZIP, so that a complete batch is easy to retrieve.
73. As a User, I want newest-first paginated history, so that I can return to recent Image Generation Records without an unbounded page.
74. As a User, I want history to be collapsible on desktop and a drawer on mobile, so that it does not crowd the workbench.
75. As a User, I want Regenerate to create a new full batch using the original count, prompt, ordered Reference Images, and options, so that an unsatisfactory batch can be retried without overwriting history.
76. As a User, I want Regenerate to use the current verified Image Model revision and current Image Credit Rate, so that new work never relies on retired configuration or stale pricing.
77. As a User, I want the current maximum Credit estimate shown before regeneration, so that changed rates are visible before I submit.
78. As a User, I want regeneration disabled when a required Reference Image has expired, so that the platform never submits an incomplete reconstruction.
79. As a User, I want Reuse Settings to load still-valid inputs and replace invalid options, so that I can adapt an old request without claiming it is the same request.
80. As a User, I want Reference Image and Generated Image bytes retained for 90 days, so that I have a predictable download and iteration window.
81. As a User, I want unsubmitted temporary Reference Images cleaned up after 24 hours, so that abandoned private uploads do not persist indefinitely.
82. As a User, I want leaving the page to clear the prompt, undo state, current edits, and temporary Reference Images, so that unsubmitted private content is not treated as a saved draft.
83. As a User, I want to delete an entire terminal Image Generation Record, so that I can remove its prompts, images, and identifying metadata.
84. As a User, I want deletion to remove content without refunding Credits, so that history cleanup cannot reverse completed consumption.
85. As a User, I want only a generic dated Credit Ledger amount retained after deletion, so that my balance remains explainable without preserving private creation content.
86. As a User, I want every record, upload, preview, and download restricted to its owner, so that other Users cannot discover or access my images.
87. As a User, I want an Administrator to lack access to my prompts, Reference Images, Generated Images, and record details, so that administration does not bypass content privacy.
88. As a User, I want safe public errors for provider rejection and failure, so that credentials, prompts, images, Object Keys, and raw provider bodies never leak.
89. As an operator, I want logs and events limited to safe structured metadata, so that diagnostics do not persist prompts, images, Base64, signed URLs, Object Keys, credentials, or raw provider responses.
90. As a mobile User, I want settings and results stacked in one column with all required actions available, so that the workbench remains usable on a small screen.
91. As a keyboard User, I want every setting, gallery action, history action, dialog, and preview control reachable with a visible focus state, so that Image Generation is operable without a pointer.
92. As a Chinese or English User, I want localized labels, validation, statuses, and guidance, so that the feature matches the rest of Agent Workspace.

## Implementation Decisions

- Introduce AI Creation as a bounded context with separate Domain, Application, Data, Service, and wiring modules. It depends on narrow Workspace catalog and Credits application ports rather than importing their repositories or persistence models.
- Make the AI Creation Application the highest shared testing and integration seam. HTTP, SSE, Worker, Provider, Credits, and Object Storage details remain behind injected ports.
- Use a minimal `ImageProvider.Create` interface for both generation and editing. A request with no Reference Images uses the provider's generation operation; a request with ordered inputs uses its edit operation.
- Implement the first production adapter against the direct OpenAI Images API. Do not route image creation through a Runtime Engine or silently substitute the Responses image-generation tool.
- Use a separate `PromptOptimizer.Optimize` interface with a direct OpenAI-compatible Chat Completions adapter plus a fake adapter. Prompt Optimization is independent of image submission and is not charged.
- Resolve endpoint and API Key from the frozen Model Provider Connection revision just in time inside the production adapter. Credentials do not cross the AI Creation Application interface.
- Give each Image Model a stable identity and immutable revisions containing display metadata, connection revision, exact model identifier, protocol, supported modes, option allowlists, defaults, verification evidence, and complete Image Credit Rate revisions.
- Use lifecycle states `unverified`, `available`, `disabled`, and `deleted`. A material revision is unverified until the same revision passes a live output test; only available revisions appear in User selection.
- Treat each Image Generation Record as an immutable submitted batch and aggregate root. Regeneration always creates a new record and never replaces or reopens an existing record.
- Freeze owner, original and submitted prompts, current Image Model revision, ordered Reference Image snapshots, request options, requested count, rate revision, reservation, submission Credit Day, and timestamps at submission.
- Use `pending` and `running` as non-terminal states and require exactly one of `succeeded`, `partially_succeeded`, `failed`, `cancelled`, or `outcome_unknown` as the terminal state.
- Persist aggregate transitions, public progress events, accepted output metadata, reservation settlement, and Credit Consumption atomically when those facts change together.
- Enforce one non-terminal Image Generation Record and one active Image Credit Reservation per User with database constraints in addition to application validation.
- Reserve the maximum batch cost before record admission while preserving how much came from Daily Credit Allocation and Redeemed Credit Balance. Settlement charges only validated outputs and returns only amounts still valid under the original source rules.
- Keep Image Generation outside the existing per-User serialized Runtime-backed model queue, allowing one image batch to overlap Session or Workflow work. Prompt Optimization is a direct free request and does not enter that queue.
- Claim durable pending work with a bounded database lease and a persisted provider-dispatch marker. Reclaim only work proven not to have reached the provider; terminate expired post-dispatch work as outcome unknown without a blind retry.
- Propagate cancellation through context, close request bodies, stop local decoding, retain already committed valid outputs, and discard any output crossing the accepted Stop boundary.
- Accept one to ten ordered PNG, JPEG, or WebP Reference Images, each no larger than 20 MiB or 64 million decoded pixels. Identify media from decoded bytes and preserve accepted input bytes and metadata exactly.
- Accept one to four outputs. Validate Base64, actual media type, encoded size, decoded pixels, requested format, requested dimensions, ordering, and count before object publication and Credit settlement.
- Store temporary inputs, bound Reference Images, and Generated Images privately under server-constructed logical Object Keys. Never persist provider URLs or accept client-supplied keys.
- Give unbound uploads a 24-hour lifetime and record-owned image bytes a 90-day lifetime. Copy or promote a selected historical image into the new record so source deletion cannot break a later request.
- Expose ordinary JSON use cases through the authoritative Proto contract, explicit authenticated streaming handlers for multipart upload and binary download, and a dedicated reconnectable SSE stream for the active record. A normal record read remains authoritative.
- Require idempotency or aggregate version checks for submission, Stop, regeneration, and deletion. Replaying a submission returns the original record without creating another reservation.
- Return frozen display metadata, safe state, progress, output metadata, total Credit Consumption, timestamps, and action availability. Never return endpoints, credentials, raw provider responses, Object Keys, or cross-owner existence.
- Require owner authorization for every User operation and use the same not-found behavior for missing and cross-owner resources. Administrator role does not grant access to User creation content.
- Delete only terminal records. Mark deletion transactionally, clear prompts, input and output metadata, private bytes, and identifying linkage, then retry idempotent object cleanup. Retain only the generic Credit Ledger entry needed to explain balance.
- Add a responsive AI Creation navigation and Image Generation workbench. Desktop uses settings on the left and results on the right; mobile stacks the surfaces and moves history into a drawer.
- Keep model selection, prompt controls, generation options, Credit estimate, submit, and Stop in the settings surface. Keep progress, results, metadata, downloads, regeneration, reuse settings, and history in the result surface.
- Lock all mutable generation controls and Prompt Optimization while a record is non-terminal. Stop is the only active request control during generation, and the server independently rejects conflicts.
- Preserve lightweight recent model and valid-option preferences, but do not persist an unsubmitted prompt draft, undo content, form edits, or temporary Reference Image selection across navigation.
- Add an in-product completion Toast and unread marker for background records. Do not add public sharing, galleries, or administrative content browsing.
- Add immutable migrations for Image Models and revisions, Image Credit Rates, Prompt Optimization candidates, preferences, temporary uploads, records, events, bound inputs, outputs, reservations, and Credit projections. Existing migrations remain unchanged.

## Testing Decisions

- Test externally observable behavior at the AI Creation Application seam wherever possible. Tests should assert state, returned contracts, emitted events, stored objects, Credit effects, and stable errors rather than private helper calls, SQL shape, or provider-library internals.
- Run the same provider contract suite against production-shaped fake Image Provider and Prompt Optimizer adapters. Keep real-provider HTTP fixture tests deterministic and network-free for JSON, multipart, Base64, error classification, cancellation, and redaction behavior.
- Test the Image Generation Record state table exhaustively, including every valid transition, rejected transition, one terminal event, full-batch regeneration, version conflicts, cancellation races, and post-dispatch unknown outcomes.
- Test Administrator catalog behavior through the Application seam, including authority, immutable revisions, template narrowing, verification invalidation, live-test evidence matching, complete rate matrices, explicit zero rates, enablement, disablement, deletion, and active-record protection.
- Test User request validation through the Application seam, including prompt length, ownership, input count and order, media spoofing, input size and decoded-pixel bounds, expired images, unsupported option combinations, JPEG transparency, output count, model availability, and concurrent active-record conflicts.
- Test Credits reservation and settlement as a cross-context contract, including atomic admission, concurrent text admission, Daily and Redeemed source preservation, Credit Day rollover, expired daily remainder, partial results, zero-output release, zero rates, exactly-once settlement, and deletion without refund.
- Test durable work behavior at the Worker/Application boundary, including claim exclusivity, lease expiry, pre-dispatch reclaim, post-dispatch outcome unknown, accepted Stop, late output rejection, event ordering, persistence failure, and no event after terminal state.
- Test output validation against malformed or oversized Base64, MIME spoofing, decompression bombs, wrong dimensions, wrong format, missing and excessive results, mixed valid and invalid results, cancellation during decoding, checksum verification, and no charge for rejected output.
- Test Object Storage behavior with the existing shared provider conformance style, including logical key validation, exact size and lowercase SHA-256, private access, owner authorization, copy/promotion, 24-hour temporary cleanup, 90-day expiry, record deletion, and idempotent retry.
- Test service behavior for User and Administrator authority, generic cross-owner not-found responses, idempotent submission, optimistic concurrency, public error mapping, binary streaming authorization, ZIP contents, and SSE resume from the last event identity.
- Test the frontend for text-to-image and image-to-image modes, dynamic option compatibility, model grouping, prompt optimization and undo, Credit estimate, locked generation, Stop, progress, empty and no-model guidance, partial and unknown results, preview, individual and ZIP download, history, regeneration, reuse settings, expiry, deletion, Toast, unread marker, and navigation cleanup.
- Test desktop and mobile layouts, keyboard navigation, visible focus, screen-reader names, and Chinese and English copy as observable accessibility and localization behavior.
- Keep generated Proto, OpenAPI, and frontend types synchronized, then run the repository's Go tests and build plus Web typecheck and production build.
- Treat a fake adapter pass as local contract evidence only. A separate opt-in live-provider gate must prove text-to-image, image-to-image, configured defaults, real byte decoding and persistence, and configured Responses and Chat Completions optimization while verifying secret and User-content canaries do not appear in logs, events, errors, or ordinary database fields.
- Follow existing prior art for User-owned attachments, private object storage conformance, Credit Ledger admission and settlement, durable Worker claims, optimistic aggregate updates, and reconnectable progress streams rather than introducing parallel infrastructure patterns.

## Out of Scope

- Conversational or multi-turn image editing.
- Image Generation from Sessions, Workflows, Scheduled Triggers, Experts, or Expert Teams.
- Runtime Engine integration or a new image-specific Runtime Engine.
- Workflow credential access for Image Models or Prompt Optimization.
- An external Image Generation API for third-party callers.
- Public sharing links, public galleries, team galleries, or cross-User result reuse.
- Administrator access to User prompts, Reference Images, Generated Images, records, or execution-level content.
- Manual Image Model grouping; the UI groups models by Model Provider Connection.
- A content-review setting or an option to disable provider safety enforcement.
- Search and filtering in first-release Image Generation history.
- Persisted prompt drafts or restoration of unsubmitted form state.
- Selective regeneration of individual images; regeneration always requests the original full count.
- Refunds caused by record deletion.
- Native mobile applications; the first release is responsive Web only.
- Provider cost reconciliation or claims that Stop cancels provider billing.

## Further Notes

- The accepted design is recorded on branch `codex/image-generation` at commit `054cb0c` and is expanded by the product requirements, technical design, object-storage rules, service architecture, and ADRs 0029 and 0030.
- The attached screenshot is a visual reference for the two-pane workbench, not a source of product instructions. Confirmed conversation decisions override controls shown in the screenshot, including removal of content review and manual grouping.
- The official OpenAI guidance identifies the Images API as the direct interface for single-prompt image generation and editing. Responses and Chat Completions are used only for the separate Prompt Optimization action in this release.
- Live provider credentials, network access, and any required provider organization verification are not present evidence. A skipped live gate must remain reported as missing production conformance rather than a pass.
- The agreed highest test seam is the AI Creation Application. Production and fake Provider adapters share contracts behind that seam; provider-specific transport behavior is covered with fixtures rather than broadening the application interface.
