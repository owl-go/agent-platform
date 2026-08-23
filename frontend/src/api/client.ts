import type { InjectionKey } from "vue";
import type { components } from "./generated";

export type Health = components["schemas"]["v1HealthResponse"];
export type CurrentUser = components["schemas"]["v1CurrentUser"];
export type RuntimeImage = components["schemas"]["v1RuntimeImage"];
export type RegisterRuntimeImageInput = components["schemas"]["v1RegisterRuntimeImageRequest"];
export type RuntimeImageStatusInput = components["schemas"]["RuntimeCatalogServiceChangeRuntimeImageStatusBody"];
export type CredentialProfile = components["schemas"]["v1CredentialProfile"];
export type ConfiguredModel = components["schemas"]["v1ConfiguredModel"];
export type RegisterCredentialProfileInput = components["schemas"]["v1RegisterCredentialProfileRequest"];
export type RegisterConfiguredModelInput = components["schemas"]["v1RegisterConfiguredModelRequest"];
export type SourceControlProvider = components["schemas"]["v1SourceControlProvider"];
export type RegisterSourceControlProviderInput = components["schemas"]["v1RegisterSourceControlProviderRequest"];
export type RepositoryBinding = components["schemas"]["v1RepositoryBinding"];
export type RepositoryBindingInput = components["schemas"]["v1RepositoryBindingInput"];
export type Agent = components["schemas"]["v1Agent"];
export type AgentDraft = components["schemas"]["v1AgentDraft"];
export type ReleaseApproval = components["schemas"]["v1ReleaseApproval"];
export type AgentRelease = components["schemas"]["v1AgentRelease"];
export type AgentConfiguration = components["schemas"]["v1AgentConfiguration"];
export type CodingTask = components["schemas"]["v1CodingTask"];
export type CodingTaskLaunchOption = components["schemas"]["v1CodingTaskLaunchOption"];
export type CodingTaskLaunchCatalog = { items: CodingTaskLaunchOption[]; prerequisite: string };
export type CodingTaskSession = components["schemas"]["v1Session"];
export type Run = components["schemas"]["v1Run"];
export type RunApproval = Omit<components["schemas"]["v1RunApproval"], "request"> & { request?: Record<string, unknown> };
export type Artifact = components["schemas"]["v1Artifact"];
export type ArtifactDownload = components["schemas"]["v1GetArtifactDownloadResponse"];
export type RunEvent = { run_id: string; sequence: number; event_type: string; payload: unknown; created_at: string };
export type RunEventStreamResult = { cursor: number; terminal: boolean };
export type CreateCodingTaskInput = Omit<components["schemas"]["v1CreateCodingTaskRequest"], "team_id">;
export type CodingTaskLaunch = components["schemas"]["v1CreateCodingTaskResponse"];
export type CreateAgentInput = Omit<components["schemas"]["v1CreateAgentRequest"], "team_id">;
export type DraftInput = { configuration: AgentConfiguration; release_risk: string };

export type ApiErrorKind = "unauthenticated" | "forbidden" | "not_found" | "conflict" | "validation" | "rate_limited" | "unavailable" | "unknown";

export class ApiError extends Error {
  constructor(
    public readonly kind: ApiErrorKind,
    public readonly status: number,
    public readonly code: string,
    public readonly requestID: string,
  ) {
    super(code || `request_failed_${status}`);
    this.name = "ApiError";
  }
}

export interface RuntimeImagePage {
  items: RuntimeImage[];
  nextPageToken: string;
}

export interface PlatformApi {
  listRuntimeImages(pageToken?: string, pageSize?: number, signal?: AbortSignal): Promise<RuntimeImagePage>;
  getRuntimeImage(id: string, signal?: AbortSignal): Promise<RuntimeImage>;
  registerRuntimeImage(input: RegisterRuntimeImageInput, idempotencyKey: string, signal?: AbortSignal): Promise<RuntimeImage>;
  changeRuntimeImageStatus(id: string, input: RuntimeImageStatusInput, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<RuntimeImage>;
  listCredentialProfiles(signal?: AbortSignal): Promise<CredentialProfile[]>;
  getCredentialProfile(id: string, signal?: AbortSignal): Promise<CredentialProfile>;
  registerCredentialProfile(input: RegisterCredentialProfileInput, idempotencyKey: string, signal?: AbortSignal): Promise<CredentialProfile>;
  changeCredentialProfileStatus(id: string, enabled: boolean, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<CredentialProfile>;
  listConfiguredModels(signal?: AbortSignal): Promise<ConfiguredModel[]>;
  getConfiguredModel(id: string, signal?: AbortSignal): Promise<ConfiguredModel>;
  registerConfiguredModel(input: RegisterConfiguredModelInput, idempotencyKey: string, signal?: AbortSignal): Promise<ConfiguredModel>;
  changeConfiguredModelStatus(id: string, enabled: boolean, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<ConfiguredModel>;
  listSourceControlProviders(signal?: AbortSignal): Promise<SourceControlProvider[]>;
  getSourceControlProvider(id: string, signal?: AbortSignal): Promise<SourceControlProvider>;
  registerSourceControlProvider(input: RegisterSourceControlProviderInput, idempotencyKey: string, signal?: AbortSignal): Promise<SourceControlProvider>;
  changeSourceControlProviderStatus(id: string, enabled: boolean, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<SourceControlProvider>;
  listRepositoryBindings(teamID: string, signal?: AbortSignal): Promise<RepositoryBinding[]>;
  getRepositoryBinding(id: string, teamID: string, signal?: AbortSignal): Promise<RepositoryBinding>;
  registerRepositoryBinding(input: RepositoryBindingInput, idempotencyKey: string, signal?: AbortSignal): Promise<RepositoryBinding>;
  updateRepositoryBinding(id: string, input: RepositoryBindingInput, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<RepositoryBinding>;
  validateRepositoryBinding(id: string, teamID: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<RepositoryBinding>;
  listAgents(teamID: string, signal?: AbortSignal): Promise<Agent[]>;
  getAgent(id: string, teamID: string, signal?: AbortSignal): Promise<Agent>;
  createAgent(teamID: string, input: CreateAgentInput, idempotencyKey: string, signal?: AbortSignal): Promise<Agent>;
  updateAgent(id: string, teamID: string, input: CreateAgentInput, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<Agent>;
  listAgentDrafts(agentID: string, teamID: string, signal?: AbortSignal): Promise<AgentDraft[]>;
  getAgentDraft(agentID: string, draftID: string, teamID: string, signal?: AbortSignal): Promise<AgentDraft>;
  createAgentDraft(agentID: string, teamID: string, input: DraftInput, idempotencyKey: string, signal?: AbortSignal): Promise<AgentDraft>;
  updateAgentDraft(agentID: string, draftID: string, teamID: string, input: DraftInput, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<AgentDraft>;
  validateAgentDraft(agentID: string, draftID: string, teamID: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<AgentDraft>;
  getAgentDraftApproval(agentID: string, draftID: string, teamID: string, signal?: AbortSignal): Promise<ReleaseApproval>;
  requestAgentDraftApproval(agentID: string, draftID: string, teamID: string, riskReason: string, idempotencyKey: string, signal?: AbortSignal): Promise<ReleaseApproval>;
  decideAgentDraftApproval(agentID: string, draftID: string, teamID: string, approved: boolean, reason: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<ReleaseApproval>;
  publishAgentDraft(agentID: string, draftID: string, teamID: string, idempotencyKey: string, signal?: AbortSignal): Promise<AgentRelease>;
  listAgentReleases(agentID: string, teamID: string, signal?: AbortSignal): Promise<AgentRelease[]>;
  getAgentRelease(agentID: string, releaseID: string, teamID: string, signal?: AbortSignal): Promise<AgentRelease>;
  deprecateAgentRelease(agentID: string, releaseID: string, teamID: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<AgentRelease>;
  blockAgentRelease(agentID: string, releaseID: string, teamID: string, reason: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<AgentRelease>;
  listCodingTaskLaunchOptions(teamID: string, signal?: AbortSignal): Promise<CodingTaskLaunchCatalog>;
  listCodingTasks(teamID: string, signal?: AbortSignal): Promise<CodingTask[]>;
  getCodingTask(id: string, teamID: string, signal?: AbortSignal): Promise<CodingTask>;
  createCodingTask(teamID: string, input: CreateCodingTaskInput, idempotencyKey: string, signal?: AbortSignal): Promise<CodingTaskLaunch>;
  getCodingTaskSession(taskID: string, teamID: string, signal?: AbortSignal): Promise<CodingTaskSession>;
  listRuns(teamID: string, taskID: string, signal?: AbortSignal): Promise<Run[]>;
  getRun(runID: string, signal?: AbortSignal): Promise<Run>;
  listRunApprovals(runID: string, signal?: AbortSignal): Promise<RunApproval[]>;
  decideRunApproval(approvalID: string, approved: boolean, reason: string, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<RunApproval>;
  controlRun(runID: string, action: "interrupt" | "resume" | "cancel", version: number, idempotencyKey: string, signal?: AbortSignal): Promise<Run>;
  streamRunEvents(runID: string, after: number, onEvent: (event: RunEvent) => void, signal?: AbortSignal): Promise<RunEventStreamResult>;
  listRunArtifacts(runID: string, signal?: AbortSignal): Promise<Artifact[]>;
  getArtifactDownload(artifactID: string, signal?: AbortSignal): Promise<ArtifactDownload>;
}

export const platformApiKey: InjectionKey<PlatformApi> = Symbol("agent-platform-api");

export async function getHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch("/api/healthz", { headers: { Accept: "application/json" }, signal });
  if (!response.ok) throw new Error(`Health check failed with status ${response.status}`);
  return (await response.json()) as Health;
}

export async function getCurrentUser(accessToken: string, signal?: AbortSignal): Promise<CurrentUser> {
  return request<CurrentUser>(accessToken, "/api/v1/me", { signal });
}

export function createPlatformApi(getAccessToken: () => string | undefined): PlatformApi {
  const authorizedRequest = <T>(path: string, init: RequestInit = {}) => {
    const token = getAccessToken();
    if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication", "");
    return request<T>(token, path, init);
  };

  return {
    async listRuntimeImages(pageToken = "", pageSize = 20, signal) {
      const query = new URLSearchParams({ page_size: String(pageSize) });
      if (pageToken) query.set("page_token", pageToken);
      const body = await authorizedRequest<components["schemas"]["v1ListRuntimeImagesResponse"]>(`/api/v1/runtime-images?${query}`, { signal });
      return { items: body.items ?? [], nextPageToken: body.next_page_token ?? "" };
    },
    getRuntimeImage(id, signal) {
      return authorizedRequest<RuntimeImage>(`/api/v1/runtime-images/${encodeURIComponent(id)}`, { signal });
    },
    registerRuntimeImage(input, idempotencyKey, signal) {
      return authorizedRequest<RuntimeImage>("/api/v1/runtime-images", {
        method: "POST", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    changeRuntimeImageStatus(id, input, version, idempotencyKey, signal) {
      return authorizedRequest<RuntimeImage>(`/api/v1/runtime-images/${encodeURIComponent(id)}/status`, {
        method: "PATCH", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async listCredentialProfiles(signal) {
      const body = await authorizedRequest<components["schemas"]["v1ListCredentialProfilesResponse"]>("/api/v1/credential-profiles", { signal });
      return body.items ?? [];
    },
    getCredentialProfile(id, signal) {
      return authorizedRequest<CredentialProfile>(`/api/v1/credential-profiles/${encodeURIComponent(id)}`, { signal });
    },
    registerCredentialProfile(input, idempotencyKey, signal) {
      return authorizedRequest<CredentialProfile>("/api/v1/credential-profiles", {
        method: "POST", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    changeCredentialProfileStatus(id, enabled, version, idempotencyKey, signal) {
      return authorizedRequest<CredentialProfile>(`/api/v1/credential-profiles/${encodeURIComponent(id)}/status`, {
        method: "PATCH", body: JSON.stringify({ enabled }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async listConfiguredModels(signal) {
      const body = await authorizedRequest<components["schemas"]["v1ListConfiguredModelsResponse"]>("/api/v1/configured-models", { signal });
      return body.items ?? [];
    },
    getConfiguredModel(id, signal) {
      return authorizedRequest<ConfiguredModel>(`/api/v1/configured-models/${encodeURIComponent(id)}`, { signal });
    },
    registerConfiguredModel(input, idempotencyKey, signal) {
      return authorizedRequest<ConfiguredModel>("/api/v1/configured-models", {
        method: "POST", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    changeConfiguredModelStatus(id, enabled, version, idempotencyKey, signal) {
      return authorizedRequest<ConfiguredModel>(`/api/v1/configured-models/${encodeURIComponent(id)}/status`, {
        method: "PATCH", body: JSON.stringify({ enabled }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async listSourceControlProviders(signal) {
      const body = await authorizedRequest<components["schemas"]["v1ListSourceControlProvidersResponse"]>("/api/v1/source-control-providers", { signal });
      return body.items ?? [];
    },
    getSourceControlProvider(id, signal) {
      return authorizedRequest<SourceControlProvider>(`/api/v1/source-control-providers/${encodeURIComponent(id)}`, { signal });
    },
    registerSourceControlProvider(input, idempotencyKey, signal) {
      return authorizedRequest<SourceControlProvider>("/api/v1/source-control-providers", {
        method: "POST", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    changeSourceControlProviderStatus(id, enabled, version, idempotencyKey, signal) {
      return authorizedRequest<SourceControlProvider>(`/api/v1/source-control-providers/${encodeURIComponent(id)}/status`, {
        method: "PATCH", body: JSON.stringify({ enabled }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async listRepositoryBindings(teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListRepositoryBindingsResponse"]>(`/api/v1/repository-bindings?${query}`, { signal });
      return body.items ?? [];
    },
    getRepositoryBinding(id, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<RepositoryBinding>(`/api/v1/repository-bindings/${encodeURIComponent(id)}?${query}`, { signal });
    },
    registerRepositoryBinding(input, idempotencyKey, signal) {
      return authorizedRequest<RepositoryBinding>("/api/v1/repository-bindings", {
        method: "POST", body: JSON.stringify({ binding: input }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    updateRepositoryBinding(id, input, version, idempotencyKey, signal) {
      return authorizedRequest<RepositoryBinding>(`/api/v1/repository-bindings/${encodeURIComponent(id)}`, {
        method: "PATCH", body: JSON.stringify({ binding: input }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    validateRepositoryBinding(id, teamID, version, idempotencyKey, signal) {
      return authorizedRequest<RepositoryBinding>(`/api/v1/repository-bindings/${encodeURIComponent(id)}/validation`, {
        method: "POST", body: JSON.stringify({ team_id: teamID }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async listAgents(teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListAgentsResponse"]>(`/api/v1/agents?${query}`, { signal });
      return body.items ?? [];
    },
    getAgent(id, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<Agent>(`/api/v1/agents/${encodeURIComponent(id)}?${query}`, { signal });
    },
    createAgent(teamID, input, idempotencyKey, signal) {
      return authorizedRequest<Agent>("/api/v1/agents", { method: "POST", body: JSON.stringify({ team_id: teamID, ...input }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey } });
    },
    updateAgent(id, teamID, input, version, idempotencyKey, signal) {
      return authorizedRequest<Agent>(`/api/v1/agents/${encodeURIComponent(id)}`, { method: "PATCH", body: JSON.stringify({ team_id: teamID, ...input }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    async listAgentDrafts(agentID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListAgentDraftsResponse"]>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts?${query}`, { signal });
      return body.items ?? [];
    },
    getAgentDraft(agentID, draftID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<AgentDraft>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}?${query}`, { signal });
    },
    createAgentDraft(agentID, teamID, input, idempotencyKey, signal) {
      return authorizedRequest<AgentDraft>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts`, { method: "POST", body: JSON.stringify({ team_id: teamID, ...input }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey } });
    },
    updateAgentDraft(agentID, draftID, teamID, input, version, idempotencyKey, signal) {
      return authorizedRequest<AgentDraft>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}`, { method: "PATCH", body: JSON.stringify({ team_id: teamID, ...input }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    validateAgentDraft(agentID, draftID, teamID, version, idempotencyKey, signal) {
      return authorizedRequest<AgentDraft>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}/validation`, { method: "POST", body: JSON.stringify({ team_id: teamID }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    getAgentDraftApproval(agentID, draftID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<ReleaseApproval>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}/approval?${query}`, { signal });
    },
    requestAgentDraftApproval(agentID, draftID, teamID, riskReason, idempotencyKey, signal) {
      return authorizedRequest<ReleaseApproval>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}/approval`, { method: "POST", body: JSON.stringify({ team_id: teamID, risk_reason: riskReason }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey } });
    },
    decideAgentDraftApproval(agentID, draftID, teamID, approved, reason, version, idempotencyKey, signal) {
      return authorizedRequest<ReleaseApproval>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}/approval`, { method: "PATCH", body: JSON.stringify({ team_id: teamID, approved, reason }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    publishAgentDraft(agentID, draftID, teamID, idempotencyKey, signal) {
      return authorizedRequest<AgentRelease>(`/api/v1/agents/${encodeURIComponent(agentID)}/drafts/${encodeURIComponent(draftID)}/release`, { method: "POST", body: JSON.stringify({ team_id: teamID }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey } });
    },
    async listAgentReleases(agentID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListAgentReleasesResponse"]>(`/api/v1/agents/${encodeURIComponent(agentID)}/releases?${query}`, { signal });
      return body.items ?? [];
    },
    getAgentRelease(agentID, releaseID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<AgentRelease>(`/api/v1/agents/${encodeURIComponent(agentID)}/releases/${encodeURIComponent(releaseID)}?${query}`, { signal });
    },
    deprecateAgentRelease(agentID, releaseID, teamID, version, idempotencyKey, signal) {
      return authorizedRequest<AgentRelease>(`/api/v1/agents/${encodeURIComponent(agentID)}/releases/${encodeURIComponent(releaseID)}/deprecation`, { method: "POST", body: JSON.stringify({ team_id: teamID }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    blockAgentRelease(agentID, releaseID, teamID, reason, version, idempotencyKey, signal) {
      return authorizedRequest<AgentRelease>(`/api/v1/agents/${encodeURIComponent(agentID)}/releases/${encodeURIComponent(releaseID)}/block`, { method: "POST", body: JSON.stringify({ team_id: teamID, reason }), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` } });
    },
    async listCodingTaskLaunchOptions(teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListCodingTaskLaunchOptionsResponse"]>(`/api/v1/coding-task-launch-options?${query}`, { signal });
      return { items: body.items ?? [], prerequisite: body.prerequisite ?? "" };
    },
    async listCodingTasks(teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      const body = await authorizedRequest<components["schemas"]["v1ListCodingTasksResponse"]>(`/api/v1/coding-tasks?${query}`, { signal });
      return body.items ?? [];
    },
    getCodingTask(id, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<CodingTask>(`/api/v1/coding-tasks/${encodeURIComponent(id)}?${query}`, { signal });
    },
    createCodingTask(teamID, input, idempotencyKey, signal) {
      return authorizedRequest<CodingTaskLaunch>("/api/v1/coding-tasks", {
        method: "POST", body: JSON.stringify({ team_id: teamID, ...input }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    getCodingTaskSession(taskID, teamID, signal) {
      const query = new URLSearchParams({ team_id: teamID });
      return authorizedRequest<CodingTaskSession>(`/api/v1/coding-tasks/${encodeURIComponent(taskID)}/session?${query}`, { signal });
    },
    async listRuns(teamID, taskID, signal) {
      const query = new URLSearchParams({ team_id: teamID, task_id: taskID, limit: "50" });
      const body = await authorizedRequest<components["schemas"]["v1ListRunsResponse"]>(`/api/v1/runs?${query}`, { signal });
      return body.items ?? [];
    },
    getRun(runID, signal) {
      return authorizedRequest<Run>(`/api/v1/runs/${encodeURIComponent(runID)}`, { signal });
    },
    async listRunApprovals(runID, signal) {
      const body = await authorizedRequest<components["schemas"]["v1ListRunApprovalsResponse"]>(`/api/v1/runs/${encodeURIComponent(runID)}/approvals`, { signal });
      return body.items ?? [];
    },
    decideRunApproval(approvalID, approved, reason, version, idempotencyKey, signal) {
      return authorizedRequest<RunApproval>(`/api/v1/approvals/${encodeURIComponent(approvalID)}/decision`, {
        method: "POST", body: JSON.stringify({ approved, reason }), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    controlRun(runID, action, version, idempotencyKey, signal) {
      return authorizedRequest<Run>(`/api/v1/runs/${encodeURIComponent(runID)}/${action}`, {
        method: "POST", signal,
        headers: { "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
    async streamRunEvents(runID, after, onEvent, signal) {
      const token = getAccessToken();
      if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication", "");
      const response = await fetch(`/api/v1/runs/${encodeURIComponent(runID)}/events`, {
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${token}`, "Last-Event-ID": String(after) }, signal,
      });
      if (!response.ok) throw await normalizeError(response);
      if (!response.body) throw new ApiError("unavailable", 503, "event_stream_unavailable", response.headers.get("X-Request-ID") ?? "");
      return consumeRunEventStream(response.body, runID, after, onEvent, signal);
    },
    async listRunArtifacts(runID, signal) {
      const body = await authorizedRequest<components["schemas"]["v1ListRunArtifactsResponse"]>(`/api/v1/runs/${encodeURIComponent(runID)}/artifacts`, { signal });
      return body.items ?? [];
    },
    getArtifactDownload(artifactID, signal) {
      return authorizedRequest<ArtifactDownload>(`/api/v1/artifacts/${encodeURIComponent(artifactID)}/download`, { signal });
    },
  };
}

const terminalRunEvents = new Set(["run.completed", "run.failed", "run.cancelled", "run.killed"]);

async function consumeRunEventStream(stream: ReadableStream<Uint8Array>, runID: string, after: number, onEvent: (event: RunEvent) => void, signal?: AbortSignal): Promise<RunEventStreamResult> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = ""; let cursor = after; let terminal = false;
  try {
    while (true) {
      if (signal?.aborted) throw signal.reason;
      const { done, value } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      buffer = buffer.replace(/\r\n/g, "\n");
      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        const frame = buffer.slice(0, boundary).replace(/\r/g, ""); buffer = buffer.slice(boundary + 2);
        if (frame && !frame.startsWith(":")) {
          const lines = frame.split("\n");
          const eventType = lines.find((line) => line.startsWith("event:"))?.slice(6).trim() ?? "message";
          const data = lines.filter((line) => line.startsWith("data:")).map((line) => line.slice(5).trimStart()).join("\n");
          if (eventType === "stream_error") {
            let code = "event_stream_failed";
            try { code = (JSON.parse(data) as { error?: string }).error || code; } catch { /* Keep the safe generic code. */ }
            const kind: ApiErrorKind = code === "invalid_authentication" ? "unauthenticated" : code === "run_access_denied" ? "forbidden" : "unavailable";
            throw new ApiError(kind, kind === "unauthenticated" ? 401 : kind === "forbidden" ? 403 : 503, code, "");
          }
          let event: RunEvent;
          try { event = JSON.parse(data) as RunEvent; }
          catch { throw new ApiError("unavailable", 503, "event_contract_invalid", ""); }
          if (event.run_id !== runID || event.event_type !== eventType) throw new ApiError("unavailable", 503, "event_contract_invalid", "");
          if (event.sequence === cursor) { boundary = buffer.indexOf("\n\n"); continue; }
          if (terminal || event.sequence !== cursor + 1) throw new ApiError("unavailable", 503, "event_contract_invalid", "");
          terminal = terminalRunEvents.has(event.event_type); cursor = event.sequence; onEvent(event);
        }
        boundary = buffer.indexOf("\n\n");
      }
      if (done) {
        if (buffer.trim()) throw new ApiError("unavailable", 503, "event_contract_invalid", "");
        return { cursor, terminal };
      }
    }
  } finally { reader.releaseLock(); }
}

async function request<T>(accessToken: string, path: string, init: RequestInit): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(path, { ...init, headers });
  if (!response.ok) throw await normalizeError(response);
  return (await response.json()) as T;
}

async function normalizeError(response: Response): Promise<ApiError> {
  let code = "";
  try {
    const body = await response.json() as { error?: string; reason?: string; message?: string };
    code = body.error ?? body.reason ?? body.message ?? "";
  } catch {
    // A malformed upstream response is still represented by its safe status class.
  }
  const kind: ApiErrorKind = response.status === 401 ? "unauthenticated"
    : response.status === 403 ? "forbidden"
      : response.status === 404 ? "not_found"
        : response.status === 409 || response.status === 412 ? "conflict"
          : response.status === 400 || response.status === 422 || response.status === 428 ? "validation"
            : response.status === 429 ? "rate_limited"
              : response.status >= 500 ? "unavailable" : "unknown";
  return new ApiError(kind, response.status, code, response.headers.get("X-Request-ID") ?? "");
}
