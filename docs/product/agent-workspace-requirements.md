# Agent Workspace Product Requirements

Status: accepted for implementation on 2026-08-25.

## 1. Product Outcome

Agent Workspace is a personal AI workspace centered on four user-visible areas: Sessions, Workflows, Experts, and Settings. It replaces the previous Coding Agent control-plane product rather than adding another layer on top of Agent Studio, Coding Tasks, Releases, and Operations.

The primary experience is intentionally lightweight: a ChatGPT-like left navigation and object list with one focused content surface. Advanced configuration remains collapsed until requested. The product name shown in the UI is `Agent Workspace`; the repository and deployment identifiers may remain `agent-platform`.

The UI supports Chinese and English. Language is stored in Personal Settings.

Transient operation success and failure feedback uses one standardized Toast pattern across product pages. Toasts remain above modal layers, carry the appropriate accessible live-region role, can be dismissed manually, and disappear automatically; persistent business content such as setup guidance and one-time credentials remains in the page.

## 2. Ownership And Accounts

- Every business resource is owned by one authenticated User and is invisible to other Users.
- Organization, Team, Role Grant, and the former four product roles are removed.
- One bootstrap Administrator account remains. The Administrator can also use their own private Sessions, Workflows, Experts, and Settings.
- The Administrator opens User Management from the avatar menu and can create, disable, enable, and reset passwords for ordinary User accounts.
- Account creation requires username, display name, and email. Keycloak generates a temporary password that is shown once and must be changed at first login.
- Keycloak remains the credential authority. The application stores only the User projection and never stores login passwords.
- The Administrator can see account metadata and state but cannot read or modify User-owned content.
- Disabling a User cancels running work, stops schedules, rejects Workflow API credentials, and prevents login.
- Accounts are not permanently deleted in the first version.

## 3. Navigation And First Use

The main navigation contains exactly:

1. Sessions
2. Workflows
3. Experts
4. Settings

Login opens Sessions. A User without a usable Model Provider Connection, Provider Model, or Runtime Engine default sees a three-step guide: connect a model provider, select a default Provider Model for a Runtime Engine, then start a Session or Workflow. Expert and Extension setup is optional and never blocks first use.

## 4. Sessions

### 4.1 Lifecycle

- A User can create, rename, archive, cancel archive, and permanently delete a Session.
- New Session creation optionally selects one Expert. A Session can start without an Expert and then uses Personal Settings directly.
- The Expert choice becomes fixed after the first message. Its MCP and Skill configuration is snapshotted at that point.
- The title is derived locally from the first User message and remains editable; title generation does not invoke a model.
- Archived Sessions are hidden from the active list and read-only until archive is cancelled.
- Deleting a Session requires confirmation, cancels active generation, and permanently deletes messages and Session execution data.
- Sessions support text only in the first version. They do not own a Workspace or accept file uploads.

### 4.2 Conversation Execution

- The composer has a searchable Provider Model selector in its lower-left corner, grouped by Model Provider Connection. A new Session starts with the current Runtime Engine's default Provider Model; a User selection persists for that Session.
- Each User message freezes a Response Snapshot containing the Runtime Engine, Model Provider Connection version, Model API Protocol, Endpoint, and Provider Model selected when Send is pressed. API Keys remain protected versioned credentials and never enter ordinary message data.
- While generation is active the selector is disabled. Switching Runtime Engines selects that engine's default Provider Model; switching only the Provider Model keeps the same Native Session when Resume is supported.
- A Session keeps its isolated Runtime container definition warm for 30 minutes after a response finishes. A later message using the same Runtime Engine reuses that container definition; switching Runtime Engines starts a separate warm container.
- The UI streams the response and shows generating, failed, cancelled, retry, and elapsed-time states. While a response is generating, the send control becomes a stop control that requests backend cancellation and stops the active Runtime execution. It does not expose Attempt, Lease, Runtime Event, or a separate Run record.
- While generating, the UI may show platform-derived progress stages such as preparing the Runtime, analyzing, using a tool, and composing the answer. It never exposes raw model chain-of-thought or private reasoning.
- Experts may use their snapshotted MCP Servers and Skills, but the first version returns text only.
- The platform maintains a Rolling Summary and recent-message window after every successful response.

### 4.3 Native Resume

- Claude Code and Codex may use native Session Resume only for Runtime images whose `native_resume` conformance evidence has passed.
- Hermes and OpenClaw use the Rolling Summary and recent messages for every response until their native Resume capability is independently verified.
- If the Runtime Engine name changes, the platform starts a new Native Session using the Rolling Summary.
- Runtime identity comparison deliberately uses only the Runtime Engine name. A CLI, Adapter, or image upgrade does not proactively invalidate the Native Session; a safe classified Resume failure may cause fallback.
- Changing the Provider Model while keeping the same Runtime Engine continues native Resume and passes the new model configuration to that Runtime. A Runtime rejection fails that response without losing platform history.
- Existing Sessions retain their initial Expert/MCP/Skill snapshot. Later Expert or Extension edits affect new Sessions and future Workflow Runs, not existing Sessions.
- Automatic fallback is allowed only when the Runtime reports that the checkpoint is invalid before any action executes. All other Resume failures are shown to the User and may be retried manually.
- Native Resume is an optimization. Platform messages and the Rolling Summary remain the correctness boundary.

## 5. Workflows

### 5.1 Definition And Lifecycle

- A Workflow is a single executable definition, not a visual DAG and not multi-agent orchestration.
- A User can create, edit, run, and delete a Workflow.
- Required fields are name and goal. Expert is optional.
- The selected Runtime Engine's default Provider Model applies unless the Workflow overrides it in advanced settings.
- The first Run in a Run Conversation freezes a Workflow Snapshot containing the goal, optional Expert and Extension revisions, Model Provider Connection version, Model API Protocol, Endpoint, Provider Model, Runtime Engine, and environment. Follow-up Runs reuse that frozen snapshot and add one immutable turn input. The API Key is held only through a protected versioned credential reference.
- A Workflow keeps its isolated Runtime container definition warm for 30 minutes after a Run finishes so frequent serialized Runs do not recreate the container. The container is stopped between Runs and destroyed only after 30 minutes without use.
- Editing a Workflow affects future Runs only. There is no Draft, Release, publish, approval, or visible version flow.
- Only one Run may modify a Workflow at a time. Additional manual, scheduled, and API requests enter the queue.

### 5.2 Detail Page

The Workflow detail page contains four tabs in this order:

1. Artifacts
2. Workspace
3. Run History
4. Settings

Settings contains five collapsed sections:

- Basic: name, goal, optional Expert
- Execution: optional Provider Model and Runtime Engine overrides, environment variables
- Schedule: hourly, daily, or weekly trigger with time and optional time-zone override
- API Credential: generate or regenerate API Key/API Secret and show usage examples
- Git Source: public HTTPS or private SSH Clone configuration

### 5.3 Triggers And Input

- Manual Runs launched from the Workflow interface execute the fixed Workflow goal directly; the detail header does not expose a separate input or input-type control. After creation, the interface immediately opens that Run Conversation and streams the active Run instead of requiring another click in Run History.
- API Runs accept optional text or JSON input.
- Scheduled Runs execute only the fixed Workflow goal and do not have a separate default input.
- Run trigger types shown to Users are manual, scheduled, and API.

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
- Authentication uses HTTP Basic Auth with API Key as username and API Secret as password.
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
- The tree view shows directories and files. Users can create a directory, upload a file, preview supported text, download a file, and clear the entire Workspace.
- Users cannot edit, rename, move, or individually delete Workspace entries in the first version.
- Clearing the Workspace requires destructive confirmation.
- Maximum Workspace size is 1 GB, maximum upload size is 100 MB, and maximum inline text preview size is 1 MB.
- The Workspace may be initialized by file upload or by cloning one Git Source into the empty root.
- Git supports public HTTPS and private SSH repositories. A private key belongs only to that Workflow, is write-only, and is destroyed with the Workspace.
- Clone requires an empty Workspace. A User may clear the Workspace before cloning.
- Runs operate on a temporary copy. A successful Run atomically advances the persistent Workspace; failed or cancelled Runs discard their file changes.
- Runtime and Skill processes access only the temporary Workspace, explicitly assigned environment, and allowed public network. They cannot access the host, platform private services, or another User's data.

## 7. Artifacts And Run History

### 7.1 Artifacts

- A successful Run produces an immutable final text or JSON result and captures files added or changed by that Run.
- Artifacts are grouped by Run time and can be previewed or downloaded.
- Failed and cancelled Runs do not create Artifacts; their temporary Workspace changes are discarded.
- Artifact files expire after 90 days. The UI preserves metadata and reports that the file has expired.

### 7.2 Run History

The Run list shows:

- start time
- trigger type: manual, scheduled, or API
- state: queued, running, succeeded, failed, or cancelled
- elapsed time

Each Run History row represents a Run Conversation. Its detail uses the same conversation presentation as a Session: the frozen Workflow goal appears as the first User message, every follow-up input and Run result appear as later User/Agent message pairs, and a composer remains available while the Workflow exists. API-triggered initial text or JSON remains visible with the goal when present. Run time, trigger, latest state, accumulated elapsed time, and Artifacts remain visible as supporting metadata. Raw Workflow Snapshots, Runtime logs, and Runtime events are retained for platform operation but are not exposed as the primary User interface. A follow-up creates a new immutable Run turn using the initiating Workflow Snapshot and serialized Workspace; it never reopens a terminal Run. Users can cancel the active Run and rerun a failed Run. Interrupt, Resume, Kill, Recovery, Attempt, Lease, and Sandbox diagnostics are not product controls.

Run metadata and final text/JSON results are retained without a time limit in the first version. Users cannot delete individual Run records.

### 7.3 Workflow Deletion

- Deleting a Workflow requires confirmation and permanently deletes its mutable configuration, API credential, schedule, Workspace, private Git key, and ability to run.
- Run metadata, final results, and unexpired Artifacts remain available through the Workflows list's `Deleted Records` filter.
- A Deleted Workflow Record is read-only, cannot be restored, and cannot be run again.

## 8. Experts And Extensions

### 8.1 Experts

- An Expert contains name, display-only description, selected MCP Servers, and selected Skills.
- The description is not injected into the model and does not act as a hidden instruction.
- A Session or Workflow may run without an Expert.
- Deleting an Expert clears it from Workflows, which then use Personal Settings. Sessions using that Expert retain their snapshot and are automatically archived read-only.
- Editing an Expert affects future Workflow Runs and new Sessions only.

### 8.2 MCP Servers

- Users can create, edit, test, and delete MCP Server configurations.
- Supported transports are Streamable HTTP and stdio.
- Streamable HTTP configuration includes name, URL, and write-only authentication values.
- stdio configuration uses fixed-version `npx` or `uvx` packages plus structured arguments and environment; `latest` and arbitrary host commands are rejected.
- MCP execution occurs only in the isolated Runtime environment.
- Only a successfully tested MCP Server may be attached to an Expert.
- Deleting an MCP Server removes it from all Experts. Historical Session and Run snapshots remain unchanged.

### 8.3 Skills

- Users install Skills from a Git URL or ZIP upload. A valid package contains `SKILL.md` and may include scripts and resources.
- Updating a Skill makes the newest revision available to Experts and future Runs. Each Run Snapshot records the exact SHA-256 used.
- Skill scripts execute only inside the isolated Runtime environment.
- Deleting a Skill removes it from all Experts. Historical Session and Run snapshots remain unchanged.

### 8.4 Third-party CLI

- Settings shows a Third-party CLI Extension tab with a `Coming Soon` state and no create action.
- Binary format, installation, permission, and execution semantics are intentionally undecided and out of scope.

## 9. Personal Settings

- Personality choices are gentle-professional, direct-efficient, lively-friendly, and custom.
- A User may add personality guidance to any preset; custom requires guidance.
- The selected Personality applies globally to Session responses and Workflow Runs.
- Model configuration uses User-owned Model Provider Connections rather than individual Model Profiles. One User may create multiple named connections for the same built-in or custom OpenAI-compatible provider.
- A connection contains provider type, editable absolute HTTP or HTTPS Endpoint, supported Model API Protocols, and a write-only API Key. Built-in official Endpoints are prefilled; editing one marks the connection as a custom, unverified Endpoint. HTTP supports trusted private or self-hosted gateways; because it does not encrypt API Keys or model traffic in transit, the User is responsible for using it only on a trusted network.
- Initial built-ins are OpenAI, Anthropic, Google Gemini, xAI, DeepSeek, Alibaba Model Studio, Volcengine Ark, Moonshot, Zhipu, and MiniMax, plus a custom OpenAI-compatible connection.
- Built-in connections preset their supported protocols. A custom connection explicitly selects one or more of OpenAI Responses, OpenAI Chat Completions, and Anthropic Messages.
- Saving or refreshing a connection first requests the provider Endpoint's `/models` API. If that API is unsupported, fails, or returns no usable models, the platform loads its maintained default model list for that provider instead; a custom provider without maintained defaults remains available for explicit model entry.
- Provider management lists the resulting models and provides both Refresh Models and Add Model actions. A fallback catalog is not presented as a provider error, while a connection with neither discovered nor default models shows the discovery failure and still permits manual model entry through one Model field.
- Provider Models have one identifier used for invocation and selection. Provider-discovered display metadata may improve presentation, but Users never configure a separate model name or model-type classification. Every available model appears in Session, Runtime, and Workflow selectors, subject only to Runtime Model Compatibility derived from the connection's Model API Protocols.
- A Model Provider Connection referenced by a Runtime default, current Session, or Workflow cannot be deleted until those references are changed.
- Runtime Settings shows only Claude Code, Codex, Hermes, and OpenClaw, their available state, and one default Provider Model per Runtime Engine. Creating a connection never changes these defaults automatically.
- Runtime Model Compatibility is verified, unverified, or incompatible. Unverified combinations show a non-blocking warning and remain selectable; an incompatible invocation fails explicitly and is never silently replaced.
- Personality controls communication style only and does not select a model.
- Historical Agent responses show the effective model and expose connection, model identifier, and Runtime metadata on demand. Regeneration reuses the original Response Snapshot.
- An unavailable selected Runtime fails explicitly and is never silently replaced.
- Personal Settings also stores language and time zone.

Settings presents Extensions as three tabs: MCP, Skills, and the Third-party CLI placeholder.

## 10. Security And Isolation

- All queries and mutations enforce the authenticated User owner at the server and return non-enumerating not-found behavior across owners.
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

Deployment resets the business database instead of migrating legacy Agent/Run data. Preserve a pre-cutover backup and rollback source. Keep the current `platform-admin` Keycloak identity as the sole Administrator, delete the three role test identities, and clear all former business data.

## 13. Acceptance Boundary

Completion requires real browser-to-API closure for both Administrator and ordinary User flows, not interface previews. At minimum acceptance covers:

- account create, first-login password change, disable/enable/reset, and owner isolation
- Session create, stream, retry, rename, archive, cancel archive, delete, Rolling Summary, Engine switch, and capability-gated native Resume
- Workflow CRUD, manual/scheduled/API Run, queueing, cancellation, rerun, record detail, and deleted-record access
- Workspace upload, directory create, clear, public/private Clone, quotas, success merge, and failure rollback
- Artifact creation, preview/download, expiry, and post-Workflow deletion access
- Expert optionality, MCP test gate, Skill install/update/delete, and snapshot behavior
- Personal Settings, Model Provider Connections and catalogs, per-Runtime default models, Session model switching and snapshots, Runtime compatibility warnings, Personality, language, and time zone
- Secret canary absence from browser, API, SSE, logs, Workspace persistence, and Artifacts
- Chinese/English, keyboard navigation, mobile layout, empty state, offline state, and explicit errors

Real Runtime and native Resume claims remain gated by the conformance evidence for the exact deployed Runtime image. Unsupported capabilities use the documented Rolling Summary fallback rather than fabricated evidence.
