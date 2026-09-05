import type { InjectionKey } from "vue";

export interface CreditBalance { total_hundredths: number; daily_remaining_hundredths: number; persistent_hundredths: number; today_consumed_hundredths: number; daily_allocation_hundredths: number; credit_day: string; timezone: string; next_allocation_at: string; pending_daily_allocation_hundredths?: number; pending_effective_day?: string; version: number }
export interface CreditStageConsumption { stage_position: number; provider_model: string; runtime_engine: string; input_tokens: number; output_tokens: number; usage_reported: boolean; input_multiplier_micros: number; output_multiplier_micros: number; fallback_hundredths: number; amount_hundredths: number; estimated: boolean; rate_revision_id: string }
export interface CreditConsumption { total_hundredths: number; stages: CreditStageConsumption[] }
export interface CreditLedgerEntry { id: string; type: string; amount_hundredths: number; resulting_balance_hundredths: number; credit_day: string; reason?: string; created_at: string }
export interface ModelCreditRate { revision_id: string; provider_type?: string; api_protocol?: string; provider_model_id?: string; input_multiplier_micros: number; output_multiplier_micros: number; fallback_hundredths: number; created_at: string; superseded_at?: string }
export interface RedemptionCodeBatch { id: string; count: number; value_hundredths: number; expires_at?: string; created_at: string; codes: Array<{ id: string; identifier: string; plaintext: string; state: string }> }
export interface RedemptionCodeStatus { id: string; batch_id: string; identifier: string; state: "available" | "redeemed" | "void" | "expired"; value_hundredths: number; expires_at?: string; redeemed_at?: string; voided_at?: string; created_at: string }
export interface CurrentUser { id: string; username: string; email: string; display_name: string; administrator: boolean; settings_ready: boolean; credit_balance?: CreditBalance }
export interface Session { id: string; title: string; expert_id?: string; expert_team_id?: string; archived: boolean; created_at: string; updated_at: string; version: number }
export interface ExecutionStageSnapshot { position: number; expert?: { id: string; name: string; execution_instruction: string; version: number }; runtime_engine: RuntimeEngine; provider_model: { id: string; connection_id: string; connection_version: number; connection_name: string; provider_type: string; model_id: string; name: string; endpoint: string; protocols: string[]; compatibility: CompatibilityStatus }; cli_connectors?: Array<{ id: string; name: string; executable: string; authentication_driver: string; bundle_sha256: string; runtime_digests: string[]; version: number }> }
export interface ResponseSnapshot { provider_model_id: string; connection_id: string; connection_name: string; provider_type: string; model_id: string; model_name: string; endpoint: string; protocols: string[]; runtime_engine: RuntimeEngine; compatibility: CompatibilityStatus; connection_version: number; schema_version?: number; stages?: ExecutionStageSnapshot[] }
export interface Attachment { id: string; name: string; content_type: string; size: number; sha256: string; image: boolean }
export interface ExpertStage { expert_id: string; expert_name: string; provider_model_id?: string; provider_model_name?: string; runtime_engine?: RuntimeEngine; position: number; total: number; state: "running" | "succeeded" | "failed" | "cancelled"; elapsed_ms: number; final_text?: string; error?: string; credit_consumption?: CreditStageConsumption }
export interface ExecutionActivity { type: string; detail: string }
export interface SessionMessage { id: number; role: "user" | "assistant"; state: string; content: string; error?: string; progress_stage?: string; elapsed_ms: number; created_at: string; response_snapshot?: ResponseSnapshot; attachments?: Attachment[]; expert_stages?: ExpertStage[]; credit_consumption?: CreditConsumption; activities?: ExecutionActivity[]; artifacts?: Artifact[] }
export interface SessionMessageSnapshot { state: string; content: string; error?: string; progress_stage?: string; elapsed_ms: number; expert_stages?: ExpertStage[]; credit_consumption?: CreditConsumption; activities?: ExecutionActivity[] }
export interface EnvironmentVariable { name: string; value?: string; secret: boolean; configured: boolean }
export interface Schedule { enabled: boolean; frequency: "hourly" | "daily" | "weekly"; hour: number; minute: number; weekday: number; timezone: string }
export interface GitConfigEntry { key: string; value: string }
export interface GitSource { url: string; branch: string; authentication: "none" | "basic" | "ssh"; username?: string; config: GitConfigEntry[]; ssh_config?: string; credential_configured: boolean }
export interface GitSourceInput { url: string; branch: string; authentication: "none" | "basic" | "ssh"; username?: string; password?: string; ssh_private_key?: string; config: GitConfigEntry[]; ssh_config?: string }
export interface WorkflowInput { name: string; goal: string; expert_id?: string; expert_team_id?: string; environment: EnvironmentVariable[]; schedule?: Schedule }
export interface Workflow extends WorkflowInput { id: string; git_source?: GitSource; api_credential_configured: boolean; deleted: boolean; created_at: string; updated_at: string; version: number }
export interface Run { id: string; conversation_id: string; turn_number: number; workflow_id: string; workflow_name: string; trigger: "manual" | "scheduled" | "api"; state: "queued" | "running" | "waiting_for_user" | "succeeded" | "failed" | "cancelled"; text_input?: string; json_input?: Record<string, unknown>; attachments?: Attachment[]; final_text?: string; final_json?: Record<string, unknown>; error?: string; queued_at: string; started_at?: string; ended_at?: string; elapsed_ms: number; workflow_snapshot?: Record<string, unknown>; expert_stages?: ExpertStage[]; credit_consumption?: CreditConsumption }
export interface RunEvent { sequence: number; type: string; payload: Record<string, unknown>; raw: string }
export interface Artifact { id: string; run_id?: string; message_id?: number; kind: "result" | "file"; name: string; path: string; size: number; sha256?: string; text_preview?: string; expired: boolean; created_at: string; expires_at?: string }
export interface WorkspaceEntry { path: string; name: string; directory: boolean; size: number; modified_at: string }
export interface WorkspaceFile { path: string; content: string; content_type: string; size: number; modified_at: string }
export interface ExpertInput { name: string; icon: string; icon_background: string; introduction: string; core_capability: string; operating_procedure: string; output_standard: string; cautions: string; mcp_server_ids: string[]; skill_ids: string[]; cli_connector_definition_ids: string[] }
export interface Expert extends ExpertInput { id: string; expertise_tags: string[]; tag_projection_status?: "idle" | "queued" | "running" | "succeeded" | "failed"; tag_projection_error?: string; complete: boolean; available: boolean; availability_reason?: string; compatibility: "verified" | "unverified" | "incompatible" | "unavailable"; created_at: string; updated_at: string; version: number }
export interface ExpertTeamMemberInput { id: string; name: string; expert_id: string; labels: string[] }
export interface ExpertTeamMember extends ExpertTeamMemberInput { expert: Expert; position: number }
export interface ExpertTeamInput { name: string; icon: string; icon_background: string; introduction: string; core_capability: string; members: ExpertTeamMemberInput[] }
export interface ExpertTeam extends ExpertTeamInput { id: string; experts: Expert[]; expertise_tags: string[]; capability_introduction?: string; available: boolean; created_at: string; updated_at: string; version: number; members: ExpertTeamMember[] }
export interface RuntimeModelDefault { runtime_engine: RuntimeEngine; provider_model_id: string }
export interface PersonalSettings { personality: Personality; personality_instructions: string; runtime_model_defaults: RuntimeModelDefault[]; default_runtime_engine: RuntimeEngine; language: "zh-CN" | "en-US"; timezone: string; version: number }
export interface RuntimeEngineStatus { name: RuntimeEngine; available: boolean; native_resume: boolean; cli_version: string }
export type CompatibilityStatus = "verified" | "unverified" | "incompatible";
export interface RuntimeModelCompatibility { runtime_engine: RuntimeEngine; status: CompatibilityStatus; reason?: string }
export interface ProviderModel { id: string; connection_id: string; model_id: string; display_name: string; available: boolean; manually_added: boolean; compatibility: RuntimeModelCompatibility[] }
export interface ModelProviderConnection { id: string; name: string; provider_type: string; endpoint: string; protocols: string[]; api_key_configured: boolean; verification_status: "verified" | "unverified" | "failed"; verification_error?: string; custom_endpoint: boolean; last_synced_at?: string; last_sync_error?: string; models: ProviderModel[]; created_at: string; updated_at: string; version: number }
export interface ModelProviderPreset { provider_type: string; display_name: string; official_endpoint: string; protocols: string[] }
export interface MCPServer { id: string; name: string; transport: "stdio" | "streamable_http"; url?: string; runner?: "npx" | "uvx"; package?: string; package_version?: string; arguments: string[]; environment: EnvironmentVariable[]; tested: boolean; test_pending: boolean; test_error?: string; created_at: string; updated_at: string; version: number }
export interface Skill { id: string; name: string; source: "git" | "upload"; git_url?: string; git_ref?: string; sha256: string; created_at: string; updated_at: string; version: number }
export interface ResourceDeletionImpact { affected_experts: Array<{ id: string; name: string; version: number }>; confirmation_token: string }
export interface CLICapability { id: string; argv_prefix: string[]; risk: "low" | "high"; identities: Array<"user" | "bot">; scopes: string[]; egress_hosts: string[]; timeout_seconds: number }
export interface CLIConnectorDefinitionInput { name: string; npm_package: string; npm_version: string; npm_integrity: string; executable: string; authentication_driver: "none" | "feishu"; capabilities: CLICapability[]; supported_architectures: Array<"linux-amd64" | "linux-arm64">; recommended_skill_ids: string[] }
export interface CLIConnectorDefinition extends CLIConnectorDefinitionInput { id: string; state: "draft" | "building" | "testing" | "available" | "failed" | "disabled"; failure_reason?: string; bundle_sha256?: string; mutable: boolean; version: number; conformance_runtime_digests: string[] }
export interface CLIConnectorEnablement { id: string; definition_id: string; state: "waiting_for_user" | "enabled" | "invalid" | "disabled"; action_url?: string; action_expires_at?: string; version: number }
export interface CommandApproval { id: string; execution_kind: "session" | "run"; execution_id: string; connector_name: string; operation: string; target: string; redacted_arguments: string; state: "pending" | "approved" | "rejected" | "consumed" | "expired" | "closed"; identity?: "user" | "bot"; expires_at: string; version: number }
export interface UserAccount { id: string; username: string; email: string; display_name: string; administrator: boolean; enabled: boolean; created_at: string; version: number; credit_balance?: CreditBalance }
export type RuntimeEngine = "claude" | "codex" | "hermes" | "openclaw" | "pi";

export function runtimeEngineDisplayName(runtime?: RuntimeEngine | string | null): string {
  if (!runtime) return "—";
  if (runtime === "claude") return "Claude Code";
  if (runtime === "openclaw") return "OpenClaw";
  if (runtime === "pi") return "PI Agent";
  return runtime[0].toUpperCase() + runtime.slice(1);
}
export type Personality = "gentle_professional" | "direct_efficient" | "lively_friendly" | "custom";

export type ApiErrorKind = "unauthenticated" | "forbidden" | "not_found" | "conflict" | "validation" | "rate_limited" | "unavailable" | "unknown";
export class ApiError extends Error {
  constructor(public readonly kind: ApiErrorKind, public readonly status: number, public readonly code: string, public readonly requestID = "") {
    super(code || `request_failed_${status}`);
    this.name = "ApiError";
  }
}

export interface PlatformApi {
  getCreditBalance(signal?: AbortSignal): Promise<CreditBalance>;
  listCreditLedger(cursor?: string, signal?: AbortSignal): Promise<{ items: CreditLedgerEntry[]; next_cursor?: string }>;
  redeemCreditCode(code: string, signal?: AbortSignal): Promise<CreditBalance>;
  configureUserDailyCredits(userID: string, allocationHundredths: number, signal?: AbortSignal): Promise<CreditBalance>;
  adjustUserCredits(userID: string, amountHundredths: number, reason: string, requestID: string, signal?: AbortSignal): Promise<CreditBalance>;
  listModelCreditRates(signal?: AbortSignal): Promise<ModelCreditRate[]>;
  createModelCreditRate(rate: Omit<ModelCreditRate, "revision_id" | "created_at" | "superseded_at"> & { expected_revision_id?: string }, signal?: AbortSignal): Promise<ModelCreditRate>;
  createRedemptionCodeBatch(count: number, valueHundredths: number, expiresAt?: string, signal?: AbortSignal): Promise<RedemptionCodeBatch>;
  listRedemptionCodes(cursor?: string, signal?: AbortSignal): Promise<{ items: RedemptionCodeStatus[]; next_cursor?: string }>;
  voidRedemptionCode(codeID: string, signal?: AbortSignal): Promise<RedemptionCodeStatus>;
  listSessions(archived?: boolean, signal?: AbortSignal): Promise<Session[]>;
  createSession(selection?: { expert_id?: string; expert_team_id?: string }, signal?: AbortSignal): Promise<Session>;
  renameSession(id: string, title: string, version: number, signal?: AbortSignal): Promise<Session>;
  archiveSession(id: string, archived: boolean, version: number, signal?: AbortSignal): Promise<Session>;
  setSessionExpertSelection(id: string, selection: { expert_id?: string; expert_team_id?: string }, version: number, signal?: AbortSignal): Promise<Session>;
  deleteSession(id: string, signal?: AbortSignal): Promise<void>;
  listSessionMessages(id: string, signal?: AbortSignal): Promise<SessionMessage[]>;
  streamSessionMessage(id: string, messageID: number, onSnapshot: (snapshot: SessionMessageSnapshot) => void, signal?: AbortSignal): Promise<void>;
  uploadAttachment(file: File, signal?: AbortSignal): Promise<Attachment>;
  getAttachmentDownload(id: string, signal?: AbortSignal): Promise<Blob>;
  sendSessionMessage(id: string, content: string, attachmentIDs?: string[], signal?: AbortSignal): Promise<{ user_message: SessionMessage; assistant_message: SessionMessage }>;
  retrySessionMessage(sessionID: string, messageID: number, signal?: AbortSignal): Promise<{ user_message: SessionMessage; assistant_message: SessionMessage }>;
  cancelSessionMessage(sessionID: string, messageID: number, signal?: AbortSignal): Promise<SessionMessage>;
  getSessionArtifactDownload(sessionID: string, artifactID: string, signal?: AbortSignal): Promise<Blob>;
  listWorkflows(deleted?: boolean, signal?: AbortSignal): Promise<Workflow[]>;
  createWorkflow(workflow: WorkflowInput, signal?: AbortSignal): Promise<Workflow>;
  getWorkflow(id: string, signal?: AbortSignal): Promise<Workflow>;
  updateWorkflow(id: string, workflow: WorkflowInput, version: number, signal?: AbortSignal): Promise<Workflow>;
  deleteWorkflow(id: string, signal?: AbortSignal): Promise<void>;
  generateWorkflowCredential(id: string, signal?: AbortSignal): Promise<{ api_key: string; api_secret: string; created_at: string }>;
  runWorkflow(id: string, input?: { text_input?: string; json_input?: Record<string, unknown> }, signal?: AbortSignal): Promise<Run>;
  listRuns(id: string, signal?: AbortSignal): Promise<Run[]>;
  getRun(workflowID: string, runID: string, signal?: AbortSignal): Promise<Run>;
  listRunTurns(workflowID: string, runID: string, signal?: AbortSignal): Promise<Run[]>;
  continueRunConversation(workflowID: string, runID: string, content: string, attachmentIDs?: string[], signal?: AbortSignal): Promise<Run>;
  streamRunEvents(workflowID: string, runID: string, onEvent: (event: RunEvent) => void, signal?: AbortSignal): Promise<void>;
  cancelRun(workflowID: string, runID: string, signal?: AbortSignal): Promise<Run>;
  rerunWorkflow(workflowID: string, runID: string, signal?: AbortSignal): Promise<Run>;
  listArtifacts(id: string, signal?: AbortSignal): Promise<Artifact[]>;
  getArtifactDownload(workflowID: string, artifactID: string, signal?: AbortSignal): Promise<Blob>;
  listWorkspace(id: string, path?: string, signal?: AbortSignal): Promise<{ items: WorkspaceEntry[]; used_bytes: number; limit_bytes: number }>;
  getWorkspaceFile(id: string, path: string, signal?: AbortSignal): Promise<WorkspaceFile>;
  downloadWorkspaceFile(id: string, path: string, signal?: AbortSignal): Promise<Blob>;
  configureWorkflowGitSource(id: string, input: GitSourceInput, signal?: AbortSignal): Promise<Workflow>;
  listExperts(signal?: AbortSignal): Promise<Expert[]>;
  getExpert(id: string, signal?: AbortSignal): Promise<Expert>;
  createExpert(input: ExpertInput, signal?: AbortSignal): Promise<Expert>;
  updateExpert(id: string, input: ExpertInput, version: number, signal?: AbortSignal): Promise<Expert>;
  deleteExpert(id: string, signal?: AbortSignal): Promise<void>;
  listExpertTeams(signal?: AbortSignal): Promise<ExpertTeam[]>;
  getExpertTeam(id: string, signal?: AbortSignal): Promise<ExpertTeam>;
  createExpertTeam(input: ExpertTeamInput, signal?: AbortSignal): Promise<ExpertTeam>;
  updateExpertTeam(id: string, input: ExpertTeamInput, version: number, signal?: AbortSignal): Promise<ExpertTeam>;
  deleteExpertTeam(id: string, signal?: AbortSignal): Promise<void>;
  getSettings(signal?: AbortSignal): Promise<PersonalSettings>;
  updateSettings(settings: PersonalSettings, signal?: AbortSignal): Promise<PersonalSettings>;
  listRuntimeEngines(signal?: AbortSignal): Promise<RuntimeEngineStatus[]>;
  listModelProviderPresets(signal?: AbortSignal): Promise<ModelProviderPreset[]>;
  listModelProviderConnections(signal?: AbortSignal): Promise<ModelProviderConnection[]>;
  createModelProviderConnection(input: { name: string; provider_type: string; endpoint: string; protocols: string[]; api_key: string }, signal?: AbortSignal): Promise<ModelProviderConnection>;
  updateModelProviderConnection(id: string, input: { name: string; endpoint: string; protocols: string[]; api_key?: string }, version: number, signal?: AbortSignal): Promise<ModelProviderConnection>;
  deleteModelProviderConnection(id: string, signal?: AbortSignal): Promise<void>;
  refreshProviderModels(id: string, signal?: AbortSignal): Promise<ModelProviderConnection>;
  createProviderModel(connectionID: string, input: { model_id: string }, signal?: AbortSignal): Promise<ProviderModel>;
  listMCPServers(signal?: AbortSignal): Promise<MCPServer[]>;
  createMCPServer(input: Record<string, unknown>, signal?: AbortSignal): Promise<MCPServer>;
  updateMCPServer(id: string, input: Record<string, unknown>, version: number, signal?: AbortSignal): Promise<MCPServer>;
  testMCPServer(id: string, signal?: AbortSignal): Promise<MCPServer>;
  getMCPConnectorDeletionImpact(id: string, signal?: AbortSignal): Promise<ResourceDeletionImpact>;
  deleteMCPServer(id: string, confirmationToken: string, signal?: AbortSignal): Promise<void>;
  listSkills(signal?: AbortSignal): Promise<Skill[]>;
  createGitSkill(input: { name: string; git_url: string; git_ref?: string }, signal?: AbortSignal): Promise<Skill>;
  createUploadSkill(input: { name: string; archive: string }, signal?: AbortSignal): Promise<Skill>;
  updateSkill(id: string, input: { git_ref?: string; archive?: string }, version: number, signal?: AbortSignal): Promise<Skill>;
  getSkillDeletionImpact(id: string, signal?: AbortSignal): Promise<ResourceDeletionImpact>;
  deleteSkill(id: string, confirmationToken: string, signal?: AbortSignal): Promise<void>;
  listCLIConnectorDefinitions(signal?: AbortSignal): Promise<CLIConnectorDefinition[]>;
  createCLIConnectorDefinition(input: CLIConnectorDefinitionInput, signal?: AbortSignal): Promise<CLIConnectorDefinition>;
  updateCLIConnectorDefinition(id: string, input: CLIConnectorDefinitionInput, version: number, signal?: AbortSignal): Promise<CLIConnectorDefinition>;
  publishCLIConnectorDefinition(id: string, version: number, signal?: AbortSignal): Promise<CLIConnectorDefinition>;
  disableCLIConnectorDefinition(id: string, version: number, signal?: AbortSignal): Promise<CLIConnectorDefinition>;
  enableCLIConnector(id: string, signal?: AbortSignal): Promise<CLIConnectorEnablement>;
  listCLIConnectorEnablements(signal?: AbortSignal): Promise<CLIConnectorEnablement[]>;
  listCommandApprovals(signal?: AbortSignal): Promise<CommandApproval[]>;
  decideCommandApproval(id: string, decision: "approved" | "rejected", identity: "user" | "bot" | undefined, version: number, signal?: AbortSignal): Promise<CommandApproval>;
  listUsers(signal?: AbortSignal): Promise<UserAccount[]>;
  createUser(input: { username: string; email: string; display_name: string }, signal?: AbortSignal): Promise<{ user: UserAccount; temporary_password: string }>;
  setUserEnabled(id: string, enabled: boolean, version: number, signal?: AbortSignal): Promise<UserAccount>;
  resetUserPassword(id: string, signal?: AbortSignal): Promise<{ temporary_password: string }>;
}

export const platformApiKey: InjectionKey<PlatformApi> = Symbol("agent-workspace-api");

export async function getHealth(signal?: AbortSignal): Promise<{ status: string }> {
  const response = await fetch("/api/healthz", { headers: { Accept: "application/json" }, signal });
  if (!response.ok) throw new Error(`Health check failed with status ${response.status}`);
  return response.json() as Promise<{ status: string }>;
}

export async function getCurrentUser(accessToken: string, signal?: AbortSignal): Promise<CurrentUser> {
  return request<CurrentUser>(accessToken, "/api/v1/me", { signal });
}

export function createPlatformApi(getAccessToken: () => string | undefined): PlatformApi {
  const call = <T>(path: string, init: RequestInit = {}) => {
    const token = getAccessToken();
    if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication");
    return request<T>(token, path, init);
  };
  const json = (method: string, body?: unknown, signal?: AbortSignal): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body), signal, headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() } });
  const download = async (path: string, signal?: AbortSignal) => {
    const token = getAccessToken();
    if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication");
    const response = await fetch(path, { signal, headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) throw new ApiError(response.status === 404 ? "not_found" : "unknown", response.status, "download_failed");
    return response.blob();
  };
  const remove = async (path: string, signal?: AbortSignal) => { await call<{ deleted: boolean }>(path, { method: "DELETE", signal, headers: { "Idempotency-Key": crypto.randomUUID() } }); };
  return {
    getCreditBalance(signal) { return call("/api/v1/credits/balance", { signal }); },
    listCreditLedger(cursor = "", signal) { return call(`/api/v1/credits/ledger?limit=50${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`, { signal }); },
    redeemCreditCode(code, signal) { return call("/api/v1/credits/redemptions", json("POST", { code }, signal)); },
    configureUserDailyCredits(userID, allocationHundredths, signal) { return call(`/api/v1/admin/users/${encodeURIComponent(userID)}/daily-credits`, json("PATCH", { allocation_hundredths: allocationHundredths }, signal)); },
    adjustUserCredits(userID, amountHundredths, reason, requestID, signal) { return call(`/api/v1/admin/users/${encodeURIComponent(userID)}/credit-adjustments`, json("POST", { amount_hundredths: amountHundredths, reason, request_id: requestID }, signal)); },
    async listModelCreditRates(signal) { return (await call<{ items: ModelCreditRate[] }>("/api/v1/admin/model-credit-rates", { signal })).items ?? []; },
    createModelCreditRate(rate, signal) { return call("/api/v1/admin/model-credit-rates", json("POST", rate, signal)); },
    createRedemptionCodeBatch(count, valueHundredths, expiresAt, signal) { return call("/api/v1/admin/redemption-code-batches", json("POST", { count, value_hundredths: valueHundredths, ...(expiresAt ? { expires_at: expiresAt } : {}) }, signal)); },
    listRedemptionCodes(cursor = "", signal) { return call(`/api/v1/admin/redemption-codes?limit=50${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`, { signal }); },
    voidRedemptionCode(codeID, signal) { return call(`/api/v1/admin/redemption-codes/${encodeURIComponent(codeID)}/void`, json("POST", {}, signal)); },
    async listSessions(archived = false, signal) { return (await call<{ items: Session[] }>(`/api/v1/sessions?archived=${archived}`, { signal })).items ?? []; },
    createSession(selection = {}, signal) { return call("/api/v1/sessions", json("POST", selection, signal)); },
    renameSession(id, title, version, signal) { return call(`/api/v1/sessions/${encodeURIComponent(id)}`, json("PATCH", { title, expected_version: version }, signal)); },
    archiveSession(id, archived, version, signal) { return call(`/api/v1/sessions/${encodeURIComponent(id)}/archived`, json("PATCH", { archived, expected_version: version }, signal)); },
    setSessionExpertSelection(id, selection, version, signal) { return call(`/api/v1/sessions/${encodeURIComponent(id)}/expert-selection`, json("PATCH", { ...selection, expected_version: version }, signal)); },
    deleteSession(id, signal) { return remove(`/api/v1/sessions/${encodeURIComponent(id)}`, signal); },
    async listSessionMessages(id, signal) {
      const messages: SessionMessage[] = [];
      let after = 0;
      while (true) {
        const page = (await call<{ items: SessionMessage[] }>(`/api/v1/sessions/${encodeURIComponent(id)}/messages?after=${after}&limit=200`, { signal })).items ?? [];
        messages.push(...page.map((message) => ({ ...message, artifacts: (message.artifacts ?? []).map((artifact) => ({ ...artifact, message_id: Number(artifact.message_id ?? 0), size: Number(artifact.size ?? 0) })) })));
        if (page.length < 200) return messages;
        const next = page.at(-1)?.id ?? after;
        if (next <= after) throw new ApiError("unknown", 500, "invalid_message_page");
        after = next;
      }
    },
    async streamSessionMessage(id, messageID, onSnapshot, signal) {
      const token = getAccessToken();
      if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication");
      const response = await fetch(`/api/v1/sessions/${encodeURIComponent(id)}/messages/${messageID}/events`, { signal, headers: { Accept: "text/event-stream", Authorization: `Bearer ${token}` } });
      if (!response.ok || !response.body) throw new ApiError(response.status === 404 ? "not_found" : "unknown", response.status, "message_stream_failed");
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let pending = "";
      const handleBlock = (value: string) => {
        const fields = Object.fromEntries(value.split("\n").filter((line) => line.includes(":") && !line.startsWith(":"))
          .map((line) => { const index = line.indexOf(":"); return [line.slice(0, index), line.slice(index + 1).trimStart()]; }));
        if (fields.event === "message.snapshot") {
          try { onSnapshot(JSON.parse(fields.data ?? "{}") as SessionMessageSnapshot); } catch { throw new ApiError("unknown", 500, "message_stream_invalid"); }
        } else if (fields.event === "stream.error") {
          throw new ApiError("unknown", 500, "message_stream_failed");
        }
      };
      while (true) {
        const { value, done } = await reader.read();
        pending = (pending + (value ?? "")).replace(/\r\n/g, "\n");
        let boundary = pending.indexOf("\n\n");
        while (boundary >= 0) {
          const block = pending.slice(0, boundary);
          pending = pending.slice(boundary + 2);
          handleBlock(block);
          boundary = pending.indexOf("\n\n");
        }
        if (done) {
          if (pending.trim()) handleBlock(pending.replace(/\r/g, ""));
          return;
        }
      }
    },
    async uploadAttachment(file, signal) {
      const token = getAccessToken();
      if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication");
      const response = await fetch(`/api/v1/attachments/upload?name=${encodeURIComponent(file.name)}`, { method: "POST", body: file, signal, headers: { Authorization: `Bearer ${token}`, "Content-Type": file.type || "application/octet-stream", "Idempotency-Key": crypto.randomUUID() } });
      if (!response.ok) throw new ApiError(response.status === 413 || response.status === 422 ? "validation" : "unknown", response.status, "attachment_upload_failed");
      return response.json() as Promise<Attachment>;
    },
    getAttachmentDownload(id, signal) { return download(`/api/v1/attachments/${encodeURIComponent(id)}/download`, signal); },
    sendSessionMessage(id, content, attachmentIDs = [], signal) { return call(`/api/v1/sessions/${encodeURIComponent(id)}/messages`, json("POST", { content, attachment_ids: attachmentIDs }, signal)); },
    retrySessionMessage(sessionID, messageID, signal) { return call(`/api/v1/sessions/${encodeURIComponent(sessionID)}/messages/${messageID}/retry`, json("POST", {}, signal)); },
    cancelSessionMessage(sessionID, messageID, signal) { return call(`/api/v1/sessions/${encodeURIComponent(sessionID)}/messages/${messageID}/cancellation`, json("POST", {}, signal)); },
    getSessionArtifactDownload(sessionID, artifactID, signal) { return download(`/api/v1/sessions/${encodeURIComponent(sessionID)}/artifacts/${encodeURIComponent(artifactID)}/download`, signal); },
    async listWorkflows(deleted = false, signal) { return (await call<{ items: Workflow[] }>(`/api/v1/workflows?deleted=${deleted}`, { signal })).items ?? []; },
    createWorkflow(workflow, signal) { return call("/api/v1/workflows", json("POST", { workflow }, signal)); },
    getWorkflow(id, signal) { return call(`/api/v1/workflows/${encodeURIComponent(id)}`, { signal }); },
    updateWorkflow(id, workflow, version, signal) { return call(`/api/v1/workflows/${encodeURIComponent(id)}`, json("PATCH", { workflow, expected_version: version }, signal)); },
    deleteWorkflow(id, signal) { return remove(`/api/v1/workflows/${encodeURIComponent(id)}`, signal); },
    generateWorkflowCredential(id, signal) { return call(`/api/v1/workflows/${encodeURIComponent(id)}/api-credential`, json("POST", {}, signal)); },
    async runWorkflow(id, input, signal) { return normalizeRun(await call(`/api/v1/workflows/${encodeURIComponent(id)}/runs`, json("POST", input ?? {}, signal))); },
    async listRuns(id, signal) { return ((await call<{ items: Run[] }>(`/api/v1/workflows/${encodeURIComponent(id)}/runs`, { signal })).items ?? []).map(normalizeRun); },
    async getRun(workflowID, runID, signal) { return normalizeRun(await call(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}`, { signal })); },
    async listRunTurns(workflowID, runID, signal) { return ((await call<{ items: Run[] }>(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}/turns`, { signal })).items ?? []).map(normalizeRun); },
    async continueRunConversation(workflowID, runID, content, attachmentIDs = [], signal) { return normalizeRun(await call(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}/turns`, json("POST", { content, attachment_ids: attachmentIDs }, signal))); },
    async streamRunEvents(workflowID, runID, onEvent, signal) {
      const token = getAccessToken();
      if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication");
      const response = await fetch(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}/events`, { signal, headers: { Accept: "text/event-stream", Authorization: `Bearer ${token}` } });
      if (!response.ok || !response.body) throw new ApiError(response.status === 403 ? "forbidden" : "unknown", response.status, "event_stream_failed");
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let pending = "";
      while (true) {
        const { value, done } = await reader.read();
        pending += value ?? "";
        let boundary = pending.indexOf("\n\n");
        while (boundary >= 0) {
          const block = pending.slice(0, boundary).replace(/\r/g, "");
          pending = pending.slice(boundary + 2);
          const fields = Object.fromEntries(block.split("\n").filter((line) => line.includes(":") && !line.startsWith(":"))
            .map((line) => { const index = line.indexOf(":"); return [line.slice(0, index), line.slice(index + 1).trimStart()]; }));
          if (fields.id && fields.event) {
            let payload: Record<string, unknown> = {};
            try { payload = JSON.parse(fields.data ?? "{}") as Record<string, unknown>; } catch { payload = { parse_error: true }; }
            onEvent({ sequence: Number(fields.id), type: fields.event, payload, raw: fields.data ?? "{}" });
          }
          boundary = pending.indexOf("\n\n");
        }
        if (done) return;
      }
    },
    async cancelRun(workflowID, runID, signal) { return normalizeRun(await call(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}/cancellation`, json("POST", {}, signal))); },
    async rerunWorkflow(workflowID, runID, signal) { return normalizeRun(await call(`/api/v1/workflows/${encodeURIComponent(workflowID)}/runs/${encodeURIComponent(runID)}/rerun`, json("POST", {}, signal))); },
    async listArtifacts(id, signal) {
      const result = await call<{ items?: Array<Omit<Artifact, "size"> & { size?: number | string }> }>(`/api/v1/workflows/${encodeURIComponent(id)}/artifacts`, { signal });
      return (result.items ?? []).filter((item) => item.kind === "file").map((item) => ({ ...item, size: Number(item.size ?? 0) }));
    },
    getArtifactDownload(workflowID, artifactID, signal) { return download(`/api/v1/workflows/${encodeURIComponent(workflowID)}/artifacts/${encodeURIComponent(artifactID)}/download`, signal); },
    async listWorkspace(id, path = "", signal) {
      const result = await call<{ items?: WorkspaceEntry[]; used_bytes?: number | string; limit_bytes?: number | string; usedBytes?: number | string; limitBytes?: number | string }>(`/api/v1/workflows/${encodeURIComponent(id)}/workspace?path=${encodeURIComponent(path)}`, { signal });
      return {
        items: result.items ?? [],
        used_bytes: Number(result.used_bytes ?? result.usedBytes ?? 0),
        limit_bytes: Number(result.limit_bytes ?? result.limitBytes ?? 0),
      };
    },
    getWorkspaceFile(id, path, signal) { return call(`/api/v1/workflows/${encodeURIComponent(id)}/workspace/file?path=${encodeURIComponent(path)}`, { signal }); },
    downloadWorkspaceFile(id, path, signal) { return download(`/api/v1/workflows/${encodeURIComponent(id)}/workspace/download?path=${encodeURIComponent(path)}`, signal); },
    configureWorkflowGitSource(id, input, signal) { return call(`/api/v1/workflows/${encodeURIComponent(id)}/git-source`, json("PUT", input, signal)); },
    async listExperts(signal) {
      const items = (await call<{ items: Expert[] }>("/api/v1/experts", { signal })).items ?? [];
      return items.map(normalizeExpert);
    },
    async getExpert(id, signal) { return normalizeExpert(await call(`/api/v1/experts/${encodeURIComponent(id)}`, { signal })); },
    async createExpert(input, signal) { return normalizeExpert(await call("/api/v1/experts", json("POST", { expert: input }, signal))); },
    async updateExpert(id, input, version, signal) { return normalizeExpert(await call(`/api/v1/experts/${encodeURIComponent(id)}`, json("PATCH", { expert: input, expected_version: version }, signal))); },
    deleteExpert(id, signal) { return remove(`/api/v1/experts/${encodeURIComponent(id)}`, signal); },
    async listExpertTeams(signal) {
      const items = (await call<{ items: ExpertTeam[] }>("/api/v1/expert-teams", { signal })).items ?? [];
      return items.map(normalizeExpertTeam);
    },
    async getExpertTeam(id, signal) { return normalizeExpertTeam(await call(`/api/v1/expert-teams/${encodeURIComponent(id)}`, { signal })); },
    async createExpertTeam(input, signal) { return normalizeExpertTeam(await call("/api/v1/expert-teams", json("POST", { expert_team: input }, signal))); },
    async updateExpertTeam(id, input, version, signal) { return normalizeExpertTeam(await call(`/api/v1/expert-teams/${encodeURIComponent(id)}`, json("PATCH", { expert_team: input, expected_version: version }, signal))); },
    deleteExpertTeam(id, signal) { return remove(`/api/v1/expert-teams/${encodeURIComponent(id)}`, signal); },
    async getSettings(signal) {
      const settings = await call<PersonalSettings>("/api/v1/settings", { signal });
      return { ...settings, runtime_model_defaults: settings.runtime_model_defaults ?? [] };
    },
    updateSettings(settings, signal) { const { version, ...values } = settings; return call("/api/v1/settings", json("PATCH", { ...values, expected_version: version }, signal)); },
    async listRuntimeEngines(signal) { return (await call<{ items: RuntimeEngineStatus[] }>("/api/v1/runtime-engines", { signal })).items ?? []; },
    async listModelProviderPresets(signal) { return (await call<{ items: ModelProviderPreset[] }>("/api/v1/model-provider-presets", { signal })).items ?? []; },
    async listModelProviderConnections(signal) {
      const items = (await call<{ items: ModelProviderConnection[] }>("/api/v1/model-provider-connections", { signal })).items ?? [];
      return items.map((connection) => ({
        ...connection,
        protocols: connection.protocols ?? [],
        models: (connection.models ?? []).map((model) => ({ ...model, compatibility: model.compatibility ?? [] })),
      }));
    },
    createModelProviderConnection(input, signal) { return call("/api/v1/model-provider-connections", json("POST", input, signal)); },
    updateModelProviderConnection(id, input, version, signal) {
      const { name, endpoint, protocols, api_key } = input;
      return call(`/api/v1/model-provider-connections/${encodeURIComponent(id)}`, json("PATCH", { name, endpoint, protocols, replacement_api_key: api_key || undefined, expected_version: version }, signal));
    },
    deleteModelProviderConnection(id, signal) { return remove(`/api/v1/model-provider-connections/${encodeURIComponent(id)}`, signal); },
    refreshProviderModels(id, signal) { return call(`/api/v1/model-provider-connections/${encodeURIComponent(id)}/refresh`, json("POST", {}, signal)); },
    createProviderModel(connectionID, input, signal) { return call(`/api/v1/model-provider-connections/${encodeURIComponent(connectionID)}/models`, json("POST", input, signal)); },
    async listMCPServers(signal) { return (await call<{ items: MCPServer[] }>("/api/v1/connectors/mcp", { signal })).items ?? []; },
    createMCPServer(input, signal) { return call("/api/v1/connectors/mcp", json("POST", { mcp_server: input }, signal)); },
    updateMCPServer(id, input, version, signal) { return call(`/api/v1/connectors/mcp/${encodeURIComponent(id)}`, json("PATCH", { mcp_server: input, expected_version: version }, signal)); },
    testMCPServer(id, signal) { return call(`/api/v1/connectors/mcp/${encodeURIComponent(id)}/test`, json("POST", {}, signal)); },
    getMCPConnectorDeletionImpact(id, signal) { return call(`/api/v1/connectors/mcp/${encodeURIComponent(id)}/deletion-impact`, { signal }); },
    deleteMCPServer(id, confirmationToken, signal) { return remove(`/api/v1/connectors/mcp/${encodeURIComponent(id)}?confirmation_token=${encodeURIComponent(confirmationToken)}`, signal); },
    async listSkills(signal) { return (await call<{ items: Skill[] }>("/api/v1/skills", { signal })).items ?? []; },
    createGitSkill(input, signal) { return call("/api/v1/skills", json("POST", { name: input.name, source: "git", git_url: input.git_url, git_ref: input.git_ref }, signal)); },
    createUploadSkill(input, signal) { return call("/api/v1/skills", json("POST", { name: input.name, source: "upload", archive: input.archive }, signal)); },
    updateSkill(id, input, version, signal) { return call(`/api/v1/skills/${encodeURIComponent(id)}`, json("PATCH", { ...input, expected_version: version }, signal)); },
    getSkillDeletionImpact(id, signal) { return call(`/api/v1/skills/${encodeURIComponent(id)}/deletion-impact`, { signal }); },
    deleteSkill(id, confirmationToken, signal) { return remove(`/api/v1/skills/${encodeURIComponent(id)}?confirmation_token=${encodeURIComponent(confirmationToken)}`, signal); },
    async listCLIConnectorDefinitions(signal) { return (await call<{ items: CLIConnectorDefinition[] }>("/api/v1/connectors/cli", { signal })).items ?? []; },
    createCLIConnectorDefinition(input, signal) { return call("/api/v1/admin/connectors/cli", json("POST", { definition: input }, signal)); },
    updateCLIConnectorDefinition(id, input, version, signal) { return call(`/api/v1/admin/connectors/cli/${encodeURIComponent(id)}`, json("PATCH", { definition: input, expected_version: version }, signal)); },
    publishCLIConnectorDefinition(id, version, signal) { return call(`/api/v1/admin/connectors/cli/${encodeURIComponent(id)}/publish`, json("POST", { expected_version: version }, signal)); },
    disableCLIConnectorDefinition(id, version, signal) { return call(`/api/v1/admin/connectors/cli/${encodeURIComponent(id)}/disable`, json("POST", { expected_version: version }, signal)); },
    enableCLIConnector(id, signal) { return call(`/api/v1/connectors/cli/${encodeURIComponent(id)}/enable`, json("POST", {}, signal)); },
    async listCLIConnectorEnablements(signal) { return (await call<{ items: CLIConnectorEnablement[] }>("/api/v1/connectors/cli/enablements", { signal })).items ?? []; },
    async listCommandApprovals(signal) { return (await call<{ items: CommandApproval[] }>("/api/v1/command-approvals", { signal })).items ?? []; },
    decideCommandApproval(id, decision, identity, version, signal) { return call(`/api/v1/command-approvals/${encodeURIComponent(id)}/decision`, json("POST", { decision, identity, expected_version: version }, signal)); },
    async listUsers(signal) { return (await call<{ items: UserAccount[] }>("/api/v1/admin/users", { signal })).items ?? []; },
    createUser(input, signal) { return call("/api/v1/admin/users", json("POST", input, signal)); },
    setUserEnabled(id, enabled, version, signal) { return call(`/api/v1/admin/users/${encodeURIComponent(id)}/enabled`, json("PATCH", { enabled, expected_version: version }, signal)); },
    resetUserPassword(id, signal) { return call(`/api/v1/admin/users/${encodeURIComponent(id)}/password-reset`, json("POST", {}, signal)); },
  };
}

function normalizeExpert(expert: Expert): Expert {
  return { ...expert, expertise_tags: expert.expertise_tags ?? [], mcp_server_ids: expert.mcp_server_ids ?? [], skill_ids: expert.skill_ids ?? [], cli_connector_definition_ids: expert.cli_connector_definition_ids ?? [] };
}

function normalizeExpertTeam(team: ExpertTeam): ExpertTeam {
  return { ...team, expertise_tags: team.expertise_tags ?? [], experts: (team.experts ?? []).map(normalizeExpert), members: (team.members ?? []).map((member) => ({ ...member, labels: member.labels ?? [], expert: normalizeExpert(member.expert) })) };
}

async function request<T>(accessToken: string, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(path, { ...init, headers });
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { reason?: string; message?: string; error?: string };
    const code = body.reason ?? body.error ?? body.message ?? `request_failed_${response.status}`;
    const kind: ApiErrorKind = response.status === 401 ? "unauthenticated" : response.status === 403 ? "forbidden" : response.status === 404 ? "not_found" : response.status === 409 || response.status === 412 ? "conflict" : response.status === 400 || response.status === 422 ? "validation" : response.status === 429 ? "rate_limited" : response.status >= 500 ? "unavailable" : "unknown";
    throw new ApiError(kind, response.status, code, response.headers.get("X-Request-ID") ?? "");
  }
  if (response.status === 204) return undefined as T;
  return response.json().then(normalizeTimestamps) as Promise<T>;
}

function normalizeTimestamps<T>(value: T): T {
  if (Array.isArray(value)) return value.map(normalizeTimestamps) as T;
  if (!value || typeof value !== "object") return value;
  const normalized: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    normalized[key] = key.endsWith("_at") ? timestampString(item) : normalizeTimestamps(item);
  }
  return normalized as T;
}

function timestampString(value: unknown): unknown {
  if (typeof value === "string" || value === undefined || value === null) return value;
  if (typeof value !== "object" || Array.isArray(value)) return normalizeTimestamps(value);
  const timestamp = value as { seconds?: number | string; nanos?: number };
  if (timestamp.seconds === undefined) return normalizeTimestamps(value);
  const milliseconds = Number(timestamp.seconds) * 1000 + Math.floor((timestamp.nanos ?? 0) / 1_000_000);
  return Number.isFinite(milliseconds) ? new Date(milliseconds).toISOString() : value;
}

function normalizeRun(item: Run & { elapsed_ms?: number | string }): Run {
  const elapsed = Number(item.elapsed_ms ?? 0);
  return { ...item, attachments: item.attachments ?? [], expert_stages: item.expert_stages ?? [], elapsed_ms: Number.isFinite(elapsed) && elapsed >= 0 ? elapsed : 0 };
}
