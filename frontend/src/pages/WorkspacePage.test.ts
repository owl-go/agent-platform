import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type PlatformApi } from "../api/client";
import { authContextKey, type AuthSession, type AuthState } from "../auth/session";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import WorkspacePage from "./WorkspacePage.vue";

const binding = { id: "binding-1", name: "Payments API", validation_report: { valid: true, errors: {} } };
const runtime = { id: "runtime-1", runtime: "codex", status: "production" };
const model = { id: "model-1", name: "Primary model", enabled: true };
const agent = { id: "agent-1", name: "Coding Agent" };
const release = {
  id: "release-1", agent_id: "agent-1", release_number: 3, status: "released", release_risk: "low",
  repository_binding_id: "binding-1", runtime_image_id: "runtime-1", configured_model_id: "model-1",
  repository_binding_snapshot: { id: "binding-1", name: "Payments API" },
  runtime_image_snapshot: { id: "runtime-1", runtime: "codex", image_digest: "registry.example/codex@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" },
  configured_model_snapshot: { id: "model-1", name: "Primary model", model_id: "gpt-enterprise" },
};
const task = { id: "task-1", team_id: "team-1", agent_release_id: "release-1", title: "Fix parser", request_text: "Handle empty input", state: "active", created_at: "2026-08-20T00:00:00Z", version: 2 };
const session = { id: "session-1", coding_task_id: "task-1", repository_binding_id: "binding-1", target_branch: "main", review_branch: "agent-platform/backend/task-abcd", run_count: 1, version: 2 };
const run = { id: "run-1", session_id: "session-1", agent_release_id: "release-1", runtime_image_id: "runtime-1", state: "queued" };

describe("Conversation Workspace", () => {
  it("restores a direct Coding Task URL from real Task, Session, Run, and frozen Release data", async () => {
    const api = platformApi();
    const { wrapper } = await mountWorkspace(api, "/workspace?team=team-1&task=task-1");

    expect(api.getCodingTaskSession).toHaveBeenCalledWith("task-1", "team-1");
    expect(api.listRuns).toHaveBeenCalledWith("team-1", "task-1");
    expect(wrapper.text()).toContain("agent-platform/backend/task-abcd");
    expect(wrapper.text()).toContain("gpt-enterprise");
    expect(wrapper.text()).toContain("registry.example/codex@sha256");
    expect(wrapper.text()).toContain("run-1");
  });

  it("submits an immutable Issue Snapshot and reuses the same intent key after a network retry", async () => {
    const firstFailure = new ApiError("unavailable", 503, "temporary", "request-1");
    const create = vi.fn()
      .mockRejectedValueOnce(firstFailure)
      .mockResolvedValue({ task: { ...task, id: "task-2", title: "Issue 42" }, session: { ...session, id: "session-2", coding_task_id: "task-2" }, run_id: "run-2" });
    const api = platformApi({ createCodingTask: create, getCodingTask: vi.fn(async () => ({ ...task, id: "task-2", title: "Issue 42" })) });
    const { wrapper } = await mountWorkspace(api);

    await wrapper.findAll(".source-switch button")[1]!.trigger("click");
    await wrapper.get("[data-testid='issue-title']").setValue("Issue 42");
    await wrapper.get("[data-testid='issue-body']").setValue("Repair the parser and add regression coverage.");
    await wrapper.get("[data-testid='issue-url']").setValue("https://git.example/issues/42");
    await wrapper.get(".task-form").trigger("submit");
    await flushPromises();
    await wrapper.get(".task-form").trigger("submit");
    await flushPromises();

    expect(create).toHaveBeenCalledTimes(2);
    expect(create.mock.calls[0]![1]).toMatchObject({
      title: "Issue 42", request_text: "Repair the parser and add regression coverage.",
      issue_snapshot: { title: "Issue 42", body: "Repair the parser and add regression coverage.", url: "https://git.example/issues/42" },
    });
    expect(create.mock.calls[0]![2]).toBe(create.mock.calls[1]![2]);
  });

  it("shows the exact missing Production Runtime prerequisite and prevents a fake launch", async () => {
    const api = platformApi({ listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [], prerequisite: "runtime" })) });
    const { wrapper } = await mountWorkspace(api);

    expect(wrapper.get("[data-testid='launch-prerequisite']").text()).toContain("Production Runtime");
    expect(wrapper.get("[data-testid='create-task']").attributes("disabled")).toBeDefined();
    expect(api.createCodingTask).not.toHaveBeenCalled();
  });

  it("only offers server-authorized launch combinations and keeps historical details on the Release snapshot", async () => {
    const unavailableBinding = { ...binding, id: "binding-2", name: "Unavailable repository" };
    const api = platformApi({
      listRepositoryBindings: vi.fn(async () => [{ ...binding, name: "Renamed current binding" }, unavailableBinding]),
      listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [{ agent_release_id: release.id, repository_binding_id: binding.id }], prerequisite: "" })),
    });
    const { wrapper } = await mountWorkspace(api, "/workspace?team=team-1&task=task-1");

    const choices = wrapper.findAll("[data-testid='binding-select'] option").map((option) => option.text());
    expect(choices).toEqual(["Renamed current binding"]);
    expect(wrapper.find(".workspace-detail").text()).toContain("Payments API");
    expect(wrapper.find(".workspace-detail").text()).not.toContain("Renamed current binding");
  });
});

async function mountWorkspace(api: PlatformApi, path = "/workspace?team=team-1") {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path); await router.isReady();
  const state = ref<AuthState>({ kind: "authenticated", currentUser: {
    user_id: "user-1", email: "user@example.test", display_name: "Agent User",
    organization: { id: "org-1", slug: "acme", name: "Acme" }, teams: [{ id: "team-1", slug: "platform", name: "Platform" }],
    role_grants: [{ role: "agent_user", team_id: "team-1" }],
  } });
  const auth: AuthSession = { state, accessToken: () => "token", initialize: vi.fn(), signIn: vi.fn(), signOut: vi.fn(), dispose: vi.fn() };
  const wrapper = mount(WorkspacePage, { global: { plugins: [router, createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: {
    [platformApiKey as symbol]: api, [authContextKey as symbol]: { session: auth, isCallback: false },
  } } });
  await flushPromises();
  return { wrapper, router };
}

function platformApi(overrides: Partial<PlatformApi> = {}): PlatformApi {
  return {
    listRuntimeImages: vi.fn(async () => ({ items: [runtime], nextPageToken: "" })), getRuntimeImage: vi.fn(), registerRuntimeImage: vi.fn(), changeRuntimeImageStatus: vi.fn(),
    listCredentialProfiles: vi.fn(async () => []), getCredentialProfile: vi.fn(), registerCredentialProfile: vi.fn(), changeCredentialProfileStatus: vi.fn(),
    listConfiguredModels: vi.fn(async () => [model]), getConfiguredModel: vi.fn(), registerConfiguredModel: vi.fn(), changeConfiguredModelStatus: vi.fn(),
    listSourceControlProviders: vi.fn(async () => []), getSourceControlProvider: vi.fn(), registerSourceControlProvider: vi.fn(), changeSourceControlProviderStatus: vi.fn(),
    listRepositoryBindings: vi.fn(async () => [binding]), getRepositoryBinding: vi.fn(), registerRepositoryBinding: vi.fn(), updateRepositoryBinding: vi.fn(), validateRepositoryBinding: vi.fn(),
    listAgents: vi.fn(async () => [agent]), getAgent: vi.fn(), createAgent: vi.fn(), updateAgent: vi.fn(), listAgentDrafts: vi.fn(async () => []), getAgentDraft: vi.fn(), createAgentDraft: vi.fn(), updateAgentDraft: vi.fn(), validateAgentDraft: vi.fn(), getAgentDraftApproval: vi.fn(), requestAgentDraftApproval: vi.fn(), decideAgentDraftApproval: vi.fn(), publishAgentDraft: vi.fn(), listAgentReleases: vi.fn(async () => [release]), getAgentRelease: vi.fn(), deprecateAgentRelease: vi.fn(), blockAgentRelease: vi.fn(),
    listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [{ agent_release_id: release.id, repository_binding_id: binding.id }], prerequisite: "" })), listCodingTasks: vi.fn(async () => [task]), getCodingTask: vi.fn(async () => task), createCodingTask: vi.fn(), getCodingTaskSession: vi.fn(async () => session), listRuns: vi.fn(async () => [run]),
    ...overrides,
  } as PlatformApi;
}
