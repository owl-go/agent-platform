# Agent Workspace Product Requirements

Status: accepted for implementation on 2026-08-25; Expert Team amendment accepted on 2026-09-02; Expert-owned execution and Credits amendments accepted on 2026-09-04.

## 1. Product Outcome

Agent Workspace is a personal AI workspace centered on four user-visible areas: Sessions, Workflows, Experts, and Settings. It replaces the previous Coding Agent control-plane product rather than adding another layer on top of Agent Studio, Coding Tasks, Releases, and Operations.

The primary experience is intentionally lightweight: a ChatGPT-like left navigation and object list with one focused content surface. Advanced configuration remains collapsed until requested. The product name shown in the UI is `Agent Workspace`; the repository and deployment identifiers may remain `agent-platform`.

The UI supports Chinese and English. Language is stored in Personal Settings.

Transient operation success and failure feedback uses one standardized Toast pattern across product pages. Toasts remain above modal layers, carry the appropriate accessible live-region role, can be dismissed manually, and disappear automatically; persistent business content such as setup guidance and one-time credentials remains in the page.

## 2. Ownership And Accounts

- Every business resource is owned by one authenticated User and is invisible to other Users.
- Organization, Team, Role Grant, and the former four product roles are removed.
- One bootstrap Administrator account remains. The Administrator can also use their own private Sessions, Workflows, Experts, and Settings.
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
- The displayed Credit Balance is the sum of the remaining Daily Credit Allocation and Redeemed Credit Balance. A User whose balance is not positive cannot start another Session response or Workflow Run.
- Every actual Provider Model invocation consumes Credits, including every Subagent invocation in an Expert Team. Validation, queueing, Runtime preparation, and MCP testing do not consume Credits.
- At a multiplier of 1.00, one Credit represents 10,000 input or output Tokens. Credit Consumption is calculated as `input Tokens / 10000 * input multiplier + output Tokens / 10000 * output multiplier`, rounded to two decimal places per invocation with a minimum non-zero charge of 0.01 Credits.
- Each invocation freezes its Model Credit Rate when the Session response or Workflow Run is queued. Later rate changes affect only subsequently queued invocations.
- Successful, failed, cancelled, and timed-out invocations consume their measured usage when available. An execution that fails before Provider Model usage does not consume Credits, and a retry is a new independently charged execution.
- Starting an invocation requires a positive Credit Balance. Because final usage is settled after execution, one invocation may make the balance negative; no further invocation may start until Credits become available again.
- Every Expert Team member rechecks Credit Balance before its own invocation. If a preceding member's settled usage makes the balance non-positive, the turn fails before the next member starts; completed invocations remain charged, unstarted members are not charged, and the turn's temporary Workspace and Native Session changes are discarded.
- Each terminal Assistant response displays that response's total Credit Consumption. Each terminal Workflow Run turn does the same; an Expert Team total is the sum of all member invocations that consumed Credits.
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
- Credit-consuming model invocations are serialized per User. Waiting invocations recheck the balance immediately before starting so concurrently submitted work cannot amplify a negative balance.
- A User may read their complete Credit Ledger. The Administrator can read account balance, today's usage, Daily Credit Allocation, redemption, and adjustment records, but cannot read execution-level consumption records or their Session, Workflow, model, or Token details.
- The Administrator's own executions follow the same credit rules. Administrator changes to their own allocation or persistent balance use the same immutable records as changes for an ordinary User.
- Runtime usage is normalized to the input and output Token delta for the current model invocation. Resumed native-session totals must not cause tokens from earlier responses to be charged again.
- A newly created User immediately receives the current Credit Day's full allocation. On feature rollout, every existing User receives the current day's allocation with no redeemed Credits.
- A model invocation belongs to the Credit Day in effect when that invocation starts, even if it settles after midnight. The next day's allocation offsets any resulting negative balance rather than changing the invocation's original day.
- The current Credit Day's allocation is materialized transactionally on the first balance read or execution admission after its boundary. A uniqueness constraint for User and Credit Day prevents duplicate allocation without requiring a midnight batch job.
- Each invocation's terminal Execution Stage state, Credit Ledger consumption, and balance projections commit in one database transaction. A single-stage execution or the final Expert Team stage commits the Assistant Message or Run terminal state in that transaction; the execution identity and stage position make every settlement idempotent across Worker retries.
- Credit Ledger entries, Redemption Code records, and Model Credit Rate revisions are retained for the life of the User and cannot be deleted in the first version.
- The avatar menu displays the User's current Credit Balance and opens a Credit panel with total balance, today's allocation remaining, redeemed balance, today's usage, next allocation time, Redemption Code input, and the User's Credit Ledger. The first version has no low-balance notification or configurable alert threshold.
- Each terminal Assistant response and Workflow Run displays only its total Credit Consumption, such as `共消耗 ✧ 79.05`. Per-invocation Tokens, frozen multipliers, and fallback details are not shown in the conversation interface.
- User Management contains `Users`, `Model Rates`, and `Redemption Codes` tabs. The Users tab displays balance, Credits consumed today, and Daily Credit Allocation and supports reasoned adjustments; allocation changes take effect on the next Credit Day, and a value of zero disables future daily allocation.
- A Model Credit Rate may explicitly set its input multiplier, output multiplier, or missing-Usage fallback to zero to make matching usage free. An unmatched model always uses the versioned platform default rather than becoming free implicitly.
- Insufficient Credits use the public error code `insufficient_credits`. HTTP APIs return `429 Too Many Requests` with the current balance and next Daily Credit Allocation time, without exposing another account or internal rate data.

## 3. Navigation And First Use

The main navigation contains exactly:

1. Sessions
2. Workflows
3. Experts
4. Settings

Login opens Sessions. A User without a usable platform Model Provider Connection, Provider Model, or personal Runtime Engine default sees setup guidance: an Administrator must configure the platform Model Catalog, then the User selects a default Provider Model for a Runtime Engine before starting a Session or Workflow. Expert and Extension setup is optional and never blocks first use.

## 4. Sessions

### 4.1 Lifecycle

- A User can create, rename, archive, cancel archive, and permanently delete a Session.
- New Session creation succeeds immediately without an Expert. Before the first message, the composer offers a grouped `No Expert / Expert / Expert Team` selector without reopening a creation modal.
- A Session can use no specialist profile, one Expert, or one Expert Team. The choice becomes fixed after the first message as an Expert Snapshot containing the visible metadata and each Expert's Provider Model, Runtime Engine, Execution Instruction, ordered position, and exact MCP and Skill revisions.
- The title is derived locally from the first User message and remains editable; title generation does not invoke a model.
- Archived Sessions are hidden from the active list and read-only until archive is cancelled.
- Deleting a Session requires confirmation, cancels active generation, and permanently deletes messages and Session execution data.
- A Session message may contain text, up to ten images/files, or both. Each attachment is at most 100 MiB. Sessions do not own a persistent Workspace; the Runtime receives checksum-verified, read-only copies for that turn.

### 4.2 Conversation Execution

- The Session composer has no Provider Model or Runtime Engine selector. Without an Expert, every sent message resolves the current Personal Settings default Runtime Engine and that engine's default Provider Model; with an Expert or Expert Team, it uses the Provider Model and Runtime Engine frozen in the Session's Expert Snapshot.
- Each User message freezes a Response Snapshot containing one ordered Execution Stage Snapshot per actual Provider Model invocation. Every stage records its optional Expert identity, Model Provider Connection version, Model API Protocol, Endpoint, Provider Model, Runtime Engine, Execution Instruction, and exact Extensions; API Keys remain protected versioned credentials and never enter ordinary message data.
- A settings change affects the next message of a no-Expert Session but never changes a queued or active response. Editing an Expert affects only Sessions started afterward; regeneration always reuses the original Response Snapshot.
- A Session may keep each isolated Runtime container definition warm for 30 minutes after a response finishes. Warm reuse is scoped by Session, frozen Expert identity, and Runtime Engine, so team members never share execution context even when they use the same engine.
- The UI streams the response and shows generating, failed, cancelled, retry, and elapsed-time states. While a response is generating, the send control becomes a stop control that requests backend cancellation and stops the active Runtime execution. It does not expose Attempt, Lease, Runtime Event, or a separate Run record.
- While generating and after completion, the UI may show an expandable, persisted execution activity summary such as preparing the Runtime, public reasoning summaries, redacted tool commands, file updates, and composing the answer. It never exposes raw model chain-of-thought, private reasoning, raw Runtime events, or tool output.
- A single selected Expert executes directly using its visible Execution Instruction and snapshotted MCP Servers and Skills. An Expert Team uses the fixed sequential execution contract in Section 8.2.
- Ordinary Runtime text remains a message; only files explicitly uploaded by a User or actually generated by a Runtime are presented as files.
- The platform maintains a Rolling Summary and recent-message window after every successful response.

### 4.3 Native Resume

- Claude Code and Codex may use native Session Resume only for Runtime images whose `native_resume` conformance evidence has passed.
- Hermes, OpenClaw, and PI Agent use the Rolling Summary and recent messages for every response until their native Resume capability is independently verified.
- For a no-Expert Session, a Personal Settings change that changes the Runtime Engine starts a new Native Session using the Rolling Summary.
- Runtime identity comparison deliberately uses only the Runtime Engine name. A CLI, Adapter, or image upgrade does not proactively invalidate the Native Session; a safe classified Resume failure may cause fallback.
- For a no-Expert Session, a Personal Settings change that changes only the Provider Model may continue native Resume and passes the new model configuration to that Runtime. A Runtime rejection fails that response without losing platform history.
- Existing Sessions retain their initial Expert Snapshot. Later Expert, Expert Team, or Extension edits and deletion affect new Sessions and new Run Conversations, not an existing conversation's ability to continue.
- Each Expert Team member owns independent staged Native Session state. The platform promotes all member states only after the entire turn succeeds and discards every staged state on failure or cancellation; retry restarts the full frozen member order from the preceding successful turn.
- Automatic fallback is allowed when the platform can prove before execution that the local native state for a checkpoint is absent, or when the Runtime reports that the checkpoint is invalid before any action executes. All other Resume failures are shown to the User and may be retried manually.
- Native Resume is an optimization. Platform messages and the Rolling Summary remain the correctness boundary.

## 5. Workflows

### 5.1 Definition And Lifecycle

- A Workflow is a single executable definition, not a visual DAG. It may invoke one fixed-order Expert Team but does not support arbitrary multi-agent graphs.
- A User can create, edit, run, and delete a Workflow.
- Required fields are name and goal. One Expert or Expert Team is optional.
- A Workflow has no Provider Model or Runtime Engine override. Without an Expert it resolves Personal Settings defaults when a new Run Conversation starts; with an Expert or Expert Team it uses each frozen Expert's required Provider Model and Runtime Engine.
- The first Run in a Run Conversation freezes a Workflow Snapshot containing the goal, ordered Execution Stage Snapshots, environment, and other execution inputs. Follow-up Runs reuse that frozen snapshot and add one immutable turn input; API Keys are held only through protected versioned credential references.
- A Workflow keeps each isolated Runtime container definition warm for 30 minutes after a Run finishes so frequent serialized Runs do not recreate containers. Warm reuse is scoped by Run Conversation, frozen Expert identity, and Runtime Engine; containers stop between Runs and are destroyed after 30 minutes without use.
- Editing a Workflow affects future Run Conversations only; follow-up Runs reuse their initiating snapshot. There is no Draft, Release, publish, approval, or visible version flow.
- Only one Run may modify a Workflow at a time. Additional manual, scheduled, and API requests enter the queue.

### 5.2 Detail Page

The Workflow detail page contains four tabs in this order:

1. Artifacts
2. Workspace
3. Run History
4. Settings

Settings contains five collapsed sections:

- Basic: name, goal, optional Expert or Expert Team
- Execution: read-only resolved Expert execution summary when applicable, plus environment variables
- Schedule: hourly, daily, or weekly trigger with time and optional time-zone override
- API Credential: generate or regenerate API Key/API Secret and show the JWT exchange and Bearer invocation examples
- Git Source: URL, branch, public HTTPS/account-password/private-key authentication, safe local Git config, and optional Workflow-scoped SSH config

### 5.3 Triggers And Input

- Manual Runs launched from the Workflow interface execute the fixed Workflow goal directly; the detail header does not expose a separate input or input-type control. After creation, the interface immediately opens that Run Conversation and streams the active Run instead of requiring another click in Run History.
- API Runs accept optional text or JSON input.
- Scheduled Runs execute only the fixed Workflow goal and do not have a separate default input.
- Run trigger types shown to Users are manual, scheduled, and API.
- An unavailable selected Expert or Expert Team prevents manual and API requests before a Run is created. A Scheduled Trigger instead creates an explicit failed, uncharged Run identifying the unavailable dependency; a queued Run revalidates its frozen first stage before model invocation and fails uncharged if that dependency became unavailable.

### 5.4 Environment

- Workflow environment variables are either ordinary or Secret.
- Ordinary values may be viewed and edited.
- Secret values are write-only after submission and can only be replaced or deleted.
- Only variables explicitly assigned to the Workflow enter its execution environment.
- Secret values are removed from logs, events, final results, and Artifacts before persistence.

### 5.5 API Credential And Contract

- API access is opt-in. No credential is generated when the Workflow is created.
- A Workflow has one API Key/API Secret pair. Regeneration immediately revokes the old pair.
- The API Secret is shown once and stored only as a verifier, never as recoverable product data.
- HTTP Basic Auth with API Key as username and API Secret as password is accepted only by the token exchange endpoint. It returns a 15-minute JWT access token.
- Workflow invocation and inspection use `Authorization: Bearer <jwt_token>`. Regenerating the credential invalidates outstanding tokens because their signature key derives from the current credential verifier.
- `POST /api/v1/workflows/{workflowId}/runs` accepts text or JSON input and returns `202` with a Run ID.
- `GET /api/v1/workflows/{workflowId}/runs/{runId}` returns status and final result.
- SSE provides live Run events.
- `Idempotency-Key` prevents external retries from creating duplicate Runs.
- A Workflow credential authorizes only starting and inspecting that Workflow. It cannot access Settings, Workspace mutation, another Workflow, or User APIs.

### 5.6 Schedule

- A Workflow can have one optional hourly, daily, or weekly Scheduled Trigger.
- Personal Settings provides the default time zone. The Workflow schedule may override it.
- A queued scheduled Run waits behind the active Run for the same Workflow.
- Deleting a Workflow or disabling its owner stops future scheduling.

## 6. Workspace

- Every Workflow owns one persistent Workspace reused across Runs.
- The Workspace tab is read-only: its tree view shows directories and files and allows supported text preview and file download. It does not create directories, upload, edit, rename, move, or delete entries.
- Maximum Workspace size is 1 GB and maximum inline text preview size is 1 MB.
- Workspace initialization by Git Clone is configured only in Workflow Git Settings, not in the Workspace browser.
- Git supports public HTTPS, HTTPS username/password or token, and private SSH repositories. Passwords, tokens, and private keys belong only to that Workflow, are write-only, and are destroyed with the Workspace.
- Git config is stored as an ordered key/value list and restricted to a safe allowlist; command, credential-helper, include, URL rewrite, and transport override keys are rejected.
- A private SSH source may store one Workflow-scoped SSH config containing an exact `Host` alias plus allowlisted connection fields. During Clone, the API materializes it as `~/.ssh/config` inside an isolated temporary HOME and writes the private key under the configured `IdentityFile`; both are removed after the attempt. It never modifies the API host account's SSH config.
- SSH config accepts only `Host`, `HostName`, `User`, `Port`, `IdentityFile`, `IdentitiesOnly`, `ServerAliveInterval`, and `ServerAliveCountMax`. It rejects wildcard hosts, includes, match blocks, proxy or local commands, arbitrary identity paths, and every other directive. Host verification remains pinned to the Administrator-provided `known_hosts` file.
- Clone requires an empty Workspace.
- Runs operate on a temporary copy. A successful Run atomically advances the persistent Workspace; failed or cancelled Runs discard their file changes.
- Runtime and Skill processes access only the temporary Workspace, explicitly assigned environment, and allowed public network. They cannot access the host, platform private services, or another User's data.

## 7. Artifacts And Run History

### 7.1 Artifacts

- A successful Run persists its final text or JSON in the Run Conversation and captures files added or changed by that Run as Artifacts.
- Only actual generated or changed files are Artifacts. An ordinary text or JSON response does not create a synthetic file or Artifact.
- Artifacts are grouped by Run time and can be previewed or downloaded.
- Failed and cancelled Runs do not create file Artifacts; their temporary Workspace changes are discarded.
- Artifact files expire after 90 days. The UI preserves metadata and reports that the file has expired.

### 7.2 Run History

The Run list shows:

- latest turn run time
- trigger type: manual, scheduled, or API
- latest turn state: queued, running, succeeded, failed, or cancelled
- latest turn elapsed time

Run Conversation rows are ordered by their latest turn time. Their stable identity remains the first Run ID so opening a row always loads the complete conversation.

Each Run History row represents a Run Conversation. Opening it replaces the Workflow detail content with a full-page conversation view rather than displaying a modal. Its detail uses the same conversation presentation as a Session: the frozen Workflow goal appears as the first User message, every follow-up input, attached image/file, and Run result appear as later User/Agent message pairs, and a composer remains available while the Workflow exists. API-triggered initial text or JSON remains visible with the goal when present. Run time, trigger, latest state, accumulated elapsed time, and Artifacts remain visible as supporting metadata. Raw Workflow Snapshots, Runtime logs, and Runtime events are retained for platform operation but are not exposed as the primary User interface. A follow-up creates a new immutable Run turn using the initiating Workflow Snapshot and serialized Workspace; it never reopens a terminal Run. Users can cancel the active Run and rerun a failed Run. Interrupt, Resume, Kill, Recovery, Attempt, Lease, and Sandbox diagnostics are not product controls.

Run metadata and final text/JSON results are retained without a time limit in the first version. Users cannot delete individual Run records.

### 7.3 Workflow Deletion

- Deleting a Workflow requires confirmation and permanently deletes its mutable configuration, API credential, schedule, Workspace, private Git key, and ability to run.
- Run metadata, final results, and unexpired Artifacts remain available through the Workflows list's `Deleted Records` filter.
- A Deleted Workflow Record is read-only, cannot be restored, and cannot be run again.

## 8. Experts And Extensions

### 8.1 Experts

- An Expert contains a unique-per-User name, required display-only Capability Introduction, required visible Execution Instruction, required Provider Model, required Runtime Engine, zero to ten Expertise Tags, selected MCP Servers, and selected Skills. Tags are free-form, case-insensitively unique within the Expert, and limited to twenty characters each.
- Capability Introduction replaces the former description and is never injected into model input. Execution Instruction is the only Expert role definition injected into model input; the product does not create a hidden instruction from the Expert name or Capability Introduction.
- Expert create and edit use a dedicated page with Provider Model and Runtime Engine selectors and the same MCP, Skills, and Third-party CLI extension manager as Settings. The selected pair must not be incompatible; unverified pairs remain selectable with a persistent warning. Users may manage MCP Servers and Skills without leaving the page; newly installed Skills are selected for that Expert, while MCP Servers become selectable only after a successful test. The Third-party CLI tab remains the same non-actionable Coming Soon placeholder defined below.
- A single Expert executes directly rather than through a coordinator or extra synthesis call. It receives the current task, bounded conversation context, attachments, Personal Settings, its Execution Instruction, its Provider Model and Runtime Engine, and only its own snapshotted MCP Servers and Skills.
- A Session or Workflow may run without an Expert or Expert Team.
- Editing or deleting an Expert affects only future selection. Existing Session and Run Conversation Expert Snapshots continue normally. Deleting an Expert removes it from mutable Expert Teams; a team left with fewer than two members remains editable but cannot be selected or run.
- Expert names are unique among a User's Experts but do not conflict with Expert Team names.
- A migrated Expert missing an Execution Instruction, Provider Model, or Runtime Engine is an Incomplete Expert. A complete Expert whose model, connection, engine, or compatibility becomes unusable is an Unavailable Expert. Both remain visible and editable but cannot be selected for new execution, and neither ever falls back to another execution configuration.

### 8.2 Expert Teams

- An Expert Team contains a unique-per-User name, required display-only Capability Introduction, zero to ten Expertise Tags, and an ordered list of two to ten distinct Experts. It has no team-level Execution Instruction, MCP Servers, Skills, Provider Model, or Runtime Engine setting.
- The Experts page has `Experts` and `Expert Teams` tabs. Each tab has its own create action, name/introduction search, Expertise Tag filter, and newest-first ordering; no popularity or `Hottest` metric is introduced. Desktop uses a responsive three-column card grid and mobile uses one column. Expert cards show name, Capability Introduction summary, Provider Model, Runtime Engine, compatibility warning, availability, and tags; team cards additionally show each ordered member's model and engine plus `N Experts per turn` cost disclosure.
- Create and edit use dedicated pages. Team members support drag reorder plus accessible move-up and move-down controls. An Expert or Expert Team selection uses grouped options; an invalid team remains visible, shows `At least two Experts required`, and is disabled in selectors.
- Every new Session message or Run turn executes the full frozen member order. Each Subagent runs in its own process and context using its Expert's Provider Model and Runtime Engine; a team may mix both. Runtime-native subagent capability is neither required nor enabled by this design.
- Every Subagent receives the current task, Rolling Summary and recent messages, current attachments, and the final text results of all preceding members. It receives only its own Execution Instruction, MCP Servers, and Skills. Raw reasoning, tool logs, and private Runtime events are never passed or displayed as collaboration context.
- A Workflow Expert Team operates on one shared temporary Workspace for the whole turn, so later members see earlier file changes. The Workspace is merged only after every member succeeds; any failure or cancellation rolls back the whole turn.
- Execution is sequential and fail-fast. The UI streams the current member result and shows progress such as `2/4 · Architecture Expert is running`. Completed intermediate results persist with Expert identity, Provider Model, Runtime Engine, order, status, elapsed time, Credit Consumption, and final text; they are collapsed by default and can be expanded and copied. The final member's result is the official Agent response and exposes that final member's execution identity in its summary.
- Failure identifies the failing member and prevents later members from starting. Cancellation stops the current member and the remaining chain. Retry always restarts at the first member. The existing task timeout applies to the entire team, with each member using only the remaining time.
- Workflow API status returns the overall state and official final result together with an ordered member-stage result list containing each member's Provider Model and Runtime Engine. It does not expose separate child Runs or require callers to combine results.
- Every follow-up turn reruns the complete frozen team. Platform-owned Rolling Summary and recent messages provide correctness; independently staged per-member Native Sessions are an optimization committed only after the full turn succeeds.
- Deleting an Expert Team clears it from mutable Workflows and unstarted Sessions, which return to no Expert. Existing Session and Run Conversation snapshots remain executable. Deletion uses the standard non-native confirmation dialog and identifies the affected team.
- Version one intentionally excludes parallel execution, conditions, loops, dynamic routing, free-form Agent conversation, team-level Extensions, team-level Provider Model or Runtime Engine overrides, coordinator synthesis, and Runtime-native subagents.

### 8.3 MCP Servers

- Users can create, edit, test, and delete MCP Server configurations.
- Supported transports are Streamable HTTP and stdio.
- Streamable HTTP configuration includes name, URL, and write-only authentication values.
- stdio configuration uses fixed-version `npx` or `uvx` packages plus structured arguments and environment; `latest` and arbitrary host commands are rejected.
- MCP execution occurs only in the isolated Runtime environment.
- Only a successfully tested MCP Server may be attached to an Expert.
- Deleting an MCP Server removes it from all Experts. Historical Session and Run snapshots remain unchanged.

### 8.4 Skills

- Users install Skills from a Git URL or ZIP upload. A valid package contains `SKILL.md` and may include scripts and resources.
- Updating a Skill makes the newest revision available to Experts and future Runs. Each Run Snapshot records the exact SHA-256 used.
- Skill scripts execute only inside the isolated Runtime environment.
- Deleting a Skill removes it from all Experts. Historical Session and Run snapshots remain unchanged.

### 8.5 Third-party CLI

- Settings shows a Third-party CLI Extension tab with a `Coming Soon` state and no create action.
- Binary format, installation, permission, and execution semantics are intentionally undecided and out of scope.

## 9. Personal Settings

- Personality choices are gentle-professional, direct-efficient, lively-friendly, and custom.
- A User may add personality guidance to any preset; custom requires guidance.
- The selected Personality applies globally to Session responses and Workflow Runs.
- Model configuration uses a platform-wide Model Catalog of Model Provider Connections and Provider Models rather than User-owned Model Profiles. Only the Administrator may create, update, refresh, or delete connections and manually add Provider Models; every authenticated User reads the same available catalog.
- A connection contains provider type, editable absolute HTTP or HTTPS Endpoint, supported Model API Protocols, and a write-only API Key. Built-in official Endpoints are prefilled; editing one marks the connection as a custom, unverified Endpoint. HTTP supports trusted private or self-hosted gateways; because it does not encrypt API Keys or model traffic in transit, the User is responsible for using it only on a trusted network.
- Initial built-ins are OpenAI, Anthropic, Google Gemini, xAI, DeepSeek, Alibaba Model Studio, Volcengine Ark, Moonshot, Zhipu, and MiniMax, plus a custom OpenAI-compatible connection.
- Built-in connections preset their supported protocols. A custom connection explicitly selects one or more of OpenAI Responses, OpenAI Chat Completions, and Anthropic Messages.
- Saving or refreshing a connection first requests the provider Endpoint's `/models` API. If that API is unsupported, fails, or returns no usable models, the platform loads its maintained default model list for that provider instead; a custom provider without maintained defaults remains available for explicit model entry.
- Administrator provider management lists the resulting models and provides both Refresh Models and Add Model actions. A fallback catalog is not presented as a provider error, while a connection with neither discovered nor default models shows the discovery failure and still permits manual model entry through one Model field. Ordinary Users have no provider or catalog mutation controls.
- Provider Models have one identifier used for invocation and selection. Provider-discovered display metadata may improve presentation, but the Administrator never configures a separate model name or model-type classification. Every available global model appears in every User's Personal Settings and Expert selectors, subject only to Runtime Model Compatibility derived from the connection's Model API Protocols; Sessions and Workflows have no model selector.
- A Model Provider Connection referenced by any User's Runtime default, mutable Expert, or continuable Session or Run Conversation snapshot cannot be deleted until those references are changed or deleted.
- Runtime Settings shows only Claude Code, Codex, Hermes, OpenClaw, and PI Agent, their available state, and one default Provider Model per Runtime Engine. Creating a connection never changes these defaults automatically.
- Runtime Model Compatibility is verified, unverified, or incompatible. Unverified combinations show a non-blocking warning and remain selectable; an incompatible pair cannot be saved on an Expert. A saved Expert whose pair later becomes incompatible is unavailable for new execution, and an incompatible historical invocation fails explicitly without replacement.
- Personality controls communication style only and does not select a model.
- Historical Agent responses show the final execution stage's identity and expose every stage's Expert, connection, model identifier, and Runtime metadata on demand. Regeneration reuses the original ordered Response Snapshot.
- An unavailable selected Runtime fails explicitly and is never silently replaced.
- Personal Settings also stores language and time zone.

Settings presents Extensions as three tabs: MCP, Skills, and the Third-party CLI placeholder.

## 10. Security And Isolation

- User-owned resource queries and mutations enforce the authenticated User owner at the server and return non-enumerating not-found behavior across owners. Model Catalog reads are global to authenticated Users, while every Model Provider Connection and Provider Model mutation requires the Administrator.
- Login uses OIDC Authorization Code with PKCE. Workflow API credentials cannot authenticate ordinary product APIs.
- Secrets are never placed in URLs, command arguments, browser storage, logs, events, results, or Artifacts.
- External processes receive executable and argument arrays, never concatenated shell commands.
- Runtime, MCP, and Skill code executes non-root in the isolated container path and cannot execute on the host.
- Workspace paths are normalized, symlinks cannot escape the root, object-store buckets remain private, and downloads use short-lived authorization.
- Writes use idempotency and optimistic concurrency where replay or concurrent editing could duplicate or overwrite intent.

## 11. Technical Constraints

- Frontend remains Vue 3 and TypeScript with responsive desktop/mobile behavior.
- Backend remains Go with DDD Domain/Application boundaries, Kratos transport, GORM, PostgreSQL, and strict YAML configuration.
- Protobuf remains the contract source for generated Go HTTP/gRPC, OpenAPI, and TypeScript types.
- API, Worker, PostgreSQL, MinIO, Keycloak, Caddy, Secret redaction, Object Storage, Runtime Adapters, Sandbox, Run/Event/SSE, and deployment foundations are retained and refactored.
- MinIO and Aliyun OSS remain supported Object Storage providers.

## 12. Replacement And Data Reset

This product replaces rather than hides the former control-plane model. Remove the following product code, APIs, schema, and UI after the replacement paths are ready:

- Coding Task, Issue Snapshot, Repository Binding, and Source Control Provider administration
- Agent Draft, validation, Release Approval, Agent Release, deprecation, and emergency block
- Review Branch delivery, quality gates, Push flow, and high-risk approval UI
- Organization, Team, Role Grant, four product roles, and Operations Console
- Agent Memory, Memory Candidate, Outbound Webhook, and the old Studio/Workspace/Operations pages

Retain only the lower-level Git Clone capability needed by Workflow Workspace initialization.

The original Agent Workspace cutover reset the former control-plane database. The Expert Team and Expert-owned execution amendments use incremental migrations and preserve all current Sessions, Workflows, Runs, Experts, and Extensions. Existing Experts without an Execution Instruction, Provider Model, or Runtime Engine appear as Incomplete Experts and cannot be selected for new execution until edited. Legacy mutable Session and Workflow model or engine overrides are cleared and ignored for new execution; already-frozen snapshots keep their original shared configuration and are read through an explicit legacy schema path. The public Expert contract replaces `description` with `capability_introduction` and `execution_instruction` without a deprecated alias.

## 13. Acceptance Boundary

Completion requires real browser-to-API closure for both Administrator and ordinary User flows, not interface previews. At minimum acceptance covers:

- account create, first-login password change, disable/enable/reset, and owner isolation
- default and per-User Daily Credit Allocation, time-zone boundary and cross-midnight settlement, non-carrying daily balance, persistent redeemed balance, negative-balance carry-forward, and rollout initialization
- Redemption Code batch generation, one-time plaintext handling, redemption concurrency, expiry and void behavior, generic invalid-code errors, reasoned Credit Adjustments, immutable Credit Ledger, projections, retention, and owner isolation
- versioned default and exact-match Model Credit Rates, one Credit per 10,000 Tokens at multiplier 1.00, separate input/output multipliers, explicit zero rates, fixed missing-Usage fallback, per-stage rate snapshots, incremental Token normalization, two-decimal rounding, and no historical recalculation
- positive-balance admission, User-level serialized model invocation, exact-once transactional settlement for success/failure/cancellation/timeout, Expert Team stage exhaustion, retry behavior, interactive/API/Scheduled insufficient-credit paths, and `429 insufficient_credits`
- avatar Credit Balance and redemption panel, User Credit Ledger, per-response and per-Run total `共消耗` display without Token details, and Administrator Users/Model Rates/Redemption Codes tabs without access to User-owned execution detail
- Session create, image/file attachment upload and history, stream, retry, rename, archive, cancel archive, delete, Rolling Summary, settings-derived no-Expert execution, Expert-derived execution, and capability-gated native Resume
- Workflow CRUD, manual/scheduled/API Run, follow-up image/file attachments, queueing, cancellation, rerun, record detail, and deleted-record access
- read-only Workspace browse/preview/download, Git Settings Clone authentication/config validation, quotas, success merge, and failure rollback
- Artifact creation, preview/download, expiry, and post-Workflow deletion access
- Expert and Expert Team CRUD, required model and engine selection, responsive cards, search/tag filters, ordered member editing, incomplete and unavailable states, grouped selection, snapshot behavior, and deletion semantics
- real two-member mixed-model and mixed-engine sequential execution, preceding-result and attachment handoff, streaming member progress, persisted stage execution identities and results, final-member response, fail-fast behavior, cancellation, whole-team retry, atomic Native Session promotion, shared temporary Workspace success merge, and failure rollback
- Personal Settings, Administrator-managed global Model Provider Connections and catalogs, User-specific per-Runtime default model selections, no Session or Workflow execution override, Runtime compatibility warnings, Personality, language, and time zone
- Secret canary absence from browser, API, SSE, logs, Workspace persistence, and Artifacts
- Chinese/English, keyboard navigation, mobile layout, empty state, offline state, and explicit errors

Real Runtime and native Resume claims remain gated by the conformance evidence for the exact deployed Runtime image. Unsupported capabilities use the documented Rolling Summary fallback rather than fabricated evidence.
