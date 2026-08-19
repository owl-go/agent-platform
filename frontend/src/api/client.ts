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
export type AgentConfiguration = components["schemas"]["v1AgentConfiguration"];
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
  };
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
