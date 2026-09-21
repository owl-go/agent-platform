# Agent Workspace Product Requirements

Status: accepted for implementation on 2026-08-25; Expert Team amendment accepted on 2026-09-02; Credits amendment accepted on 2026-09-04; Expert, Skill, and Connector simplification accepted on 2026-09-05; AI Creation amendment accepted on 2026-09-08; AI Applications amendment accepted on 2026-09-19.

## 1. Product Outcome

Agent Workspace is a personal AI workspace centered on Sessions, Workflows, AI Applications, Experts, Skills and Connectors, and Settings. It replaces the previous Coding Agent control-plane product rather than adding another layer on top of Agent Studio, Coding Tasks, Releases, and Operations.

The primary experience is intentionally lightweight: a ChatGPT-like left navigation and object list with one focused content surface. Advanced configuration remains collapsed until requested. The product name shown in the UI is `Agent Workspace`; the repository and deployment identifiers may remain `agent-platform`.

The UI supports Chinese and English. Language is stored in Personal Settings.

Transient operation success and failure feedback uses one standardized Toast pattern across product pages. Toasts remain above modal layers, carry the appropriate accessible live-region role, can be dismissed manually, and disappear automatically; persistent business content such as setup guidance and one-time credentials remains in the page.

## 2. Ownership And Accounts

- Every business resource is owned by one authenticated User and is invisible to other Users.
- Organization, Team, Role Grant, and the former four product roles are removed.
- One bootstrap Administrator account remains. The Administrator can also use private Sessions, Workflows, Expert Teams, and Settings; Experts, Skills, and MCP Connectors they create are Platform Resources.
- The Administrator opens User Management from the avatar menu and can create, disable, enable, and reset passwords for ordinary User accounts, configure Daily Credit Allocations and Model Credit Rates, manage Redemption Codes, and record reasoned Credit Adjustments.
- Account creation requires username, display name, and email. Keycloak generates a temporary password that is shown once and must be changed at first login.
- Keycloak remains the credential authority. The application stores only the User projection and never stores login passwords.
- A signed-in browser keeps the User session for up to 72 hours. The Web client persists the OIDC session and automatically renews short-lived Access Tokens within that window; explicit sign-out or account disabling ends access immediately.
- The Administrator can see account metadata and state but cannot read or modify User-owned content.
- Disabling a User cancels running work, stops schedules, rejects Workflow API credentials, and prevents login.
- Accounts are not permanently deleted in the first version.

### 2.1 Credits And Usage

- Every User receives a Daily Credit Allocation. Its default is 600 Credits, an Administrator may configure it per User, and unused Daily Credits do not carry into the next calendar day.
- A User may redeem a Redemption Code for additional Credits. Redeemed Credits persist across daily boundaries and are consumed only after the current day's allocation.
- Credit Balance is the sum of the remaining Daily Credit Allocation and Redeemed Credit Balance. Available Credit subtracts every active Image Credit Reservation and text Execution Credit Reservation and is the amount shown in compact account surfaces and used for new-execution admission. A User whose Available Credit is not positive cannot start another Session response, Workflow Run, or Image Generation Record; Prompt Optimization is free.
- Every actual Provider Model invocation consumes Credits, including every Subagent invocation in an Expert Team. Each successfully Generated Image consumes its frozen Image Credit Rate. Prompt Optimization is a free direct request and does not consume Credits. Validation, queueing, Runtime preparation, and MCP testing do not consume Credits.
- For a text invocation at a multiplier of 1.00, one Credit represents 10,000 input or output Tokens. Its Credit Consumption is calculated as `input Tokens / 10000 * input multiplier + output Tokens / 10000 * output multiplier`, rounded to two decimal places per invocation with a minimum non-zero charge of 0.01 Credits. Image Generation instead uses its exact per-image rate and successful output count.
- Each text invocation freezes its Model Credit Rate when the Session response or Workflow Run is queued. An Image Generation Record freezes its Image Credit Rates and reserves its maximum consumption when submitted. Prompt Optimization does not freeze a credit rate or consume credits. Later rate changes affect only later submissions.
- Successful, failed, cancelled, and timed-out invocations consume their measured usage when available. An execution that fails before Provider Model usage does not consume Credits, and a retry is a new independently charged execution.
- Starting a text invocation atomically creates an Execution Credit Reservation equal to its frozen Model Credit Rate fallback. Because final text usage is settled after execution, measured usage may exceed the reservation and make Credit Balance negative; later admissions are blocked when Available Credit is not positive. Image Generation requires enough Available Credit for its full maximum consumption and cannot itself overdraw the balance.
- Every Expert Team member creates and settles its own Execution Credit Reservation before invocation. If a preceding member's settled usage or the next reservation makes Available Credit non-positive, the turn fails before the next member starts; completed invocations remain charged, unstarted members are not charged, and the turn's temporary Workspace and Native Session changes are discarded.
- Each terminal Assistant response, Workflow Run turn, and Image Generation Record displays its total Credit Consumption. An Expert Team total is the sum of all member invocations that consumed Credits; an image total is the sum of its successfully validated outputs.
- A Runtime-reported input/output Token count is accepted for this anti-abuse accounting even when the Runtime image has not passed Usage Conformance. A Model Credit Rate supplies a fixed fallback charge when a successful final response has no Runtime-reported Token counts. A failed, cancelled, or timed-out invocation with no final response and no measured Usage is uncharged; the fallback never converts Runtime output into a claim of verified Usage support.
- The platform default Model Credit Rate uses input and output multipliers of `1.00` and a missing-Usage fallback charge of `10.00` Credits. An exact matching rate overrides those values.
- The User's current credit day begins at `00:00` in their Personal Settings time zone. A time-zone change takes effect at the next credit-day boundary and cannot restore the Daily Credit Allocation twice.
- A Redemption Code has a fixed Credit value, optional expiry, and active or void state. It can be redeemed successfully only once across the platform; generated plaintext is shown only at creation, while the platform retains a verifier and immutable redemption record.
- The Administrator may generate a batch of one to one hundred single-use Redemption Codes sharing one Credit value and optional expiry. Plaintext codes are shown once and may be copied or downloaded as CSV; invalid, used, void, and expired codes all produce the same User-facing unavailable message.
- Model Credit Rates match Provider type, Model API Protocol, and exact Provider Model identifier. They never depend on User identity, connection name, Endpoint, or Runtime Engine.
- Editing a Model Credit Rate creates an immutable revision. Each execution records its resolved revision and values, and historical Credit Consumption is never recalculated.
- The Administrator may make a Credit Adjustment to a User's persistent balance only with a required reason. Adjustments are immutable records; the current balance is never overwritten without one.
- A Credit Ledger is the immutable source of every daily allocation, redemption, adjustment, and consumption. The current balance and today's usage are transactionally maintained projections, not independently editable values.
- Negative Credit Balance carries forward and is offset by the next Daily Credit Allocation or redeemed Credits; the daily reset never forgives prior consumption.
- When an interactive Session or manual Workflow lacks Credits, the request fails without creating an execution. Workflow API requests return an explicit insufficient-credit error; a Scheduled Trigger records a failed, uncharged Run so its missed execution remains auditable.
- Runtime-backed text invocations are serialized per Workflow (and per individual Session), not per User. Different Workflows and different Sessions owned by the same User may run concurrently; each active text stage uses an Execution Credit Reservation. One separately reserved Image Generation Record may run concurrently with these queues; no User may have more than one active image generation batch.
- A User may read their complete Credit Ledger. The Administrator can read account balance, today's usage, Daily Credit Allocation, redemption, and adjustment records, but cannot read execution-level consumption records or their Session, Workflow, model, or Token details.
- The Administrator's own executions follow the same credit rules. Administrator changes to their own allocation or persistent balance use the same immutable records as changes for an ordinary User.
- Runtime usage is normalized to the input and output Token delta for the current model invocation. Resumed native-session totals must not cause tokens from earlier responses to be charged again.
- A newly created User immediately receives the current Credit Day's full allocation. On feature rollout, every existing User receives the current day's allocation with no redeemed Credits.
- A text model invocation belongs to the Credit Day in effect when that invocation starts, even if it settles after midnight. An Image Generation Record belongs to the Credit Day reserved when it is submitted. The next day's allocation offsets any resulting negative balance rather than changing either execution's original day.
- The current Credit Day's allocation is materialized transactionally on the first balance read or execution admission after its boundary. A uniqueness constraint for User and Credit Day prevents duplicate allocation without requiring a midnight batch job.
- Each text invocation's terminal Execution Stage state, Credit Ledger consumption, and balance projections commit in one database transaction. A single-stage execution or the final Expert Team stage commits the Assistant Message or Run terminal state in that transaction; the execution identity and stage position make every settlement idempotent across Worker retries. Image Generation terminal state, Credit Consumption, reservation release, and Generated Image metadata likewise commit atomically under the record identity.
- Credit Ledger entries, Redemption Code records, Model Credit Rate revisions, and Image Credit Rate revisions are retained for the life of the User and cannot be deleted in the first version. Deleting an Image Generation Record removes its private content and identifying execution metadata but retains a generic dated ledger entry and amount without issuing a refund.
- The avatar menu displays the User's Available Credit and opens a Credit panel with Credit Balance, Available Credit, active Image Credit Reservations, today's allocation remaining, redeemed balance, today's usage, next allocation time, Redemption Code input, and the User's Credit Ledger. The first version has no low-balance notification or configurable alert threshold.
- Each terminal Assistant response, Workflow Run, and Image Generation Record displays only its total Credit Consumption, such as `共消耗 ✧ 79.05`. Per-invocation Tokens, frozen multipliers, provider costs, and internal rate details are not shown in the execution interface.
- User Management contains `Users`, `Model Rates`, and `Redemption Codes` tabs. The Users tab displays balance, Credits consumed today, and Daily Credit Allocation and supports reasoned adjustments; allocation changes take effect on the next Credit Day, and a value of zero disables future daily allocation.
- A Model Credit Rate may explicitly set its input multiplier, output multiplier, or missing-Usage fallback to zero to make matching usage free. An unmatched model always uses the versioned platform default rather than becoming free implicitly.
- Insufficient Credits use the public error code `insufficient_credits`. HTTP APIs return `429 Too Many Requests` with the current Available Credit, active reserved total, and next Daily Credit Allocation time, without exposing another account or internal rate data.

## 3. Navigation And First Use

The main navigation contains exactly:

1. Sessions
2. Workflows
3. AI Applications
4. Experts
5. Skills & Connectors
6. Settings

The AI Applications entry (Chinese: `AI 应用`) contains `Smart Assistants` (`智能助手`), `Digital Humans` (`数字人`), and `Image Creation` (`图片创作`). Smart Assistants and Digital Humans are independently created User-owned objects; Image Creation is a task-oriented workbench. Smart Assistants may define ordered FAQs, an Answer Safety Policy, Knowledge Base retrieval, and an optional public iframe Share Configuration; external conversations remain isolated from the owner's private Session list. Image Creation uses a dedicated workbench rather than a Session or Workflow: its left side contains request settings and its right side displays generated results. An Administrator can manage the available image models from Image Creation settings, while every User may select one of those configured models for a request. The AI Applications entry remains visible when no verified Image Model exists: ordinary Users see setup guidance for Image Creation, while the Administrator receives an action that opens Image Creation settings. The Experts entry (Chinese: `专家`) contains `Experts` and `Expert Teams` tabs. The Skills & Connectors entry (Chinese: `技能·连接器`, not Capability) contains `Skills` and `Connectors` tabs. Login opens Sessions. A User without a usable platform Model Provider Connection, Provider Model, or personal Runtime Engine default sees setup guidance: an Administrator must configure the platform Model Catalog, then the User selects a default Provider Model for a Runtime Engine before starting a Session, Workflow, or Smart Assistant conversation. Expert, Skill, Connector, Smart Assistant, and Digital Human setup is optional and never blocks first use. Detailed AI Applications requirements are recorded in `docs/product/ai-applications-requirements.md`.

## 4. Sessions

The detailed conversation specialist/resource selection rules and accepted revision precedence are recorded in `docs/product/conversation-resource-selection.md`. These are product targets, not implementation acceptance evidence. Personal Settings execution freezing and initial Workflow trigger behavior remain unchanged.

### 4.1 Lifecycle

- A User can create, rename, archive, cancel archive, and permanently delete a Session.
- New Session creation succeeds immediately without an Expert. The composer offers a grouped `No Expert / Expert / Expert Team` selector through its `+` menu, including after the first message, without reopening a creation modal.
- The first Runtime invocation in a new Session is explicitly scoped to that Session and must not assume context from any other Session. Subsequent platform summaries and recent messages are always labeled and queried as history from the same Session.
- Each Session response can use no specialist profile, one Expert, or one Expert Team. The selection persists until changed, while every accepted message preserves its actual Expert Snapshot containing visible profile metadata, structured Expert guidance, stable Team Member identities and order, and exact Skill and Connector revisions. Selection changes affect subsequent messages only.
- The composer supports direct Skill and Connector selection with or without an Expert. Explicit Skills apply to one message; the selected specialist and Connectors remain for subsequent messages. Expert-derived defaults, explicit overrides, revision retention, and Conversation Draft recovery follow `docs/product/conversation-resource-selection.md`.
- The title is derived locally from the first User message and remains editable; title generation does not invoke a model.
- Archived Sessions are hidden from the active list and read-only until archive is cancelled.
- Deleting a Session requires confirmation, cancels active generation, and permanently deletes messages and Session execution data.
- A Session message may contain text, up to ten distinct new attachments and referenced files in total, or both. Each file is at most 100 MiB. In both the Session composer and Workflow Run Conversation composer, a User may select local images and files or paste them directly from the clipboard; pasted files follow the same limits, draft preservation, and upload lifecycle. File References preserve content accepted with the message. Sessions do not own a persistent Workspace; the Runtime receives checksum-verified, read-only copies for that turn.
- Selecting an attached image opens an in-product preview instead of starting a browser download. A message with multiple images supports previous and next navigation in the preview; non-image attachments remain explicit downloads.

### 4.2 Conversation Execution

- The Session composer has no Provider Model or Runtime Engine selector. The first message resolves and freezes the current Personal Settings default Runtime Engine and that engine's default Provider Model for the Session, regardless of whether an Expert or Expert Team is selected.
- Each User message freezes a Response Snapshot containing one ordered Execution Stage Snapshot per actual Provider Model invocation. Every stage records its optional Expert and Team Member identities, Model Provider Connection version, Model API Protocol, Endpoint, the Session's frozen Provider Model and Runtime Engine, structured Expert guidance, and exact Skills and Connectors; API Keys and Connector credentials remain protected versioned references and never enter ordinary message data.
- A Personal Settings change affects only Sessions started afterward and never changes an existing Session. Editing an Expert or resource does not change a retained selection; explicit reselection adopts its latest available revision. Regeneration always reuses the original Response Snapshot.
- A Session may keep each isolated Runtime container definition warm for 30 minutes after a response finishes. Warm reuse is scoped by Session, frozen Team Member identity when present, Expert identity, and Runtime Engine, so team members never share execution context even when they reference the same Expert.
- The UI streams the response and shows generating, failed, cancelled, retry, and elapsed-time states. While a response is generating, the send control becomes a stop control that requests backend cancellation and stops the active Runtime execution. It does not expose Attempt, Lease, Runtime Event, or a separate Run record.
- While generating and after completion, the UI may show persisted execution activity. Its first expanded level groups activity into concise execution summaries such as preparing the Runtime, searching a Connector, updating files, and composing the answer. Expanding one summary reveals its public reasoning summary and redacted tool commands. It never exposes raw model chain-of-thought, private reasoning, raw Runtime events, or tool output.
- A single selected Expert executes directly using its four visible guidance fields and snapshotted Skills and Connectors. An Expert Team uses the fixed sequential execution contract in Section 9.2.
- Ordinary Runtime text remains a message; only files explicitly uploaded by a User or actually generated by a Runtime are presented as files.
- The platform maintains a Rolling Summary and recent-message window after every successful response.

### 4.3 Native Resume

- Claude Code and Codex may use native Session Resume only for Runtime images whose `native_resume` conformance evidence has passed.
- Hermes, OpenClaw, and PI Agent use the Rolling Summary and recent messages for every response until their native Resume capability is independently verified.
- Runtime identity comparison deliberately uses only the Runtime Engine name. A CLI, Adapter, or image upgrade does not proactively invalidate the Native Session; a safe classified Resume failure may cause fallback.
- Existing Sessions retain their initial execution configuration, exact retained resource revisions, and immutable historical Response Snapshots. Catalog edits and deletion do not rewrite those snapshots; explicit selection changes resolve the next message's resources. Current Connector authorization and enablement are revalidated before each external command.
- Switching specialists or changing resources must not leave removed resources executable through native Resume or warm-container reuse. Platform-owned history preserves conversational continuity when the new selection cannot safely reuse a preceding execution context.
- Each Expert Team member owns independent staged Native Session state. The platform promotes all member states only after the entire turn succeeds and discards every staged state on failure or cancellation; retry restarts the full frozen member order from the preceding successful turn.
- Automatic fallback is allowed when the platform can prove before execution that the local native state for a checkpoint is absent, or when the Runtime reports that the checkpoint is invalid before any action executes. All other Resume failures are shown to the User and may be retried manually.
- Native Resume is an optimization. Platform messages and the Rolling Summary remain the correctness boundary.

## 5. Workflows

### 5.1 Definition And Lifecycle

- A Workflow is a single executable definition, not a visual DAG. It may invoke one fixed-order Expert Team but does not support arbitrary multi-agent graphs.
- A User can create, edit, run, and delete a Workflow.
- Required fields are name and goal. One Expert or Expert Team is optional.
- Workflow cards show a specialist label only when an Expert or Expert Team name is available; omit the label and its container when no specialist is selected.
- A Workflow has no Provider Model or Runtime Engine override. Every new Run Conversation resolves and freezes the current Personal Settings default Runtime Engine and that engine's default Provider Model; the same configuration applies to no Expert, one Expert, or every Team Member.
- The first Run in a Run Conversation freezes a Workflow Snapshot containing the goal, initial ordered Execution Stage Snapshots, environment, and other execution inputs. Each Run preserves a Response Snapshot for its actual specialist and resource selections. Follow-ups retain the initiating goal, environment, and execution configuration while adding immutable turn input and applying selections from that Run Conversation's composer; API Keys are held only through protected versioned credential references.
- A Workflow keeps each isolated Runtime container definition warm for 30 minutes after a Run finishes so frequent serialized Runs do not recreate containers. Warm reuse is scoped by Run Conversation, frozen Team Member identity when present, Expert identity, and Runtime Engine; containers stop between Runs and are destroyed after 30 minutes without use.
- Editing a Workflow affects future Run Conversations only; follow-up Runs reuse their initiating snapshot. There is no Draft, Release, publish, approval, or visible version flow.
- Only one Run may modify a Workflow at a time. Manual, scheduled, API, follow-up, and rerun requests enter that Workflow's persistent FIFO queue, including concurrent follow-ups in one Run Conversation. Different Workflows may execute concurrently. The first version permits at most five queued Runs per Workflow, excluding the currently running or waiting Run; a manual/API request over the limit returns `429 queue_full` without creating a Run, while a scheduled trigger records a failed history Run.

### 5.2 Detail Page

The Workflow detail page contains four tabs in this order:

1. Artifacts
2. Workspace
3. Run History
4. Settings

Run polling is limited to visible Run Conversations, Run History, and Artifacts. Settings and Workspace browsing do not repeatedly request Runs or Artifacts. Visible active execution refreshes Run state every 1.5 seconds; idle history checks every 30 seconds for scheduled or API Runs. Artifacts refresh on execution changes or when opening their tab, rather than on every idle poll. Hidden browser tabs pause polling and refresh when visible again; requests do not overlap.

Settings contains five collapsed sections:

- Basic: name, goal, optional Expert or Expert Team
- Execution: read-only Personal Settings execution summary, plus environment variables
- Schedule: hourly, daily, or weekly trigger with time and optional time-zone override
- API Credential: generate or regenerate API Key/API Secret and show the JWT exchange and Bearer invocation examples
- Git Source: URL, branch, public HTTPS/account-password/private-key authentication, safe local Git config, and optional Workflow-scoped SSH config

### 5.3 Triggers And Input

- Manual Runs launched from the Workflow interface execute the fixed Workflow goal directly; the detail header does not expose a separate input or input-type control. After creation, the interface immediately opens that Run Conversation and streams the active Run instead of requiring another click in Run History.
- The existing Run Conversation follow-up composer supports the same `+`, Skill tokens, File References, clipboard image/file paste, and retained specialist/Connector selections as Sessions. It additionally offers the Workflow's Workspace files in the file picker. These controls do not extend to goal editing or initial trigger input and do not update the Workflow definition.
- API Runs accept optional text or JSON input.
- Scheduled Runs execute only the fixed Workflow goal and do not have a separate default input.
- Run trigger types shown to Users are manual, scheduled, and API.
- Accepted requests return `202` and a stable Run ID immediately. Queue position is computed from the authoritative `queued_at + id` order and is shown while a Run is queued; it is not a persistent ordinal.
- Concurrent follow-ups are accepted and receive turn numbers transactionally in enqueue order. A cancelled turn keeps its number and does not block later queued turns once the Workflow slot is free.
- An incomplete selected Expert, invalid Expert Team, unavailable Skill or Connector revision, or unavailable Personal Settings execution configuration prevents manual and API requests before a Run is created. A Scheduled Trigger instead creates an explicit failed, uncharged Run identifying the unavailable dependency; a queued Run revalidates its frozen first stage before model invocation and fails uncharged if that dependency became unavailable.

### 5.4 Environment

- Workflow environment variables are either ordinary or Secret.
- Ordinary values may be viewed and edited.
- Secret values are write-only after submission and can only be replaced or deleted.
- Only variables explicitly assigned to the Workflow enter its execution environment.
- Secret values are removed from logs, events, final results, and Artifacts before persistence.

### 5.5 API Credential And Contract

- API access is opt-in. No credential is generated when the Workflow is created.
- A Workflow has one API Key/API Secret pair. Regeneration immediately revokes the old pair.
- The API Key and API Secret are encrypted at rest and returned only to the owning User from the Workflow detail page, where both can be copied. HTTP Basic Auth with API Key as username and API Secret as password is accepted only by the token exchange endpoint. It returns a 72-hour JWT access token.
- `GET /api/v1/workflows/{workflowId}/api-credential` returns the current pair only to the owning User. Credentials generated before encrypted Secret storage was introduced must be regenerated before the Secret can be displayed.
- Workflow invocation and inspection use `Authorization: Bearer <jwt_token>`. Regenerating the credential invalidates outstanding tokens because their signature key derives from the current credential verifier.
- `POST /api/v1/workflows/{workflowId}/runs` accepts text or JSON input and returns `202` with a Run ID.
- `GET /api/v1/workflows/{workflowId}/runs/{runId}` returns status and final result.
- `GET /api/v1/workflows/{workflowId}/runs/{runId}/events` provides live Run events through SSE; clients can use `Last-Event-ID` to resume from the last received event.
- The one-shot result read uses the same `GET /runs/{runId}` endpoint after the Run reaches a terminal state and returns the complete `final_text` or `final_json`.
- `Idempotency-Key` prevents external retries from creating duplicate Runs.
- A Workflow credential authorizes only starting and inspecting that Workflow. It cannot access Settings, Workspace mutation, another Workflow, or User APIs.

### 5.6 Schedule

- A Workflow can have one optional hourly, daily, or weekly Scheduled Trigger.
- Personal Settings provides the default time zone. The Workflow schedule may override it.
- A queued scheduled Run waits behind the active Run for the same Workflow. Queue order is shared with manual, API, follow-up, and rerun requests and is FIFO.
- Deleting a Workflow or disabling its owner stops future scheduling.

## 6. Workspace

- Every Workflow owns one persistent Workspace reused across Runs.
- The Workspace tab is read-only: its tree view shows directories and files and allows supported text preview and file download. It does not create directories, upload, edit, rename, move, or delete entries.
- Maximum Workspace size is 1 GB and maximum inline text preview size is 1 MB.
- Workspace initialization by Git Clone is configured only in Workflow Git Settings, not in the Workspace browser.
- Git supports public HTTPS, HTTPS username/password or token, and private SSH repositories. Passwords, tokens, and private keys belong only to that Workflow, are write-only, and are destroyed with the Workspace.
- Saved Git credentials display only a saved, write-only status. Their input is removed after successful saving and when reopening the page; explicitly choosing to enter new credentials opens an empty input labeled as not yet saved. Failed replacement attempts keep only the newly entered draft, and changing authentication mode or leaving the page clears credential inputs. Read responses contain only the configured flag, never password or private-key content.
- Git config is stored as an ordered key/value list and restricted to a safe allowlist; command, credential-helper, include, URL rewrite, and transport override keys are rejected.
- Git Settings examples use generic host aliases, reserved example domains, usernames, and key filenames; they must not expose deployment IP addresses or operator-specific connection details.
- Save-and-clone feedback appears persistently beside its action and is brought into view. Failures preserve the input and explain recognized causes (nonempty Workspace, invalid configuration, missing credentials, SSH host/key checks, authentication, repository/branch access, network, or size limit) through localized safe error codes rather than raw Git output. Cloning never automatically clears an existing Workspace; success clears the entered password and private key from the form.
- A private SSH source may store one Workflow-scoped SSH config containing an exact `Host` alias plus allowlisted connection fields. During Clone, the API materializes it as `~/.ssh/config` inside an isolated temporary HOME and writes the private key under the configured `IdentityFile`; both are removed after the attempt. It never modifies the API host account's SSH config.
- SSH private keys pasted without a final newline receive that newline when materialized for OpenSSH. The caller's key bytes remain unchanged, temporary key files stay private (`0600`), and cleanup removes them after the attempt.
- SSH config accepts only `Host`, `HostName`, `User`, `Port`, `IdentityFile`, `IdentitiesOnly`, `ServerAliveInterval`, and `ServerAliveCountMax`. It rejects wildcard hosts, includes, match blocks, proxy or local commands, arbitrary identity paths, and every other directive. Host verification remains pinned to the Administrator-provided `known_hosts` file.
- Clone requires an empty Workspace.
- Runs operate on a temporary copy. A successful Run atomically advances the persistent Workspace; failed or cancelled Runs discard their file changes.
- Runtime and Skill processes access only the temporary Workspace, explicitly assigned environment, and allowed public network. They cannot access the host, platform private services, or another User's data.

## 7. AI Applications

AI Applications is a top-level product area containing Smart Assistants, Digital Humans, and Image Creation. Smart Assistant and Digital Human requirements, including their ownership, lifecycle, configuration, conversations, reuse, and deletion conflicts, are defined in `docs/product/ai-applications-requirements.md`.

Image Creation is the user-facing name for the existing Image Generation workbench. It is not a Session, Workflow, Run, or Runtime Engine capability. Its dedicated responsive workbench presents request settings on the left and the current Image Generation Record and its Generated Images on the right. The lower-level Image Generation Record, Generated Image, Reference Image, Image Model, Image Credit Rate, and Image Credit Reservation terms remain unchanged.

- Image Generation supports text-to-image requests and image-to-image requests. Supplying at least one Reference Image selects image-to-image behavior; otherwise the request is text-to-image.
- The platform invokes the selected Image Model directly rather than asking a Runtime Engine to generate the images. An OpenAI-compatible Endpoint uses the OpenAI Images protocol; the exact Alibaba Model Studio multimodal-generation Endpoint uses its native synchronous image protocol. The Administrator does not select a separate provider or protocol field.
- An Image Model is an Administrator-managed configuration containing an exact model identifier, an independent API Endpoint, and a write-only API Key. It never references or copies a Model Provider Connection or Provider Model credential and does not appear as a Runtime Engine default. The API Key is encrypted at rest, never returned by a read API, and must be entered again when migrating an older connection-backed Image Model.
- Only the Administrator can open Image Creation settings or create, update, test, enable, disable, and delete Image Models. The settings form contains only model identifier, API Endpoint, and API Key; output parameters remain User request settings. The User-facing selector lists enabled Image Models directly, without Model Provider Connection grouping.
- Every User may select an enabled Image Model for a request. The workbench remembers that User's most recently selected valid Image Model and requires a new selection if it becomes unavailable.
- The request requires a non-blank prompt of at most 10,000 Unicode characters and lets the User choose size, quality, output count, output format, and background. Size may be selected from the Image Model presets or entered as custom positive whole-pixel `widthxheight` dimensions whose total does not exceed 64 million pixels; the selected provider may still reject dimensions it does not support. Output count is limited to one through four, output format to PNG, JPEG, or WebP, quality to auto, low, medium, or high, and background to opaque or transparent. The API rejects invalid sizes and unsupported combinations such as transparent JPEG. Content safety enforcement is always active and is not displayed as a User setting.
- An image-to-image request accepts up to ten ordered Reference Images. A User may upload PNG, JPEG, or WebP images of at most 20 MiB and 64 million decoded pixels each or select one of their own unexpired Generated Images. The platform validates that each upload decodes as its declared image type, then preserves and sends the original bytes without removing EXIF or other metadata. Accepted input bytes are retained with the immutable record independently of the source file's later lifecycle and expire 90 days after record creation.
- Prompt optimization is an explicit action before generation. The Administrator configures exactly one optimization model identifier, independent API Endpoint, write-only API Key, and optimization instruction in Image Creation settings; it never references or copies a Model Provider Connection. A successful optimization replaces the editable image prompt with expanded text, offers one-step restoration of its immediate predecessor, and never starts image generation. Repeated optimizations use the current text, are free of image-generation credits, and the immutable Image Generation Record preserves the initial User prompt and final submitted prompt without retaining intermediate drafts. Prompt Optimization calls the configured OpenAI-compatible Chat Completions endpoint, does not use or change a Runtime Engine default, and is unavailable while an Image Generation Record is active.
- Submitting a valid request creates one immutable, User-owned Image Generation Record that freezes the selected Image Model revision, prompt, ordered Reference Images, requested output count, size, quality, output format, background, and applicable Image Credit Rate revisions.
- Image generation continues as a background task if the User leaves the page. The workbench restores active progress when reopened and offers best-effort cancellation. One image batch may run in parallel with the User's Session and Workflow queues, but the workbench locks every generation setting and rejects another image submission until that batch reaches a terminal state; only Stop remains available. Work not yet sent to the provider stops immediately. Once Stop is accepted, the platform rejects any later provider output: already validated images remain and are charged, while later or unknown output is neither stored nor charged to the User even if the provider charges the platform.
- Durable work ownership and leases allow a pending request that has not reached the provider to resume after an API or Worker restart. A request known to have reached the provider but lacking a confirmed result terminates as outcome_unknown and is never retried automatically.
- The active record streams bounded product progress over a dedicated SSE endpoint. Reconnection and browser visibility recovery first fetch the authoritative record and then resume streaming; events expose no raw provider response, image bytes, prompt, or private reasoning.
- An Image Generation Record has exactly one terminal state: succeeded, partially_succeeded, failed, cancelled, or outcome_unknown. The platform decodes and validates actual output bytes against the frozen format, dimensions, 64-million-pixel limit, 25-MiB encoded-size limit, and other safety limits before persistence. A mismatched or oversized output is not stored, shown, or charged; other valid outputs may still make the batch partially successful. A partially successful or cancelled record retains and settles only successfully validated images. Safe structured errors explain a provider failure or safety rejection without displaying a content-review control or raw provider response.
- Each successfully Generated Image is private to the owning User, retained for 90 days, and available for in-product preview, individual download, or reuse as a Reference Image. One result uses a large presentation; two through four results use a responsive two-column desktop grid and single-column mobile layout, with full-screen preview on selection. Missing positions in a partially successful batch display their safe failure state. Record metadata remains after image expiry. A User may delete an entire terminal Image Generation Record and all of its prompts, identifying metadata, Reference Images, and Generated Images, but cannot delete one Generated Image from a record; an active record must first be stopped. A later record that copied one of its images as a Reference Image remains independent. Downloading all available results creates a temporary ZIP and does not persist another Generated Image.
- Image Generation history is a collapsible region within the workbench, ordered newest first and paginated without search or filters in the first version. It becomes a drawer on mobile and lists only the owning User's records without exposing another User's prompts, Reference Images, Generated Images, model selections, or costs.
- The workbench has no persistent unsubmitted draft. Navigating away or refreshing clears an unsubmitted prompt, undo value, Reference Image uploads, and current parameter edits; an already submitted Image Generation Record continues independently. The User's most recently selected valid Image Model and valid options remain lightweight preferences and repopulate a fresh workbench. Successfully submitting a record likewise clears the editable prompt and Reference Images while retaining those preferences.
- Every terminal record exposes one `Regenerate` action rather than separate retry and regeneration semantics. It creates a new Image Generation Record for the full original output count using the original prompts, Reference Images, and visible request options, but resolves and displays the current verified Image Model revision and current Image Credit Rate before submission. If the current model no longer supports the original options, direct regeneration is unavailable. Every original result remains unchanged even when only some were unsatisfactory. The platform automatically retries only when it can prove a request did not reach the provider; an unknown provider outcome is never retried blindly. `Reuse settings` instead copies the original request into the editable workbench without submitting it.
- Regeneration is unavailable when its Image Model is disabled or deleted or any Reference Image bytes have expired. `Reuse settings` then copies the prompt and still-valid options, removes unavailable Reference Images, and requires a currently enabled Image Model.
- Image generation has a platform-fixed price of 50 Credits per successfully Generated Image, independent of model, size, quality, format, or background; it is not an Administrator setting. Submission atomically reserves `50 × requested output count` Credits from the current Credit Day, preserving the amounts drawn from Daily Credit Allocation and Redeemed Credit Balance, and that reservation remains assigned to the submission day across midnight. Available Credit excludes every unsettled reservation so concurrent Session and Workflow admission cannot spend it. Terminal settlement charges `50 × successfully Generated Image count` and releases the remainder to its original source: an expired prior-day allocation does not return, while unused redeemed Credit does. Every frozen Credit Consumption remains immutable. Prompt Optimization remains a separate free direct Chat Completions request and does not create Credit Consumption.
- The result panel shows the frozen model display name, creation time, requested size, quality, format, output count, terminal state, and total Credit Consumption. Each Generated Image shows its actual pixel dimensions, file size, and download action. It never exposes the Provider Endpoint, internal Object Key, raw provider response, or provider cost; downloads use a stable record-time and output-position filename.
- A new or materially changed Image Model cannot be enabled until the Administrator completes a real test generation that validates the current Endpoint, API Key, model identifier, returned content, and configured limits. The test incurs provider cost but consumes no User Credits and does not claim that every parameter combination has production evidence. Changing the Endpoint, API Key, model identifier, or supported options returns the Image Model to an unverified, disabled state.
- Disabling an Image Model immediately prevents new submissions and regeneration without changing historical records. An Image Model with an active or queued record cannot be deleted; after deletion, frozen historical identity remains readable but cannot be invoked. A deleted or disabled model selection is cleared the next time a User opens the workbench.
- The Reference Image uploader states that selected image bytes and embedded metadata are sent unchanged to the configured Image Model API. It does not expose the API Endpoint or add a per-upload confirmation dialog.
- Image Generation Records, prompts, Reference Images, and Generated Images are completely private to their owning User. The first version has no public link, cross-User share, gallery publication, or Administrator content access.
- Ordinary logs and audit records may retain only safe structured error codes, HTTP status, provider request identity, duration, returned count, and byte-validation outcome. They never retain prompts, image bytes or Base64, raw provider responses, Object Keys, signed URLs, or other User content.
- Because the workbench has no draft, leaving it requests immediate deletion of every unsubmitted temporary Reference Image. A lifecycle job deletes every unbound temporary upload after 24 hours even when browser cleanup never arrives.
- When a background record reaches a terminal state away from the open Image Creation view, the product shows an in-app Toast and an unread marker on AI Applications. Opening Image Creation clears the marker; the first version does not request browser notification permission or send email.
- The first version exposes Image Generation only through the authenticated Web workbench. Workflows, Scheduled Triggers, Workflow API credentials, and external image-generation APIs cannot start it.

## 8. Artifacts And Run History

### 8.1 Artifacts

- A successful Run persists its final text or JSON in the Run Conversation and captures only final deliverable files explicitly named in that response as Artifacts.
- A successful Session response also captures only final deliverable files explicitly named in that turn's Agent response and shows them directly beneath the response.
- Dependencies, fonts, generation scripts, caches, and other intermediate Workspace files are not Artifacts. A Workflow may retain those files in its persistent Workspace for later Runs; a file explicitly captured as a final Artifact is removed from the persistent Workspace after it is copied to Artifact storage.
- Only actual generated or changed final files are Artifacts. An ordinary text or JSON response does not create a synthetic file or Artifact.
- Artifacts are grouped by Run time. A terminal response with final deliverables shows compact, mutually exclusive `View all artifacts` and `View all changes` controls; the controls expand downloadable file cards and final-file metadata respectively, without rendering file contents in the conversation.
- Session and Run Conversation final text renders references to captured Artifacts as plain file names rather than download links. Artifact cards show the file name and size; selecting a non-expired card starts a browser download instead of exposing a Runtime Workspace path.
- Failed and cancelled Runs do not create file Artifacts; their temporary Workspace changes are discarded.
- Artifact files expire after 90 days. The UI preserves metadata and reports that the file has expired.

### 8.2 Run History

The Run list shows:

- latest turn run time
- trigger type: manual, scheduled, or API
- latest turn state: queued, running, waiting_for_user, succeeded, failed, or cancelled
- queue position while the latest turn is queued
- latest turn elapsed time

Run Conversation rows are ordered by their latest turn time. Their stable identity remains the first Run ID so opening a row always loads the complete conversation.

Each Run History row represents a Run Conversation. Opening it replaces the Workflow detail content with a full-page conversation view rather than displaying a modal. Its detail uses the same conversation presentation as a Session: the frozen Workflow goal appears as the first User message, every follow-up input, attached image/file, and Run result appear as later User/Agent message pairs, and a composer remains available while the Workflow exists. API-triggered initial text or JSON remains visible with the goal when present. Run time, trigger, latest state, queue position when queued, accumulated elapsed time, and Artifacts remain visible as supporting metadata. Raw Workflow Snapshots, Runtime logs, and Runtime events are retained for platform operation but are not exposed as the primary User interface. A follow-up creates a new immutable Run turn using the initiating Workflow Snapshot and serialized Workspace; it never reopens a terminal Run. Users can cancel queued or active Runs and rerun a failed Run. Interrupt, Resume, Kill, Recovery, Attempt, Lease, and Sandbox diagnostics are not product controls.

Run metadata and final text/JSON results are retained without a time limit in the first version. Users cannot delete individual Run records.

### 8.3 Workflow Deletion

- Deleting a Workflow requires confirmation and permanently deletes its mutable configuration, API credential, schedule, Workspace, private Git key, and ability to run.
- Run metadata, final results, and unexpired Artifacts remain available through the Workflows list's `Deleted Records` filter.
- A Deleted Workflow Record is read-only, cannot be restored, and cannot be run again.

### 8.4 Knowledge Bases

- Knowledge Bases are a top-level authenticated product area. A User may create private Knowledge Bases; the Administrator may create private or public Knowledge Bases. Public Administrator Knowledge Bases are readable, searchable, and downloadable by every authenticated User, but only the Administrator may mutate them.
- A Knowledge Base contains optional first-level Categories and Knowledge Documents. A document belongs to at most one Category or remains unclassified. The same normalized source may be uploaded again in another Category as a separate document; within one Category, replacement creates a new immutable Document Revision.
- Supported sources are PDF, DOC/DOCX, XLS/XLSX, Markdown, TXT, PNG, JPEG, WebP, and one-time public HTTP(S) URL snapshots. Unsupported or unsafe sources are rejected. The existing 100 MiB upload limit applies; configurable defaults are 10,000 documents, 10 GiB per Knowledge Base, and 1,000 ingestion revisions per day.
- Object persistence and validation make an upload accepted immediately. Parsing, OCR, chunking, and embedding/indexing run asynchronously through durable Ingestion Jobs with `accepted`, `processing`, `ready`, `failed`, and `blocked` states. Failed revisions remain downloadable and retryable; only ready revisions participate in retrieval, and a failed refresh does not displace the previous ready revision.
- A Workflow may bind multiple Knowledge Bases. The Workflow Snapshot freezes the selected Knowledge Index Generation when a Run is created; Categories are management and citation boundaries, not Workflow selection controls. Manual, scheduled, follow-up, and API Runs use the same selection.
- Before the Runtime model call, the platform queries each frozen generation using the Workflow goal plus the current Run input. It returns a bounded, deduplicated Retrieval Context with source citations. No-hit continues with an explicit marker; unavailable provider, revoked access, or missing generation fails the Run rather than silently omitting knowledge.
- Run history retains bounded, redacted Knowledge Citation summaries. Opening a source re-checks current permission. Deleting or privatizing a base revokes future retrieval and downloads without rewriting completed Run history.
- Deleting a Knowledge Base, Category, or document immediately removes it from listings and retrieval, cancels active ingestion, and schedules idempotent cleanup. A 30-day tombstone permits restore before permanent object/index deletion.
- The UI exposes a top-level Knowledge Bases entry with category/unclassified document management, upload and URL import, ingestion status, failure reasons, retry, restore, and deletion confirmation. Workflow settings expose only multi-select Knowledge Base binding.

## 9. Experts, Skills, And Connectors

The detailed profile and management behavior and implementation/test seams are defined in `docs/product/expert-skill-connector-simplification.md`. Catalog details, launch actions, and conversation resource selection are defined in `docs/product/conversation-resource-selection.md`.

### 9.1 Experts

- An Expert contains a preset Profile Icon, a unique-per-User name, required display-only Introduction, required Core Capability, required Operating Procedure, required Output Standard, optional Cautions, Derived Expertise Tags, and selected Skills and Connectors. It contains no Provider Model or Runtime Engine setting.
- The interface labels Operating Procedure as `工作流程`; domain and API contracts use `operating_procedure` so it is not confused with the executable Workflow aggregate.
- Core Capability, Operating Procedure, Output Standard, and Cautions are assembled under fixed visible headings as the complete injected Expert guidance. Introduction and Derived Expertise Tags are never injected.
- Up to five Derived Expertise Tags are generated asynchronously from Core Capability with the User's Personal Settings default Provider Model. They are non-authoritative, consume no User Credits, and never block Expert save or execution; a failed refresh retains the previous projection.
- Expert edit uses a dedicated page. A User selects visible private and Platform Skills and Connectors and may install or upload a Skill, or create and test an MCP Connector, inline; the resulting resource enters that User's private catalog, or the platform catalog when created by the Administrator, and is selected automatically. New Expert creation is conversational through the immutable `Create Expert` Skill.
- A single Expert executes directly rather than through a coordinator or extra synthesis call. A Session or Workflow may also run without an Expert or Expert Team.
- Editing an Expert affects new selections, not retained selections or historical execution snapshots. An Expert referenced by a mutable Expert Team cannot be deleted; immutable historical snapshots do not block deletion.
- A migrated Expert missing Introduction, Core Capability, Operating Procedure, or Output Standard is an Incomplete Expert. It remains visible and editable but cannot be selected for a new Session or Run Conversation until completed.

### 9.2 Expert Teams

- An Expert Team contains a preset Profile Icon, a unique-per-User name, required display-only Introduction, required display-only Core Capability, and two to ten ordered Team Members.
- Each Team Member has a stable internal identity, a team-unique User-authored name, an Expert reference, zero to five Member Labels of at most twenty characters, and an order. The same Expert may appear in multiple member roles, whose Native Sessions and Runtime contexts remain isolated by stable Team Member identity.
- The Team Member name and labels are injected visibly before that member's Expert guidance. Team Introduction and team Core Capability are display-only.
- The Experts entry has `Experts` and `Expert Teams` tabs with separate create actions. The Expert catalog separates Administrator-created `Platform Experts` from the current User's `My Experts`; an Administrator-created Expert never appears under `My Experts`. Expert cards emphasize Introduction, Derived Expertise Tags, and Skill and Connector counts; team cards show ordered member roles and `N Experts per turn`. Neither card type displays model or Runtime settings.
- Team members support drag reorder plus accessible move-up and move-down controls. An Expert or Expert Team selection uses grouped options; a team with fewer than two valid members remains editable but is disabled in selectors.
- Each Session response or Run selecting a team executes that turn's full frozen member order sequentially and fail-fast. Every member uses the Session or Run Conversation's one frozen Personal Settings execution configuration; Runtime-native subagent capability is not required.
- Every Subagent receives the current task, Rolling Summary and recent messages, current attachments and File References, all preceding members' final text, its visible Team Member role context, and its own Expert guidance and default resources. Explicit composer Skill and Connector additions reach every member; explicit Connector exclusions also apply. Another member's default resources are not implicitly shared. Raw reasoning, tool logs, and private Runtime events are not collaboration context.
- Workflow team execution retains the shared temporary Workspace, success-only merge, final-member official response, atomic Native Session promotion, cancellation behavior, and full-team retry defined by ADR-0022 and ADR-0026.
- Deleting an Expert Team clears it from mutable Workflows and unstarted Sessions. Existing Session and Run Conversation snapshots remain executable.

### 9.3 Skills And MCP Connectors

- The Skills & Connectors entry has `Skills` and `Connectors` tabs. Each catalog separates Administrator-created Platform Resources from the current User's `My Skills` or `My Connectors`. Administrator-created resources are visible to every User and never appear in a `My` section. The Connectors tab presents MCP and available Third-party CLI Connectors together under Platform Connectors without protocol-specific sections; `New Connector` asks for the Connector type before showing the permitted configuration. Connector cards omit aggregate User, setup, and authorization counts; clicking a card opens its details, where an authorized owner or Administrator can enter editing. The User-visible Extension concept and resource management in Personal Settings are removed.
- Users install private Skills from a Git URL with an optional branch or from a ZIP upload, while Skills installed by the Administrator become platform-wide. A valid package contains `SKILL.md`, uses its required `display_name` as the catalog name, and may include scripts and resources. Skill cards use the description matching the current interface language, such as `description_zh` for Chinese, with the default `description` as fallback. Clicking a card opens a dedicated in-site detail page whose Back action restores the preceding catalog page; the detail page separates parsed frontmatter from the rendered document. New selections resolve the latest available exact revision; retained selections and historical Response Snapshots keep their frozen revisions. The catalog launch action opens a new Session with the Skill selected.
- The platform seeds two immutable default Skills named `Create Skill` and `Create Expert`. They are always visible as Platform Skills and cannot be edited or deleted. The Skills add action offers `Create Skill` and `Upload Skill`; `Create Skill` opens a new Session with the default Skill selected and prefilled (but unsent) prompt `请帮我创建一个可以实现「……」的技能`. The Experts create action opens a new Session with `Create Expert` selected and prefilled (but unsent) prompt `帮我创建一个 XXX 专家，擅长 XXXXX。我的经验是：[请补充你的行业背景、相关经验]`. Expert creation is conversational; the standalone new-Expert form is not an entry point. A confirmed conversational creation installs a private Skill or Expert for an ordinary User and a Platform Resource for an Administrator. Private names are unique per owner, Platform names are reserved across owners, and different Users may use the same private name.
- A conversational creation proposal is a backend-validated, persisted Action with a 15-minute expiry and one-use state transition. The Session shows a read-only preview and explicit Confirm/Cancel controls; revisions happen by sending another message. Confirmation is idempotent, creates no partial resource, and never accepts frontend-authored resource payloads.
- ZIP uploads allow archives up to 50 MiB and show a visible size-limit error when exceeded. They accept `SKILL.md` at the archive root or inside one enclosing top-level Skill folder. Installation removes that enclosing folder and macOS archive metadata (`__MACOSX`, `.DS_Store`, and `._*` files) before freezing the package; ambiguous folders, unsafe paths, and symbolic links are rejected.
- Users create, edit, test, and delete private MCP Connectors; MCP Connectors created by the Administrator become platform-wide and remain mutable only by the Administrator. Supported transports are Streamable HTTP and fixed-version `npx` or `uvx` stdio; `latest`, arbitrary host commands, and untested selection are rejected.
- MCP and CLI Connectors accept a preset icon or a validated PNG, JPEG, WebP, or GIF upload. The selected icon is shown consistently in the Connector catalog, details, Expert bindings, and Session/Run composer selections.
- MCP and Skill execution occurs only inside the isolated Runtime environment. An npm package speaking MCP remains an MCP Connector rather than a Third-party CLI Connector.
- Before deletion, the product shows affected mutable Experts. Confirmation transactionally detaches a private resource from its owner's Experts or a Platform Resource from every referencing Expert; historical snapshots remain unchanged.
- New public routes are `/api/v1/skills` and `/api/v1/connectors/mcp`; deprecated `/extensions` aliases are not retained.

### 9.4 Third-party CLI Connectors

- Only an Administrator may create or edit a platform-wide CLI Connector Definition. Available and disabled Definitions may be edited and republished; Definitions being built or tested remain locked until that operation reaches a terminal state. Ordinary Users may browse, enable, authorize, and select available Definitions but cannot create them; each enablement and authorization remains private to its User.
- The Administrator installation form contains an optional preset or uploaded icon, name, capability description, and installation method. The method accepts an exact npm package specification or a ZIP upload whose root contains `package.json`; saving immediately starts the immutable build and conformance flow. Integrity, executable, supported architecture, built-in authentication driver, structured capabilities, identities, argument allowlists, risk, required scopes, Egress, and supported Runtime image Digests are derived from trusted built-in profiles or validated package metadata rather than entered as form fields. Arbitrary Shell install or authentication scripts are prohibited.
- The platform builds a credential-free immutable bundle outside User Runs, verifies its integrity and SHA-256, stores it in private object storage, and mounts it read-only. A User Run never installs a package.
- Definition lifecycle is `draft`, `building`, `testing`, `available`, `failed`, or `disabled`. Availability requires Conformance for the exact bundle digest and each claimed Runtime RepoDigest; upstream schema additions require explicit review before a new version becomes available.
- Draft installation metadata may leave authentication unresolved until the Builder derives it; an unresolved authentication driver cannot enter `available`. Reinstalling the same bundle updates its exact Bundle/Runtime Conformance result without a duplicate-record failure, and the catalog only reports evidence for the current bundle.
- Administrators see installation, editing, platform disablement, and confirmed deletion actions; personal enablement and account authorization are User actions. Request failures appear visibly in the catalog, and installation and enablement indicate progress while preventing duplicate submissions.
- The shared Skills and Connectors component presents action failures directly, including when embedded in an Expert editor. Localized messages distinguish expired login, forbidden access, missing resources, stale versions, invalid input, rate limits, server failures, and network failures without displaying raw server errors. Error notices remain visible above active dialogs until dismissed or the action is retried; failed submissions retain their form inputs. Background status failures preserve the last usable content and show one inline warning instead of repeated pop-ups; successful polling clears its warning.
- Confirmed Administrator deletion removes the Definition from catalogs, disables all of its User enablements, removes pending authorization attempts and stored account tokens, and transactionally detaches it from mutable Experts. Historical snapshots and Artifacts remain unchanged. Deleted Definitions cannot be edited or published; their name and exact npm package can be installed again. The User's Feishu CLI Application is retained and reused when enabling its replacement, while account authorization must be granted again.
- A common CLI Connector Wrapper, rather than each Runtime Driver, enforces the frozen executable contract, arguments, identity, scopes, Egress, authorization, risk, timeout, output, Workspace, and Secret policy.
- Recommended Skills are explicit install offers that become ordinary User-owned Skills. Enabling a Connector never injects hidden instructions.

### 9.5 Feishu CLI And User Action Waits

- The first CLI Connector uses a fixed version of the official `@larksuite/cli` package. Browsing requires no authorization; enabling idempotently creates exactly one Feishu CLI Application per Agent Workspace User, while allowing multiple isolated Feishu account authorizations under that application.
- Clicking Enable opens the Feishu setup page automatically. After application registration completes, the same User-initiated flow starts account authorization and navigates the opened tab to Feishu authorization without another platform click. A retained application goes directly to account authorization. Manual account authorization also opens its returned page automatically. Visible continuation links remain available when popups are blocked, closed, or detached; resuming registration through its link continues the same sequence. Merely browsing the catalog does not launch authorization. Feishu-side consent remains an explicit User action.
- The platform stores the provider-returned application name and offers a Feishu developer-console link for optional manual renaming. It does not claim automatic application naming.
- Enablement requests only a reviewed subset of officially review-free, non-business scopes needed for identity and diagnostics. Business scopes are requested only when a capability is enabled or an operation requires them.
- Account authorization treats omitted empty capability Scope lists as empty, submits only deduplicated User-identity scopes, and never sends null Scope entries. Authorization validation failures show authorization-specific recovery guidance rather than installation-package advice.
- User and Bot execution identities remain distinct. Operations supporting both ask the User to choose; missing User scopes start explicit OAuth, while missing Bot scopes or publication prerequisites return a direct recovery link.
- Selecting a Feishu Connector does not request account authorization. When a Session or Run actually attempts a User-identity capability and lacks that capability's reviewed User scopes, the Conversation displays the OAuth action and direct recovery link beside the composer. After Feishu reports completion, the conversation asks the User to reply `已授权`; that reply creates the next turn that continues the prior request. Popup blocking or closing the authorization tab does not remove the direct link.
- App ID, App Secret, and Tokens are encrypted and write-only. Tokens refresh before use; failed refresh invalidates that authorization. Disconnecting an authorization or disabling a Connector blocks future commands immediately while preserving historical results.
- Every high-risk command requires a separate, persisted, one-use approval from the authenticated owning User. The request displays Connector, identity, operation, target, and redacted arguments, and binds the decision to an immutable command digest and nonce. Administrator identity and Workflow API credentials cannot approve.
- The global approval inbox checks immediately on entry and browser-tab return, every 30 seconds when empty, and every 5 seconds while approvals are pending. Hidden tabs pause polling; a slow request completes before the next one starts. A matching approval for the currently open Session appears beside its composer in the same recovery area as Connector authorization; approvals for other Sessions and Runs remain in the global inbox. This global check remains available from Settings so background executions can request approval.
- Only one approval is active per Execution Stage; further requests queue. A Session response or Run enters `waiting_for_user`, may be cancelled, retains its Runtime container, temporary Workspace, Workflow queue slot, and any active Execution Credit Reservation, and pauses its ordinary execution timeout while the approval deadline runs.
- The approval timeout defaults to five minutes, has a hard cap of fifteen minutes, and may be lowered by an Administrator. Scheduled and API Runs may wait for approval in the authenticated product; without User action they expire normally.
- Rejection or expiry returns a structured CLI error to the Runtime rather than forcing the whole execution to fail. Actual model Usage remains chargeable. Definition, enablement, authorization, scopes, and policy are revalidated after approval and immediately before command execution.

## 10. Personal Settings

- Personality choices are gentle-professional, direct-efficient, lively-friendly, and custom.
- A User may add personality guidance to any preset; custom requires guidance.
- The selected Personality applies globally to Session responses and Workflow Runs.
- Model configuration uses a platform-wide Model Catalog of Model Provider Connections and Provider Models rather than User-owned Model Profiles. Only the Administrator may create, update, refresh, or delete connections and manually add Provider Models; every authenticated User reads the same available catalog.
- A connection contains provider type, editable absolute HTTP or HTTPS Endpoint, supported Model API Protocols, and a write-only API Key. Built-in official Endpoints are prefilled; editing one marks the connection as a custom, unverified Endpoint. HTTP supports trusted private or self-hosted gateways; because it does not encrypt API Keys or model traffic in transit, the User is responsible for using it only on a trusted network.
- Initial built-ins are OpenAI, Anthropic, Google Gemini, xAI, DeepSeek, Alibaba Model Studio, Volcengine Ark, Moonshot, Zhipu, and MiniMax, plus a custom OpenAI-compatible connection.
- Built-in connections preset their supported protocols. A custom connection explicitly selects one or more of OpenAI Responses, OpenAI Chat Completions, and Anthropic Messages.
- Saving or refreshing a connection first requests the provider Endpoint's `/models` API. If that API is unsupported, fails, or returns no usable models, the platform loads its maintained default model list for that provider instead; a custom provider without maintained defaults remains available for explicit model entry.
- Administrator provider management lists the resulting models and provides both Refresh Models and Add Model actions. A fallback catalog is not presented as a provider error, while a connection with neither discovered nor default models shows the discovery failure and still permits manual model entry through one Model field. Ordinary Users have no provider or catalog mutation controls.
- Provider Models have one identifier used for invocation and selection. Provider-discovered display metadata may improve presentation, but the Administrator never configures a separate model name or model-type classification. Every available global model appears in every User's Personal Settings, subject only to Runtime Model Compatibility derived from the connection's Model API Protocols; Sessions, Workflows, Experts, and Expert Teams have no model selector.
- A Model Provider Connection referenced by any User's Runtime default or continuable Session or Run Conversation snapshot cannot be deleted until those references are changed or deleted.
- Runtime Settings shows only Claude Code, Codex, Hermes, OpenClaw, and PI Agent, their available state, and one default Provider Model per Runtime Engine. Creating a connection never changes these defaults automatically.
- Runtime Model Compatibility is verified, unverified, or incompatible. Unverified combinations show a non-blocking warning and remain selectable in Personal Settings; an incompatible pair cannot be saved as a Runtime default. An incompatible historical invocation fails explicitly without replacement.
- Personality controls communication style only and does not select a model.
- Historical Agent responses show the final execution stage's identity and expose every stage's Expert, connection, model identifier, and Runtime metadata on demand. A single stage whose final text exactly matches the Agent response is not rendered as a duplicate result card. Regeneration reuses the original ordered Response Snapshot.
- An unavailable selected Runtime fails explicitly and is never silently replaced.
- Personal Settings also stores language and time zone.

Personal Settings does not manage Skills or Connectors.

## 11. Security And Isolation

- User-owned resource queries and mutations enforce the authenticated User owner at the server and return non-enumerating not-found behavior across owners. Model Catalog and available CLI Connector Definition reads are global to authenticated Users, while their mutations require the Administrator.
- Login uses OIDC Authorization Code with PKCE. Workflow API credentials cannot authenticate ordinary product APIs.
- Secrets are never placed in URLs, command arguments, browser storage, logs, events, results, or Artifacts.
- External processes receive executable and argument arrays, never concatenated shell commands.
- Runtime, MCP, Skill, and CLI Connector code executes non-root in the isolated container path and cannot execute on the host. CLI Connector bundles are exact, immutable, read-only artifacts built outside User Runs.
- A direct CLI command passes through the common Wrapper and cannot start until current Definition, capability, identity, authorization, scope, Egress, argument, risk, timeout, and approval policy all pass. One-use approvals are bound to immutable command requests and revalidated immediately before execution.
- Workspace paths are normalized, symlinks cannot escape the root, object-store buckets remain private, and downloads use short-lived authorization.
- Writes use idempotency and optimistic concurrency where replay or concurrent editing could duplicate or overwrite intent.

## 12. Technical Constraints

- Frontend remains Vue 3 and TypeScript with responsive desktop/mobile behavior.
- Backend remains Go with DDD Domain/Application boundaries, Kratos transport, GORM, PostgreSQL, and strict YAML configuration.
- Protobuf remains the contract source for generated Go HTTP/gRPC, OpenAPI, and TypeScript types.
- API, Worker, PostgreSQL, MinIO, Keycloak, Caddy, Secret redaction, Object Storage, Runtime Adapters, Sandbox, Run/Event/SSE, and deployment foundations are retained and refactored.
- MinIO and Aliyun OSS remain supported Object Storage providers.

## 13. Replacement And Data Reset

This product replaces rather than hides the former control-plane model. Remove the following product code, APIs, schema, and UI after the replacement paths are ready:

- Coding Task, Issue Snapshot, Repository Binding, and Source Control Provider administration
- Agent Draft, validation, Release Approval, Agent Release, deprecation, and emergency block
- Review Branch delivery, quality gates, Push flow, and high-risk approval UI
- Organization, Team, Role Grant, four product roles, and Operations Console
- Agent Memory, Memory Candidate, Outbound Webhook, and the old Studio/Workspace/Operations pages

Retain only the lower-level Git Clone capability needed by Workflow Workspace initialization.

The original Agent Workspace cutover reset the former control-plane database. Later amendments use incremental migrations and preserve all current Sessions, Workflows, Runs, Experts, Skills, MCP configurations, and immutable snapshots. Migrate old Capability Introduction to Introduction and old Execution Instruction unchanged to Operating Procedure; leave Core Capability, Output Standard, and Cautions empty, so affected Experts remain visible but incomplete until edited. Stop reading legacy Expert Provider Model, Runtime Engine, and hand-authored tags for new execution, while retaining compatibility columns and historical snapshot readers. Give existing profiles default icons and migrate each old team position to a stable Team Member whose initial name is the referenced Expert name and whose Member Labels are empty. New public Expert, Skill, and Connector contracts have no deprecated `description`, `capability_introduction`, `execution_instruction`, or `/extensions` aliases.

## 14. Acceptance Boundary

Completion requires real browser-to-API closure for both Administrator and ordinary User flows, not interface previews. At minimum acceptance covers:

- account create, first-login password change, disable/enable/reset, and owner isolation
- default and per-User Daily Credit Allocation, time-zone boundary and cross-midnight settlement, non-carrying daily balance, persistent redeemed balance, negative-balance carry-forward, and rollout initialization
- Redemption Code batch generation, one-time plaintext handling, redemption concurrency, expiry and void behavior, generic invalid-code errors, reasoned Credit Adjustments, immutable Credit Ledger, projections, retention, and owner isolation
- versioned default and exact-match Model Credit Rates, one Credit per 10,000 Tokens at multiplier 1.00, separate input/output multipliers, explicit zero rates, fixed missing-Usage fallback, per-stage rate snapshots, incremental Token normalization, two-decimal rounding, and no historical recalculation
- positive-balance admission, Workflow-scoped FIFO queues with a five-queued-Run limit, cross-Workflow concurrency, stage-level Execution Credit Reservations, exact-once transactional settlement for success/failure/cancellation/timeout, Expert Team stage exhaustion, retry behavior, interactive/API/Scheduled insufficient-credit paths, and `429 insufficient_credits`/`429 queue_full`
- avatar Credit Balance and redemption panel, User Credit Ledger, per-response and per-Run total `共消耗` display without Token details, and Administrator Users/Model Rates/Redemption Codes tabs without access to User-owned execution detail
- Session create, image/file attachment upload and history, stream, retry, rename, archive, cancel archive, delete, Rolling Summary, one settings-derived execution configuration with or without an Expert, and capability-gated native Resume
- Workflow CRUD, manual/scheduled/API Run, follow-up image/file attachments, Workflow-scoped FIFO queueing with concurrent follow-ups, queue position and five-Run limit, cross-Workflow concurrency, cancellation, rerun, record detail, and deleted-record access
- AI Applications navigation and empty guidance; Smart Assistant and Digital Human CRUD, owner isolation, resource binding, snapshot behavior, deletion conflicts, conversation entry, FAQ answers, Answer Safety Policy, Knowledge Base retrieval, external iframe sharing, anonymous External Conversations, owner Credit charging, and rate limits; Image Creation navigation and empty guidance; Administrator Image Model and Prompt Optimization credentials, verification, and lifecycle settings; User text-to-image and image-to-image requests, original-metadata disclosure, model and parameter selection, Prompt Optimization, one-batch concurrency, cancellation, durable recovery, SSE progress, partial and unknown outcomes, regeneration, responsive results, preview/download/ZIP/reference reuse, history, expiry, deletion, owner isolation, reservations, exact settlement, and redacted logging
- read-only Workspace browse/preview/download, Git Settings Clone authentication/config validation, quotas, success merge, and failure rollback
- Artifact creation, preview/download, expiry, and post-Workflow deletion access
- private/public Knowledge Base ownership, Category and unclassified document management, multipart/URL ingestion, asynchronous parsing/OCR/indexing state, idempotent retry and restore, owner/public authorization, quotas, and source cleanup
- Workflow multi-Knowledge-Base binding, frozen Knowledge Index Generation, no-hit behavior, provider-failure fail-closed behavior, bounded source citations in Run history, and permission-checked source viewing
- Expert and Expert Team CRUD, preset icons, visible structured guidance, derived tags, responsive cards, search/tag filters, stable ordered Team Member editing, repeated Expert roles, incomplete states, grouped selection, snapshot behavior, and deletion conflicts
- real two-member shared-model and shared-engine sequential execution, isolated member contexts, visible member role injection, preceding-result and attachment handoff, streaming member progress, persisted stage identities and results, final-member response, fail-fast behavior, cancellation, whole-team retry, atomic Native Session promotion, shared temporary Workspace success merge, and failure rollback
- Skill and MCP Connector platform/private catalog grouping, Git/ZIP Skill installation, isolated MCP testing, inline creation from Expert editing, exact revision snapshots, affected-Expert deletion confirmation, private-owner isolation, and new non-Extension routes
- Administrator-only CLI Connector Definition creation, exact npm bundle build and integrity, reviewed capability schema, lifecycle and availability, User-private enablement and authorization, Wrapper enforcement, and exact bundle/Runtime Digest Conformance
- Feishu one-application-per-User idempotent enablement, multiple account authorizations, User/Bot identity, least-privilege scope recovery, encrypted credentials, token refresh and revocation, actual provider application name, and developer-console link
- high-risk one-use approvals, owning-User authority, immutable command binding, multiple serialized approvals, `waiting_for_user`, rejection, expiry, cancellation, resume, revalidation, Scheduled/API waiting, timeout pause, resource retention, Credit settlement, event ordering, and restart reconciliation
- Personal Settings, Administrator-managed global Model Provider Connections and catalogs, User-specific per-Runtime default model selections, no Session or Workflow execution override, Runtime compatibility warnings, Personality, language, and time zone
- Secret canary absence from browser, API, SSE, logs, Workspace persistence, and Artifacts
- Chinese/English, keyboard navigation, mobile layout, empty state, offline state, and explicit errors

Real Runtime and native Resume claims remain gated by the conformance evidence for the exact deployed Runtime image. Unsupported capabilities use the documented Rolling Summary fallback rather than fabricated evidence.

Image Generation completion additionally requires Repository, HTTP contract, frontend interaction, and fake-provider browser closure. Real OpenAI Images, Alibaba Model Studio image generation, and Prompt Optimization claims require separately executed live-provider gates with protected credentials; skipped or unavailable live tests are reported as missing evidence rather than passed Conformance.
