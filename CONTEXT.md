# Agent Platform

An internal enterprise platform for building, publishing, using, and operating governed Agents. The first product slice focuses on software engineering work.

## Language

**Agent Platform**:
The internal enterprise product that governs the full lifecycle of Agents, from configuration and release through execution and operations.
_Avoid_: Agent framework, multi-agent system

**Agent**:
A governed digital worker configured to pursue a defined goal using approved knowledge and capabilities.
_Avoid_: Bot, assistant, workflow

**Coding Agent**:
An Agent whose primary purpose is to inspect, change, and validate software in a governed code workspace.
_Avoid_: Code assistant, coding chatbot

**Coding Task**:
A unit of software engineering work delegated to a Coding Agent, normally originating from an issue or an explicit user request.
_Avoid_: Job, ticket, prompt

**Run**:
One governed execution of an Agent against a specific task and fixed configuration.
_Avoid_: Job, process, session

**Session**:
The continuing collaboration for one Coding Task, fixed to one Repository Binding, target branch, and task branch across multiple Runs.
_Avoid_: Chat, conversation, runtime session

**Attempt**:
One infrastructure execution of a Run, repeated only when the platform can safely retry without changing the user's intent.
_Avoid_: Retry, Run

**Code Workspace**:
The isolated working copy in which a Coding Agent reads, changes, and validates a repository during a Run.
_Avoid_: Sandbox, repository, working directory

**Agent Runtime**:
The configured execution engine that drives a Coding Agent during a Run.
_Avoid_: Model, sandbox, worker

**Runtime Adapter**:
The platform boundary that presents one Agent Runtime through the platform's common lifecycle and event contract.
_Avoid_: Runtime, provider, CLI wrapper

**Runtime Capability**:
A declared behavior an Agent Runtime can reliably provide through its Runtime Adapter.
_Avoid_: Feature flag, compatibility assumption

**Production Runtime**:
An Agent Runtime that has passed the platform's required conformance checks and is supported for production Agent Releases.
_Avoid_: Installed runtime, available CLI

**Credential Profile**:
A governed reference that authorizes a Run to use a platform-managed model or external service credential without exposing the underlying secret.
_Avoid_: API key, login, environment variables

**Configured Model**:
A model registered by a Platform Administrator and therefore eligible to process code from any Repository Binding.
_Avoid_: Approved model, trusted model

**Model Binding**:
The exact Configured Model, Endpoint, and Credential Profile frozen for one Run.
_Avoid_: Model policy, default model

**Egress Policy**:
The network boundary that permits public internet access while keeping private infrastructure and platform control services inaccessible to a Sandbox unless explicitly authorized.
_Avoid_: Firewall rule, internet access

**Model Budget**:
The maximum model usage or monetary cost authorized for a Run, constrained by progressively tighter organizational and Agent-level ceilings.
_Avoid_: Resource quota, execution limit

**Execution Limit**:
A platform-enforced safety or capacity boundary on execution resources such as time, concurrency, processes, memory, storage, or network use.
_Avoid_: Model budget, user budget

**Agent Release**:
An immutable, validated configuration that fixes an Agent's default Runtime and governed capabilities for future Runs.
_Avoid_: Agent version, deployment

**Source Control Provider**:
A governed integration with a supported source-control service, initially GitHub.com or a self-hosted GitLab instance.
_Avoid_: Git provider, repository

**Repository Binding**:
The repository-specific configuration that makes a reusable Agent eligible to work in one repository under its instructions, quality gates, and permissions.
_Avoid_: Agent installation, repository configuration

**Issue Snapshot**:
The immutable issue title, body, and optional link submitted by a user as input to a Coding Task.
_Avoid_: Live issue, synchronized issue

**Workspace Write Lease**:
The exclusive authority held by one Run to modify a Code Workspace while other Runs may continue read-only work.
_Avoid_: File lock, branch lock

**Runtime Subagent**:
An Agent Runtime's internal helper that remains inside one platform Run and shares its authority, budget, workspace, and lifecycle.
_Avoid_: Child Run, platform Agent

## Memory

**Working Memory**:
The transient plan and intermediate state used within one Run.
_Avoid_: Session history, checkpoint

**Session Memory**:
The messages, summaries, confirmed decisions, results, and workspace references that preserve continuity for one Coding Task across Runs.
_Avoid_: Chat history, Agent Memory

**Agent Memory**:
Stable, user-approved experience retained by one Agent across Coding Tasks.
_Avoid_: Repository instructions, User Memory

**Memory Candidate**:
An Agent-proposed fact or lesson that has no long-term effect until a user approves it as Agent Memory.
_Avoid_: Memory, inference

## Product Surfaces

**Agent Studio**:
The product surface where Agent Builders configure, validate, approve, publish, and evolve Agents.
_Avoid_: Admin console, agent editor

**Conversation Workspace**:
The primary product surface where Agent Users delegate Coding Tasks, collaborate with Agents, inspect changes, and make decisions.
_Avoid_: Chat page, playground

**Operations Console**:
The product surface where Run Operators inspect, intervene in, recover, and audit Runs.
_Avoid_: Dashboard, log viewer

**Review Branch**:
The task branch pushed by a Coding Agent for human review and optional manual creation of a pull or merge request.
_Avoid_: Draft pull request, final change

**Platform Administrator**:
A person responsible for platform-wide access, providers, policies, credentials, and operational configuration.
_Avoid_: Superuser, system owner

**Agent Builder**:
A person who configures, validates, publishes, and evolves an Agent without owning the platform itself.
_Avoid_: Agent developer, prompt engineer

**Agent User**:
A person who delegates software engineering work to a published Agent and reviews its results.
_Avoid_: End user, customer

**Run Operator**:
A person who monitors, diagnoses, intervenes in, and audits Agent Runs.
_Avoid_: Operator, SRE
