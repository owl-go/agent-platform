import { flushPromises, mount } from "@vue/test-utils";
import { nextTick, ref } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type Agent, type AgentDraft, type AgentRelease, type PlatformApi, type ReleaseApproval, type RepositoryBinding, type RuntimeImage, type ConfiguredModel } from "../api/client";
import { authContextKey, type AuthSession, type AuthState } from "../auth/session";
import { createAppI18n } from "../i18n";
import AgentDraftPanel from "./AgentDraftPanel.vue";

const agent: Agent = { id: "agent-1", team_id: "team-1", name: "Coding Agent", description: "Ships code", created_by: "user-1", version: 1 };
const runtime: RuntimeImage = { id: "runtime-1", runtime: "codex", cli_version: "1", adapter_version: "1", image_digest: "registry/runtime@sha256:" + "a".repeat(64), status: "production", capabilities: { subagents: false }, version: 1 };
const model: ConfiguredModel = { id: "model-1", name: "Primary", model_id: "model", endpoint: "https://models.example", credential_profile_id: "credential-1", enabled: true, version: 1 };
const binding: RepositoryBinding = { id: "binding-1", team_id: "team-1", name: "Repository", validation_report: { valid: true, errors: {} }, version: 2 };
const draft: AgentDraft = { id: "draft-1", agent_id: "agent-1", revision: 1, state: "blocked", release_risk: "low", version: 3, configuration: {
  instructions: "Implement carefully", repository_binding_id: "binding-1", runtime_image_id: "runtime-1", configured_model_id: "model-1",
  model_budget: { max_input_tokens: 1000, max_output_tokens: 500, max_cost_amount: "10.00" }, execution_limits: { timeout_seconds: 1800, cpus: 2, memory_bytes: 4096, pids: 64, temp_bytes: 8192, egress: "public" }, native_subagents: false,
}, validation_report: { valid: false, errors: { model_budget: "Draft Model Budget exceeds Repository Binding limits" } } };
const readyDraft: AgentDraft = { ...draft, state: "ready", validation_report: { valid: true, errors: {} } };
const highDraft: AgentDraft = { ...readyDraft, release_risk: "high" };
const approval: ReleaseApproval = { id: "approval-1", draft_id: "draft-1", draft_version: 3, requested_by: "user-2", risk_reason: "Runtime-native Subagents", state: "pending", version: 1 };
const release: AgentRelease = { id: "release-1", agent_id: "agent-1", release_number: 1, source_draft_id: "draft-1", status: "released", release_risk: "high", version: 1, configuration: readyDraft.configuration, repository_binding_snapshot: { id: "binding-1", name: "Repository", default_branch: "main", repository_ssh_url: "git@example.test:acme/repository.git", egress_policy: "public" }, runtime_image_snapshot: { id: "runtime-1", runtime: "codex", cli_version: "1", adapter_version: "1", image_digest: "registry/runtime@sha256:" + "a".repeat(64), capabilities: { streaming: true } }, configured_model_snapshot: { id: "model-1", name: "Primary", model_id: "model", endpoint: "https://models.example" }, approval_evidence: { id: "approval-1", draft_id: "draft-1", draft_version: 3, requested_by: "user-2", risk_reason: "Runtime-native Subagents", approved_by: "user-1" } };

describe("AgentDraftPanel", () => {
  it("renders Team-scoped Agents, Drafts, and field validation from real API data", async () => {
    const wrapper = await mountPanel(apiStub(), [{ role: "agent_builder", team_id: "team-1" }]);
    expect(wrapper.text()).toContain("Coding Agent");
    expect(wrapper.text()).toContain("model_budget");
    expect(wrapper.text()).toContain("Draft Model Budget exceeds Repository Binding limits");
  });

  it("keeps Agent creation intent stable across retry", async () => {
    const api = apiStub();
    vi.mocked(api.createAgent).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(agent);
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='create-agent']").trigger("click");
    await wrapper.get("[data-testid='agent-name']").setValue("Coding Agent");
    await wrapper.get("[data-testid='agent-form']").trigger("submit"); await flushPromises();
    await wrapper.get("[data-testid='agent-form']").trigger("submit"); await flushPromises();
    expect(vi.mocked(api.createAgent).mock.calls[0]![2]).toBe(vi.mocked(api.createAgent).mock.calls[1]![2]);
  });

  it("starts a new Agent creation intent after the active Team changes", async () => {
    const api = apiStub();
    vi.mocked(api.createAgent).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(agent);
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='create-agent']").trigger("click");
    await wrapper.get("[data-testid='agent-name']").setValue("Coding Agent");
    await wrapper.get("[data-testid='agent-form']").trigger("submit"); await flushPromises();
    await wrapper.vm.$router.push("/studio?team=team-2"); await flushPromises();
    await wrapper.get("[data-testid='agent-form']").trigger("submit"); await flushPromises();
    expect(vi.mocked(api.createAgent).mock.calls[0]![2]).not.toBe(vi.mocked(api.createAgent).mock.calls[1]![2]);
  });

  it("submits governed Draft configuration and validation with Version", async () => {
    const api = apiStub();
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='create-draft']").trigger("click");
    await wrapper.get("[data-testid='draft-instructions']").setValue("Implement and verify");
    await wrapper.get("[data-testid='draft-form']").trigger("submit"); await flushPromises();
    expect(api.createAgentDraft).toHaveBeenCalledWith("agent-1", "team-1", expect.objectContaining({ configuration: expect.objectContaining({ repository_binding_id: "binding-1", execution_limits: expect.objectContaining({ egress: "public" }) }) }), expect.any(String));
    await wrapper.get("[data-testid='validate-draft-draft-1']").trigger("click"); await flushPromises();
    expect(api.validateAgentDraft).toHaveBeenCalledWith("agent-1", "draft-1", "team-1", 3, expect.any(String));
  });

  it("preserves safe Draft input and reloads authoritative Version after conflict", async () => {
    const api = apiStub();
    vi.mocked(api.updateAgentDraft).mockRejectedValueOnce(new ApiError("conflict", 412, "version_conflict", "request-1")).mockResolvedValueOnce({ ...draft, version: 5 });
    vi.mocked(api.getAgentDraft).mockResolvedValue({ ...draft, version: 4 });
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='edit-draft-draft-1']").trigger("click");
    await wrapper.get("[data-testid='draft-instructions']").setValue("Preserved instructions");
    await wrapper.get("[data-testid='draft-form']").trigger("submit"); await flushPromises();
    expect((wrapper.get("[data-testid='draft-instructions']").element as HTMLTextAreaElement).value).toBe("Preserved instructions");
    expect(wrapper.get("[role='alert']").text()).toContain("authoritative server Version");
    await wrapper.get("[data-testid='draft-form']").trigger("submit"); await flushPromises();
    expect(vi.mocked(api.updateAgentDraft).mock.calls[1]![4]).toBe(4);
  });

  it("keeps Agent Users read-only", async () => {
    const wrapper = await mountPanel(apiStub(), [{ role: "agent_user", team_id: "team-1" }]);
    expect(wrapper.text()).toContain("Read-only Agent catalog");
    expect(wrapper.find("[data-testid='create-agent']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='create-draft']").exists()).toBe(false);
  });

  it("clears old Drafts immediately when the active Team changes", async () => {
    const api = apiStub();
    let resolveTeamAgents!: (value: Agent[]) => void;
    vi.mocked(api.listAgents).mockImplementation(async (teamID) => teamID === "team-1" ? [agent] : new Promise((resolve) => { resolveTeamAgents = resolve; }));
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    expect(wrapper.find("[data-testid='draft-draft-1']").exists()).toBe(true);
    await wrapper.vm.$router.push("/studio?team=team-2"); await nextTick();
    expect(wrapper.find("[data-testid='draft-draft-1']").exists()).toBe(false);
    resolveTeamAgents([]); await flushPromises();
  });

  it("shows the localized validating state while validation is in flight", async () => {
    const api = apiStub();
    let resolveValidation!: (value: AgentDraft) => void;
    vi.mocked(api.validateAgentDraft).mockImplementation(() => new Promise((resolve) => { resolveValidation = resolve; }));
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='validate-draft-draft-1']").trigger("click"); await nextTick();
    expect(wrapper.get("[data-testid='draft-draft-1']").text()).toContain("Validating");
    resolveValidation({ ...draft, state: "ready", version: 5 }); await flushPromises();
  });

  it("publishes a ready low-risk Draft with a stable intent", async () => {
    const api = apiStub();
    vi.mocked(api.listAgentDrafts).mockResolvedValue([readyDraft]);
    vi.mocked(api.publishAgentDraft).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(release);
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='publish-draft-draft-1']").trigger("click"); await flushPromises();
    await wrapper.get("[data-testid='publish-draft-draft-1']").trigger("click"); await flushPromises();
    expect(vi.mocked(api.publishAgentDraft).mock.calls[0]![3]).toBe(vi.mocked(api.publishAgentDraft).mock.calls[1]![3]);
  });

  it("lets a different Builder decide an exact-version Release Approval", async () => {
    const api = apiStub();
    vi.mocked(api.listAgentDrafts).mockResolvedValue([highDraft]);
    vi.mocked(api.getAgentDraftApproval).mockResolvedValue(approval);
    vi.mocked(api.decideAgentDraftApproval).mockResolvedValue({ ...approval, state: "approved", decided_by: "user-1", version: 2 });
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    await wrapper.get("[data-testid='approve-release-draft-1']").trigger("click");
    await wrapper.get("[data-testid='release-decision-form']").trigger("submit"); await flushPromises();
    expect(api.decideAgentDraftApproval).toHaveBeenCalledWith("agent-1", "draft-1", "team-1", true, "", 1, expect.any(String));
  });

  it("marks an old Approval expired and requires a new request", async () => {
    const api = apiStub();
    vi.mocked(api.listAgentDrafts).mockResolvedValue([highDraft]);
    vi.mocked(api.getAgentDraftApproval).mockResolvedValue({ ...approval, draft_version: 2, state: "approved" });
    const wrapper = await mountPanel(api, [{ role: "agent_builder", team_id: "team-1" }]);
    expect(wrapper.get("[data-testid='release-approval-draft-1']").text()).toContain("older Draft Version");
    expect(wrapper.find("[data-testid='publish-draft-draft-1']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='request-release-approval-draft-1']").exists()).toBe(true);
  });

  it("renders immutable snapshots and restricts Block to Organization administrators", async () => {
    const api = apiStub();
    vi.mocked(api.listAgentReleases).mockResolvedValue([release]);
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    expect(wrapper.get("[data-testid='release-release-1']").text()).toContain("registry/runtime@sha256");
    expect(wrapper.get("[data-testid='release-release-1']").text()).toContain("Runtime-native Subagents");
    expect(wrapper.find("[data-testid='block-release-release-1']").exists()).toBe(true);
  });
});

async function mountPanel(api: PlatformApi, grants: Array<{ role: string; team_id?: string }>) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/studio", component: AgentDraftPanel }] });
  await router.push("/studio?team=team-1"); await router.isReady();
  const state = ref<AuthState>({ kind: "authenticated", currentUser: { user_id: "user-1", organization: { id: "org-1", slug: "acme", name: "Acme" }, role_grants: grants, teams: [{ id: "team-1", slug: "team", name: "Team" }] } });
  const session: AuthSession = { state, accessToken: () => "token", initialize: vi.fn(), signIn: vi.fn(), signOut: vi.fn(), dispose: vi.fn() };
  const wrapper = mount(AgentDraftPanel, { global: { plugins: [router, createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: { session, isCallback: false } } } });
  await flushPromises(); return wrapper;
}

function apiStub(): PlatformApi {
  return {
    listRuntimeImages: vi.fn(async () => ({ items: [runtime], nextPageToken: "" })), getRuntimeImage: vi.fn(), registerRuntimeImage: vi.fn(), changeRuntimeImageStatus: vi.fn(),
    listCredentialProfiles: vi.fn(async () => []), getCredentialProfile: vi.fn(), registerCredentialProfile: vi.fn(), changeCredentialProfileStatus: vi.fn(), listConfiguredModels: vi.fn(async () => [model]), getConfiguredModel: vi.fn(), registerConfiguredModel: vi.fn(), changeConfiguredModelStatus: vi.fn(),
    listSourceControlProviders: vi.fn(async () => []), getSourceControlProvider: vi.fn(), registerSourceControlProvider: vi.fn(), changeSourceControlProviderStatus: vi.fn(), listRepositoryBindings: vi.fn(async () => [binding]), getRepositoryBinding: vi.fn(), registerRepositoryBinding: vi.fn(), updateRepositoryBinding: vi.fn(), validateRepositoryBinding: vi.fn(),
    listAgents: vi.fn(async () => [agent]), getAgent: vi.fn(), createAgent: vi.fn(async () => agent), updateAgent: vi.fn(), listAgentDrafts: vi.fn(async () => [draft]), getAgentDraft: vi.fn(), createAgentDraft: vi.fn(async () => draft), updateAgentDraft: vi.fn(async () => draft), validateAgentDraft: vi.fn(async () => draft), getAgentDraftApproval: vi.fn(), requestAgentDraftApproval: vi.fn(), decideAgentDraftApproval: vi.fn(), publishAgentDraft: vi.fn(), listAgentReleases: vi.fn(async () => []), getAgentRelease: vi.fn(), deprecateAgentRelease: vi.fn(), blockAgentRelease: vi.fn(),
    listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [], prerequisite: "release" })), listCodingTasks: vi.fn(async () => []), getCodingTask: vi.fn(), createCodingTask: vi.fn(), getCodingTaskSession: vi.fn(), listRuns: vi.fn(async () => []), getRun: vi.fn(), listRunApprovals: vi.fn(async () => []), decideRunApproval: vi.fn(), controlRun: vi.fn(), streamRunEvents: vi.fn(async () => ({ cursor: 0, terminal: true })), listRunArtifacts: vi.fn(async () => []), getArtifactDownload: vi.fn(),
    updateCodingTaskState: vi.fn(), continueCodingTask: vi.fn(), listSessionMessages: vi.fn(async () => []), updateSessionMemory: vi.fn(), listMemoryCandidates: vi.fn(async () => []), decideMemoryCandidate: vi.fn(), listAgentMemories: vi.fn(async () => []), updateAgentMemory: vi.fn(), deleteAgentMemory: vi.fn(),
  };
}
