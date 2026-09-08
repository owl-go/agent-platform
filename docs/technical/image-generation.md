# Image Generation

Status: accepted design on 2026-09-08; implementation and live-provider evidence are pending.

This specification turns the AI Creation requirements in `docs/product/agent-workspace-requirements.md` into implementation seams and acceptance evidence. Domain terms come from `CONTEXT.md`. ADR-0029 fixes direct provider invocation outside Runtime Engines, and ADR-0030 fixes source-preserving Credit reservation as the concurrency mechanism.

## Scope

The first release provides an authenticated Web workbench for:

- text-to-image generation;
- image-to-image generation with one to ten ordered Reference Images;
- explicit Prompt Optimization with a User-selected, Administrator-approved text model;
- Administrator management and verification of Image Models and Image Credit Rates;
- durable progress, cancellation, results, history, regeneration, download, and deletion.

The first release does not provide conversational image editing, Workflow or Scheduled Trigger integration, Workflow credential access, an external image-generation API, public sharing, cross-User galleries, or Administrator access to User content.

The OpenAI Image API is the first image protocol because the official guidance identifies it as the direct generate/edit interface for a single prompt. The Adapter must not use the Responses image-generation tool as a hidden substitute. Prompt Optimization separately supports direct OpenAI Responses and OpenAI Chat Completions protocols.

## Context And Dependencies

AI Creation is a bounded context with its own Domain, Application, and Data packages:

```text
Authenticated HTTP/SSE
        |
        v
AI Creation Application
   |          |           |            |
   v          v           v            v
Catalog     Credits    Object Store   Provider ports
port        port       Provider       |-- OpenAI Images Adapter
   |          |                        |-- Fake Image Adapter
   v          v                        |-- OpenAI text Adapters
Workspace   Credits                    `-- Fake Prompt Adapter
context     context
```

The external AI Creation module interface contains use cases and observable invariants, not provider transport details. Provider request encoding, multipart construction, Base64 decoding, retry classification, and vendor errors remain behind injected ports. This gives the production and fake Adapters the same test surface.

AI Creation depends on:

- Workspace through a narrow catalog port for immutable Model Provider Connection and Provider Model revisions;
- Credits through reservation, text-admission, and settlement ports;
- `objectstore.Provider` for private bytes under logical Object Keys;
- PostgreSQL for aggregate state, durable work claims, progress events, preferences, and lifecycle metadata.

AI Creation does not import Workspace repositories, Credits GORM models, HTTP DTOs, YAML config, or a Runtime Adapter.

## Package Shape

The target package layout is:

```text
backend/internal/biz/aicreation/domain
backend/internal/biz/aicreation/application
backend/internal/data/aicreation/gormrepo
backend/internal/data/aicreation/openaiimages
backend/internal/data/aicreation/promptoptimizer
backend/internal/service/aicreation
backend/internal/wiring/aicreation
```

The API and Worker binaries explicitly assemble the same Application module with process-specific Adapters. Provider implementations may have private subpackages, but their details must not enlarge the Application interface.

## Domain Model

### Image Model

An Image Model has a stable identity and immutable revisions. A revision freezes:

- display name;
- Model Provider Connection identity and version;
- exact provider model identifier;
- `openai_images` protocol;
- supported mode: generate, edit, or both;
- allowlisted sizes, qualities, formats, backgrounds, and default values;
- verification status and evidence timestamp;
- one Image Credit Rate revision for every allowed size and quality pair.

Lifecycle states are `unverified`, `available`, `disabled`, and `deleted`. Creation and every material edit produce `unverified`; only a successful live test of the same revision permits transition to `available`. Endpoint, API Key version, model identifier, or option changes invalidate verification. Disable and delete immediately remove the model from User selection. Active records retain their frozen revision.

Known OpenAI models use versioned platform capability templates. An Administrator may narrow a template. A template update never expands an existing Image Model automatically. A custom compatible model requires explicit allowlists and the same live test.

### Prompt Optimization Candidate

A candidate binds an existing Provider Model revision to either `openai_responses` or `openai_chat_completions`. Only the Administrator changes the candidate set. A disabled or deleted Provider Model disappears from new User selection without changing completed Credit Ledger entries.

### Image Generation Record

One record is the aggregate root for a submitted batch. It freezes:

- owner and stable record identity;
- original User prompt and final submitted prompt;
- current verified Image Model revision at submission;
- ordered Reference Image snapshots;
- size, quality, output format, background, and requested output count;
- exact Image Credit Rate revision and Image Credit Reservation;
- submission Credit Day and timestamps;
- requested cancellation, work lease, progress, terminal state, and safe error;
- zero to four ordered Generated Image metadata rows.

Non-terminal states are `pending` and `running`. Exactly one terminal state is required:

- `succeeded`: every requested output is stored and validated;
- `partially_succeeded`: at least one but fewer than the requested outputs are stored, without accepted cancellation;
- `failed`: no output is stored and the provider outcome is known;
- `cancelled`: Stop was accepted, with zero or more outputs validated before cancellation;
- `outcome_unknown`: the provider may have accepted the request but the platform cannot establish its result.

Transitions are monotonic. Terminal records never reopen. Regeneration creates a new record. Aggregate state, its public progress event, successful output metadata, reservation release, and Credit Consumption commit atomically where the transition changes those facts.

### Reference And Generated Images

A Reference Image snapshot records ordered position, private Object Key, lowercase SHA-256, media type, encoded size, decoded width and height, and expiry. The record owns a copy even when its source is another Generated Image. Deleting the source record cannot break the later record.

A Generated Image records output position, private Object Key, lowercase SHA-256, actual media type, encoded size, width, height, and expiry. Generated Images are not Artifacts and do not use Workspace paths.

Reference and Generated Image bytes expire 90 days after their owning record is created. Metadata may remain until the User deletes the record. Regeneration is unavailable after any required Reference Image expires.

## Deep Provider Interfaces

The image seam is intentionally small:

```go
type ImageProvider interface {
    Create(context.Context, ProviderRequest) (ProviderResult, error)
}

type ProviderRequest struct {
    Connection ModelConnectionRef
    ModelID     string
    Prompt      string
    Inputs      []ImageInput
    Size        string
    Quality     string
    Format      string
    Background string
    Count       int
}
```

`ModelConnectionRef` contains only the frozen connection identity and version. The production Adapter receives a private credential-resolver dependency and loads the Endpoint and API Key just in time; credentials never cross the AI Creation Application interface. `Create` selects provider generate or edit transport from whether `Inputs` is empty. `ProviderResult` returns ordered encoded images plus safe request metadata; it never returns a provider URL for later unbounded fetching. The initial OpenAI Images Adapter accepts `b64_json`, decodes with explicit encoded and decoded limits, and rejects missing, duplicate, excessive, or malformed outputs.

The Application validates product and frozen-model allowlists before calling the port. The Adapter independently validates transport constraints and maps errors into stable classes:

- invalid request;
- authentication or authorization;
- model unavailable;
- safety rejected;
- rate limited;
- provider unavailable;
- deadline or cancellation;
- outcome unknown;
- invalid provider output.

Provider text and raw bodies are retained only as wrapped internal causes while in memory and must be redacted before public persistence or logs.

Prompt Optimization uses a separate interface:

```go
type PromptOptimizer interface {
    Optimize(context.Context, OptimizationRequest) (OptimizationResult, error)
}
```

The interface accepts the frozen connection revision, Provider Model, current prompt, locale, and 10,000-character output limit. Responses and Chat Completions are separate Adapters behind this seam. Success returns only expanded text and normalized token usage. Over-limit output fails without changing the editable prompt.

Live Image Model verification calls the same `ImageProvider.Create` seam with one platform-owned minimal prompt and the configured default options. It validates actual returned bytes. Tests use the fake Adapter at this interface rather than testing through provider-specific helpers.

## Application Use Cases

The Application module exposes cohesive methods for these caller intents:

### Administrator

- list Image Models, including verification and disabled states;
- create or revise an Image Model;
- test the current unverified revision;
- enable, disable, or delete an Image Model;
- replace the Prompt Optimization candidate set.

Enable validates complete capability defaults, all option combinations, complete Image Credit Rates, current connection credentials, and matching verification evidence. Delete rejects a model with a non-terminal record. Historical record snapshots never block deletion.

### User

- read available Image Models, optimization candidates, and lightweight preferences;
- update recent valid model and option preferences;
- upload or delete an unbound temporary Reference Image;
- optimize the current prompt;
- submit an Image Generation Record;
- get or page owned records;
- request Stop;
- regenerate from a terminal record using the current available model revision and current rate;
- delete a terminal record;
- authorize an image or ZIP download.

Every User method requires the authenticated owner. Cross-owner and missing identities use the same not-found behavior. Administrator status does not bypass ownership.

The workbench permits one non-terminal Image Generation Record per User. A second submission fails with conflict and creates neither a record nor a Credit reservation. Prompt Optimization is rejected while a record is non-terminal. Sessions and Workflows retain their existing serialized Runtime-backed queue and may run concurrently with the image record.

## HTTP Contract

Ordinary JSON operations are added to the authoritative Proto contract. Names may be grouped in the existing Kratos service, but implementations delegate immediately to the AI Creation Application module.

| Intent | Method and path | Authority |
|---|---|---|
| List available models and preferences | `GET /api/v1/ai-creation/image-generation/options` | User |
| Update lightweight preferences | `PATCH /api/v1/ai-creation/image-generation/preferences` | User |
| Optimize prompt | `POST /api/v1/ai-creation/image-generation/prompt-optimizations` | User |
| Submit record | `POST /api/v1/ai-creation/image-generations` | User |
| List history | `GET /api/v1/ai-creation/image-generations` | User |
| Read record | `GET /api/v1/ai-creation/image-generations/{record_id}` | Owner |
| Stop record | `POST /api/v1/ai-creation/image-generations/{record_id}/cancellation` | Owner |
| Regenerate record | `POST /api/v1/ai-creation/image-generations/{record_id}/regeneration` | Owner |
| Delete record | `DELETE /api/v1/ai-creation/image-generations/{record_id}` | Owner |
| List admin models | `GET /api/v1/admin/ai-creation/image-models` | Administrator |
| Create model | `POST /api/v1/admin/ai-creation/image-models` | Administrator |
| Revise model | `PATCH /api/v1/admin/ai-creation/image-models/{model_id}` | Administrator |
| Test model revision | `POST /api/v1/admin/ai-creation/image-models/{model_id}/test` | Administrator |
| Enable or disable model | `PATCH /api/v1/admin/ai-creation/image-models/{model_id}/availability` | Administrator |
| Delete model | `DELETE /api/v1/admin/ai-creation/image-models/{model_id}` | Administrator |
| Replace optimization candidates | `PUT /api/v1/admin/ai-creation/prompt-optimization-models` | Administrator |

Binary operations use explicit authenticated HTTP handlers because they stream bytes:

- `POST /api/v1/ai-creation/reference-images` accepts one multipart image and returns temporary metadata;
- `DELETE /api/v1/ai-creation/reference-images/{upload_id}` deletes an owned unbound upload;
- `GET /api/v1/ai-creation/image-generations/{record_id}/images/{position}` streams one authorized image;
- `GET /api/v1/ai-creation/image-generations/{record_id}/download` streams a temporary ZIP assembled from available images;
- `GET /api/v1/ai-creation/image-generations/{record_id}/events` streams bounded SSE progress.

Create, Stop, Regenerate, and Delete require an idempotency key or expected aggregate version as appropriate. Replaying the same submission returns the original record and never creates another reservation. SSE event sequence starts at 1, increases monotonically, and contains exactly one terminal event. Reconnection accepts the last event identity; a normal GET remains authoritative.

The public record response contains frozen display metadata, request options, ordered output metadata, safe status, total Credit Consumption, timestamps, and action availability. It never returns Provider Endpoint, credential identity, Object Key, raw provider response, or another User's data.

## Validation

Application validation is fail closed:

- prompt is non-blank and at most 10,000 Unicode characters;
- output count is between one and four;
- Reference Image count is between zero and ten;
- each input media type is PNG, JPEG, or WebP;
- each input is at most 20 MiB and 64 million decoded pixels;
- selected size, quality, format, and background form an allowed model combination;
- JPEG cannot request transparent background;
- every selected temporary or historical image is owned, unexpired, checksum-valid, and unbound or safely copyable;
- the Image Model is currently available and its connection revision remains usable;
- the User has no other non-terminal image record;
- Available Credit covers the complete reservation.

The server identifies media types from decoded bytes instead of trusting filenames or request headers. Per-output limits are 25 MiB encoded and 64 million decoded pixels. Output format and dimensions must match the frozen request. Invalid outputs are discarded before object-store write and do not consume User Credits.

The platform preserves Reference Image bytes and embedded metadata exactly after validation. The upload interface states that those bytes and metadata are sent to the selected provider.

## Credits

Credits gains image-specific ports rather than exposing its Repository:

- resolve or create immutable Image Credit Rate revisions;
- reserve the maximum batch amount atomically;
- read Credit Balance, Available Credit, and reserved total;
- settle a record exactly once with its validated output count;
- release a reservation on a zero-output terminal result.

Reservation locks the User's Credits projection, materializes the current Credit Day, verifies no existing record reservation, and preserves how much came from Daily Credit Allocation and Redeemed Credit Balance. The reservation belongs to the submission day across midnight. On settlement:

- successful outputs consume the frozen per-image rate;
- unused redeemed Credit returns to Redeemed Credit Balance;
- unused same-day Daily Credit returns to that day's remaining allocation;
- unused expired Daily Credit disappears instead of carrying into a later day;
- reservation, generic ledger entry, Available Credit, and projections update atomically.

Deleting a record never refunds Credits. It removes all private execution linkage while retaining a generic dated ledger amount required to explain the balance.

The avatar shows Available Credit. The Credit panel separately shows Credit Balance, Available Credit, active reserved amount, daily remaining, and redeemed balance.

Prompt Optimization uses normal text Model Credit Rate admission and settlement and joins the existing serialized text invocation queue. Each accepted click is independently charged. The workbench disables image submission while optimization is in flight and disables Prompt Optimization while an image record is active, so it never creates an ambiguous prompt or batch snapshot.

## Durable Execution

The Worker claims one pending record with PostgreSQL `FOR UPDATE SKIP LOCKED`, assigns a bounded lease, and advances it to running before provider I/O. A persisted dispatch marker distinguishes work known not to have reached the provider from work whose outcome may be unknown.

Recovery rules are:

- pending with no dispatch marker: reclaim and execute;
- expired running lease with proof of no provider dispatch: reclaim and execute;
- expired running lease after dispatch: terminate as `outcome_unknown` and release the full reservation because no image was validated;
- accepted Stop before dispatch: terminate as `cancelled` and release the reservation;
- accepted Stop after some outputs were durably validated: terminate as `cancelled`, settle those outputs, and reject later output;
- publish or persistence failure: stop processing and do not emit later progress.

The Adapter receives `context.Context`. Cancellation closes request bodies and stops local decoding. It cannot claim that a provider cancelled billing. Unknown provider cost is a platform concern and never becomes unsupported User Credit Consumption.

## Object Storage And Lifecycle

Only logical Object Keys are persisted. Suggested namespaces are:

```text
ai-creation/temp/{owner-id}/{upload-id}
ai-creation/records/{owner-id}/{record-id}/references/{position}
ai-creation/records/{owner-id}/{record-id}/outputs/{position}
```

Keys are constructed by one allowlisted helper and never accepted from clients. Writes validate exact size and lowercase SHA-256 before publication. Buckets remain private. Preview and download authorize the owner before opening an object; signed provider URLs are not stored.

Unbound uploads receive a 24-hour expiry. Navigation requests immediate cleanup, while a lifecycle job handles crashes and abandoned tabs. Binding a temporary upload copies or atomically promotes it into the record namespace only after submission and Credit reservation succeed. Failed submission removes any newly created record copies.

Record deletion requires a terminal state and transactionally records deletion before idempotent object cleanup. It clears prompts, snapshots, preferences specific to that record, Reference Images, Generated Images, and searchable metadata. A lifecycle job retries object deletion and expires bound image bytes after 90 days without erasing generic Credit Ledger entries.

## Frontend

`AI Creation` is a top-level navigation surface with `Image Generation` beneath it. The route remains visible without an available model and renders role-appropriate setup guidance.

Desktop uses the accepted two-pane workbench:

- left: mode, model, Reference Images, prompt, optimization model/action, size, quantity, quality, output format, background, estimated maximum Credits, and submit/Stop;
- right: empty state, progress, safe error, one large result or a responsive two-column grid, record metadata, downloads, regeneration, and reuse settings;
- collapsible history: newest-first pagination, no first-release search or filters.

Mobile stacks settings and results in one column and opens history as a drawer. Preview is full-screen with previous and next navigation.

While a record is non-terminal, every generation control and Prompt Optimization is disabled and only Stop remains active. A second submit attempt is also rejected server-side. Leaving the page clears prompt, undo content, temporary images, and current edits, while lightweight recent model and valid-option preferences remain. The background record continues and completion produces an in-product Toast and AI Creation unread marker.

The result view supports individual preview/download, download-all ZIP, and using an unexpired result as a new Reference Image. Regenerate uses original content and visible options but resolves current model and rate revisions and shows the current estimate before submission. Reuse settings removes expired inputs and invalid options.

The UI has no content-review selector and no model-group editor. It displays mandatory content-safety failures as safe errors and groups selectable Image Models by Model Provider Connection.

## Public Errors

At minimum the transport maps stable errors for:

- `image_model_unavailable`;
- `image_model_unverified`;
- `image_option_unsupported`;
- `active_image_generation_conflict`;
- `reference_image_invalid`;
- `reference_image_expired`;
- `image_output_invalid`;
- `image_safety_rejected`;
- `image_provider_unavailable`;
- `image_outcome_unknown`;
- `insufficient_credits`;
- `version_conflict`.

Errors preserve internal causes for diagnosis while public messages exclude provider bodies, prompts, images, credentials, Object Keys, and cross-owner existence.

## Migrations

Use new immutable, additive migrations for:

- stable Image Models and immutable model revisions;
- Image Credit Rate revisions;
- Prompt Optimization candidates;
- per-User lightweight preferences;
- temporary Reference Image uploads;
- Image Generation Records and progress events;
- bound Reference Images and Generated Images;
- source-preserving Image Credit Reservations;
- Credit Balance projection fields required for reserved and Available Credit.

Do not edit an existing migration and do not infer Image Generation Records from current attachments or Artifacts. Foreign keys and partial unique indexes enforce one non-terminal record and one active reservation per User. Object cleanup remains idempotent because database transactions cannot include Object Storage.

## Verification

### Required local gates

- Domain table tests cover every valid and invalid state transition, terminal uniqueness, full-batch regeneration, option allowlists, ownership, expiry, and cancellation races.
- Credits tests cover atomic reservation, source preservation, concurrent text admission, cross-midnight release, zero rates, partial success, deletion without refund, and exactly-once settlement.
- Repository tests cover `SKIP LOCKED` claims, lease expiry, idempotency, event sequencing, active-record uniqueness, optimistic concurrency, deletion, and lifecycle queries.
- Adapter contract tests run identically against fake image and fake prompt Adapters; HTTP fixture tests cover exact OpenAI JSON and multipart mapping without network access.
- Output tests cover MIME spoofing, malformed and oversized Base64, decompression bombs, unexpected dimensions/format, too many results, partial valid results, cancellation, and redaction.
- Service tests cover Administrator authority, owner isolation, generic not-found responses, streaming resume, binary authorization, and public error mapping.
- Frontend tests cover both modes, dynamic option compatibility, optimization/undo, locked generation, Stop, empty and no-model states, partial/unknown results, history, expiry, downloads, regeneration, deletion, responsive layout, keyboard navigation, and Chinese/English text.
- Generated Proto, OpenAPI, and frontend types remain in sync.
- `make test`, `make build`, `make web-typecheck`, and `make web-build` pass.

### Live-provider gate

A separate opt-in gate uses a protected OpenAI-compatible connection and verifies:

- one text-to-image request;
- one image edit with a Reference Image;
- every platform-advertised default option used by the enabled test model;
- actual output decoding, format, dimensions, checksums, and object persistence;
- one Responses and one Chat Completions Prompt Optimization when configured;
- secret and User-content canaries are absent from logs, events, errors, and ordinary database fields.

The gate records provider, exact model identifier, connection revision, test time, and safe outcome but never credentials, prompts, or image bytes. Missing credentials, provider organization verification, or network access causes an explicit skip and remains missing production evidence.

## Implementation Order

1. Add Domain types, state-transition tests, Credits reservation, and migrations.
2. Add catalog ports, deep provider interfaces, fake Adapters, and contract tests.
3. Add Administrator Image Model, verification, candidate, and rate use cases.
4. Add temporary uploads, submission, durable Worker execution, output validation, settlement, and lifecycle cleanup.
5. Add owner APIs, SSE, preview/download/ZIP, regeneration, and deletion.
6. Add the responsive AI Creation navigation, workbench, history, settings, and Credit UI.
7. Run local gates, then run and record the live-provider gate only when its environment is genuinely available.

## References

- `CONTEXT.md`
- `docs/product/agent-workspace-requirements.md`
- `docs/technical/service-architecture.md`
- `docs/technical/object-storage.md`
- `docs/adr/0029-call-image-models-outside-runtime-engines.md`
- `docs/adr/0030-reserve-credits-for-concurrent-image-generation.md`
- [OpenAI image generation guide](https://developers.openai.com/api/docs/guides/image-generation)
- [OpenAI Images API reference](https://developers.openai.com/api/reference/resources/images)
