# Agent Workspace

A personal AI workspace for private conversations, reusable Workflows, Experts, Expert Teams, Extensions, and managed execution.

## People And Ownership

**Agent Workspace**:
The product in which an authenticated User creates private Sessions, configures Workflows and Experts, and runs Workflows through managed Runtime Engines.
_Avoid_: Coding Agent Platform, multi-agent system

**User**:
An authenticated person who exclusively owns their Sessions, Workflows, Experts, Extensions, Model Provider Connections, and Personal Settings.
_Avoid_: Organization member, Team member, product role

**Administrator**:
The single bootstrap identity that creates, disables, enables, and resets passwords for User accounts without access to User-owned content.
_Avoid_: Platform operator, Organization administrator, support user

## Conversations

**Session**:
A private, continuing text conversation owned by one User. It may use one Expert or Expert Team snapshot chosen before the first message, and it retains the User's current Provider Model selection.
_Avoid_: Workflow Run, Coding Task, runtime process

**Response Snapshot**:
The immutable Provider Model, Model Provider Connection identity, API Protocol, and Runtime Engine selected when one Session message is sent and reused when that response is regenerated. It records no API Key and is the execution identity shown with the resulting Agent response.
_Avoid_: Workflow Snapshot, current Session model, Native Session

**Archived Session**:
A read-only Session hidden from the active list until the User cancels its archive state.
_Avoid_: Deleted Session, completed Run

**Native Session**:
An opaque conversation identity maintained by one Runtime Engine to continue a product Session, including across Provider Model changes that keep the same Runtime Engine. It is an optional, conformance-gated optimization and never replaces platform-owned history.
_Avoid_: Session, Run, rolling summary

**Rolling Summary**:
The platform-maintained bounded summary that preserves Session continuity when history is too large, the Runtime Engine changes, or native Resume is unavailable.
_Avoid_: Agent Memory, user-authored note, Runtime checkpoint

## Workflows And Execution

**Workflow**:
A reusable executable configuration that combines a name, goal, optional Expert or Expert Team, environment, API access, schedule, and one persistent Workspace. It is a single execution definition, not a visual graph or arbitrary DAG.
_Avoid_: Pipeline, visual DAG

**Workflow Snapshot**:
The immutable copy of a Workflow's goal, optional Expert or Expert Team, Provider Model, Model Provider Connection version, API Protocol, Endpoint, Runtime Engine, Extensions, and environment used by one Run Conversation; its API Key is referenced through protected versioned credentials rather than copied into the ordinary snapshot.
_Avoid_: Published Workflow, Workflow release

**Workflow API Credential**:
The single API Key and write-only API Secret pair that authorizes an external caller to start and inspect one specific Workflow. Regeneration immediately invalidates the previous pair.
_Avoid_: User Token, Idempotency Key, model credential

**Scheduled Trigger**:
An optional hourly, daily, or weekly schedule that starts a Workflow from its fixed goal in a selected time zone.
_Avoid_: API call, Webhook, file watcher

**Run Conversation**:
A continuing conversation started by one manual, scheduled, or API Workflow trigger. It keeps the initiating Workflow Snapshot and contains one or more ordered Runs, so the User can follow up without losing the original goal, prior messages, or Workspace context.
_Avoid_: Session, one Runtime process, Run Event stream

**Run**:
One immutable execution turn inside a Run Conversation, with fixed input, one terminal result, and the Run Conversation's frozen Workflow Snapshot. The first Run records the trigger; each follow-up creates another Run instead of reopening a terminal Run.
_Avoid_: Workflow, Run Conversation, Session response, Worker process

**Deleted Workflow Record**:
The read-only name, Run history, and unexpired Artifacts retained after a Workflow and its Workspace are permanently deleted.
_Avoid_: Restorable Workflow, archived Workflow

## Files And Results

**Workspace**:
The persistent directory and file tree owned by one Workflow and reused across serialized Runs. A Run changes a temporary copy and merges it only on success.
_Avoid_: Session, Artifact, per-Run sandbox

**Git Source**:
The optional single public HTTPS or private SSH repository cloned into an empty Workspace root.
_Avoid_: Repository Binding, Source Control Provider, Review Branch

**Artifact**:
An immutable file added or changed by one successful Run and shown separately from the mutable Workspace. A Run's final text or JSON remains part of its Run Conversation and is not an Artifact.
_Avoid_: Run result, Workspace file, Run Event, temporary output

## Experts And Extensions

**Expert**:
A reusable specialist profile with a display name, display-only Capability Introduction, visible Execution Instruction, Expertise Tags, and selected MCP Servers and Skills. It executes directly when selected alone and as a Subagent when included in an Expert Team.
_Avoid_: Persona, Workflow

**Capability Introduction**:
The display-only explanation of an Expert or Expert Team's abilities. It is never injected into model instructions.
_Avoid_: Execution instruction, hidden prompt

**Execution Instruction**:
The visible, User-authored instruction that defines how an Expert performs work and is injected whenever that Expert executes.
_Avoid_: Capability Introduction, hidden prompt, Personality

**Expertise Tag**:
A short User-authored label describing an Expert or Expert Team's area of strength. Each profile may have up to ten tags, each no longer than twenty characters.
_Avoid_: Category, managed taxonomy

**Expert Team**:
A reusable named profile with a Capability Introduction, Expertise Tags, and an ordered list of two to ten distinct Experts, selectable anywhere a single Expert can be selected. Its configuration is snapshotted when a Session begins or Run Conversation starts.
_Avoid_: User team, organization, visual workflow

**Subagent**:
A platform-managed execution of one Expert in its own process and context inside an Expert Team. Runtime-specific native subagent support is not required for this behavior.
_Avoid_: Expert selected alone, simulated persona, Runtime capability

**Expert Team Execution**:
A fail-fast collaboration in which every Subagent receives the current task, bounded conversation context, attachments, and all preceding Subagent results, follows the team's member order, and shares the selected Provider Model and Runtime Engine. The final member produces the official response; retry restarts the whole collaboration.
_Avoid_: Parallel fan-out, arbitrary agent graph, coordinator synthesis

**Expert Snapshot**:
The immutable Expert or Expert Team definition used by one Session or Run Conversation, including member order, Execution Instructions, and exact Extension revisions. Deleting or editing the source profile does not change this snapshot.
_Avoid_: Current Expert, mutable team, Provider Model snapshot

**Incomplete Expert**:
A migrated Expert that has a Capability Introduction but no Execution Instruction. It remains editable and visible but cannot be selected for new execution until completed.
_Avoid_: Disabled Expert, deleted Expert

**Extension**:
A User-owned MCP Server or Skill that can be selected by an Expert.
_Avoid_: Runtime Engine, Expert, plugin marketplace

**MCP Server**:
An Extension reached through Streamable HTTP or started as a fixed-version `npx` or `uvx` stdio process inside an isolated Runtime environment.
_Avoid_: API Endpoint, Skill, Third-party CLI

**Skill**:
A versioned Extension package containing a required `SKILL.md` and optional scripts or resources, installed from a Git URL or uploaded archive. Scripts run only inside an isolated Runtime environment.
_Avoid_: Prompt, MCP Server, Runtime Engine

## Personal Configuration

**Personal Settings**:
A User's personality, default Runtime Engine, Runtime Engine Settings, language, and time zone.
_Avoid_: Organization policy, Expert configuration, Workflow settings

**Personality**:
One of the gentle-professional, direct-efficient, lively-friendly, or custom communication styles, optionally refined by a User-authored explanation.
_Avoid_: Expert, model system prompt, display name

**Model Provider Connection**:
A User-owned named connection containing a provider type, Endpoint, and write-only API Key. A User may keep multiple connections for the same built-in or custom OpenAI-compatible provider.
_Avoid_: Model Profile, Provider Model, Runtime Engine

**Model API Protocol**:
The wire contract exposed by a Model Provider Connection, such as OpenAI Responses, OpenAI Chat Completions, or Anthropic Messages. It is distinct from the provider brand, and one connection may expose more than one protocol.
_Avoid_: Model Provider, Endpoint, Runtime Adapter

**Provider Model**:
A model identifier under one Model Provider Connection, loaded from its `/models` API, the platform-maintained provider defaults, or explicit User configuration. The platform does not classify model types; every available Provider Model is selectable subject to Runtime Model Compatibility.
_Avoid_: Model Profile, provider connection, Runtime Engine

**Runtime Model Compatibility**:
The verified, unverified, or incompatible relationship between one Provider Model's API Protocol and one Runtime Engine. An unverified relationship warns but does not prevent selection; an incompatible invocation fails explicitly.
_Avoid_: Provider Model availability, Runtime Capability, provider verification

**Runtime Engine**:
The selected Claude Code, Codex, Hermes, or OpenClaw engine that generates a Session response or executes a Run.
_Avoid_: Provider Model, Expert, sandbox, Worker

**Runtime Engine Setting**:
A User's preference for one Runtime Engine, including that engine's default Provider Model. A Session initially selects this model and may retain another Provider Model without changing the default.
_Avoid_: global default model, Personality model, fixed Session model

**Runtime Adapter**:
The platform boundary that presents one Runtime Engine through the common execution and event contract.
_Avoid_: Runtime Engine, model provider, CLI wrapper

**Runtime Capability**:
A behavior that a specific Runtime image can reliably provide only after its required conformance evidence passes.
_Avoid_: Parsed field, configuration flag, compatibility assumption
