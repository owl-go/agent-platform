import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import {
  ApiError, platformApiKey, type ConfiguredModel, type PlatformApi, type RepositoryBinding,
  type RuntimeImage, type SourceControlProvider,
} from "../api/client";
import { authContextKey, type AuthSession, type AuthState } from "../auth/session";
import { createAppI18n } from "../i18n";
import RepositoryBindingPanel from "./RepositoryBindingPanel.vue";

const provider: SourceControlProvider = { id: "provider-1", name: "GitHub", kind: "github_com", base_url: "https://github.com", enabled: true, version: 1 };
const runtime: RuntimeImage = { id: "runtime-1", runtime: "codex", cli_version: "1", adapter_version: "1", image_digest: "registry/repo@sha256:" + "a".repeat(64), status: "production", version: 1 };
const model: ConfiguredModel = { id: "model-1", name: "Primary", model_id: "model-v1", endpoint: "https://models.example.test", credential_profile_id: "credential-1", enabled: true, version: 1 };
const binding: RepositoryBinding = {
  id: "binding-1", team_id: "team-1", source_control_provider_id: "provider-1", name: "agent-platform",
  repository_ssh_url: "git@github.com:acme/agent-platform.git", default_branch: "main", ssh_credential_profile_id: "ssh-credential-1",
  build_credential_profile_ids: ["build-credential-1"], git_author_name: "Agent Platform", git_author_email: "agent@example.test",
  allowed_runtime_image_ids: ["runtime-1"], default_runtime_image_id: "runtime-1", default_model_id: "model-1",
  model_budget: { max_input_tokens: 1000, max_output_tokens: 500, max_cost_amount: "10.00" }, instructions: "Follow AGENTS.md",
  quality_commands: [{ name: "test", kind: "test", executable: "go", arguments: ["test", "./..."], timeout_seconds: 600 }],
  egress_policy: { mode: "public" }, validation_report: { valid: false, errors: { allowed_runtime_image_ids: "Runtime is not Production" } }, version: 2,
};

describe("RepositoryBindingPanel", () => {
  it("renders real safe Provider and Binding metadata", async () => {
    const wrapper = await mountPanel(apiStub(), [{ role: "platform_administrator" }]);
    expect(wrapper.text()).toContain("git@github.com:acme/agent-platform.git");
    expect(wrapper.text()).toContain("ssh-credential-1");
    expect(wrapper.text()).toContain("allowed_runtime_image_ids");
    expect(wrapper.text()).not.toContain("PRIVATE KEY");
    expect(wrapper.text()).not.toContain("ssh-ed25519 AAAAC3");
  });

  it("keeps a stable Provider registration intent across retry", async () => {
    const api = apiStub();
    vi.mocked(api.registerSourceControlProvider).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(provider);
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='register-provider']").trigger("click");
    await wrapper.get("[data-testid='provider-name']").setValue("GitHub");
    await wrapper.get("[data-testid='provider-form']").trigger("submit");
    await flushPromises();
    await wrapper.get("[data-testid='provider-form']").trigger("submit");
    await flushPromises();
    const calls = vi.mocked(api.registerSourceControlProvider).mock.calls;
    expect(calls[1]![1]).toBe(calls[0]![1]);
  });

  it("submits governed Repository Binding configuration", async () => {
    const api = apiStub();
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='register-binding']").trigger("click");
    await wrapper.get("[data-testid='binding-name']").setValue("new-binding");
    await wrapper.get("[data-testid='binding-repository-url']").setValue("git@github.com:acme/new.git");
    await wrapper.get("[data-testid='binding-ssh-credential']").setValue("ssh-reference");
    await wrapper.get("[data-testid='binding-author-email']").setValue("agent@example.test");
    await wrapper.get("[data-testid='binding-runtime-runtime-1']").trigger("change");
    await wrapper.get("[data-testid='binding-capability-streaming']").trigger("change");
    await wrapper.get("[data-testid='binding-quality-executable']").setValue("go");
    await wrapper.findAll(".quality-arguments button").at(-1)!.trigger("click");
    await wrapper.get("[data-testid='binding-quality-argument-0-0']").setValue("test");
    await wrapper.findAll(".quality-arguments button").at(-1)!.trigger("click");
    await wrapper.get("[data-testid='binding-quality-argument-0-1']").setValue("./...");
    await wrapper.get("[data-testid='binding-form']").trigger("submit");
    await flushPromises();

    expect(api.registerRepositoryBinding).toHaveBeenCalledWith(expect.objectContaining({
      team_id: "team-1", source_control_provider_id: "provider-1", repository_ssh_url: "git@github.com:acme/new.git",
      ssh_credential_profile_id: "ssh-reference", allowed_runtime_image_ids: ["runtime-1"], default_runtime_image_id: "runtime-1",
      required_runtime_capabilities: ["streaming"],
      default_model_id: "model-1", egress_policy: { mode: "public" },
      quality_commands: [expect.objectContaining({ executable: "go", arguments: ["test", "./..."] })],
    }), expect.any(String));
  });

  it("keeps non-Organization administrators read-only", async () => {
    const wrapper = await mountPanel(apiStub(), [{ role: "platform_administrator", team_id: "team-1" }]);
    expect(wrapper.text()).toContain("Read-only repository catalog");
    expect(wrapper.find("[data-testid='register-provider']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='edit-binding-binding-1']").exists()).toBe(false);
  });

  it("preserves every structured quality command and argument while editing", async () => {
    const api = apiStub();
    const commands = [
      { name: "lint", kind: "lint", executable: "golangci-lint", arguments: ["run", "--build-tags=unit test"], timeout_seconds: 600 },
      { name: "test", kind: "test", executable: "go", arguments: ["test", "./..."], timeout_seconds: 900 },
    ];
    vi.mocked(api.listRepositoryBindings).mockResolvedValue([{ ...binding, quality_commands: commands }]);
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='edit-binding-binding-1']").trigger("click");
    await wrapper.get("[data-testid='binding-form']").trigger("submit");
    await flushPromises();

    expect(vi.mocked(api.updateRepositoryBinding).mock.calls[0]![1].quality_commands).toEqual(commands);
  });

  it("reloads Team-scoped Bindings when the active Team changes", async () => {
    const api = apiStub();
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.vm.$router.push("/studio?team=team-2");
    await flushPromises();

    expect(api.listRepositoryBindings).toHaveBeenNthCalledWith(1, "team-1");
    expect(api.listRepositoryBindings).toHaveBeenNthCalledWith(2, "team-2");
  });

  it("does not let a stale Team response replace the active Team", async () => {
    const api = apiStub();
    let resolveFirst!: (value: RepositoryBinding[]) => void;
    vi.mocked(api.listRepositoryBindings)
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve; }))
      .mockResolvedValueOnce([{ ...binding, team_id: "team-2", name: "team-two" }]);
    const mounting = mountPanel(api, [{ role: "platform_administrator" }]);
    await Promise.resolve();
    const wrapper = await mounting;
    await wrapper.vm.$router.push("/studio?team=team-2");
    await flushPromises();
    resolveFirst([{ ...binding, name: "stale-team-one" }]);
    await flushPromises();

    expect(wrapper.text()).toContain("team-two");
    expect(wrapper.text()).not.toContain("stale-team-one");
  });

  it("loads every Runtime Image page for Binding choices", async () => {
    const api = apiStub();
    vi.mocked(api.listRuntimeImages).mockImplementation(async (token) => token
      ? { items: [{ ...runtime, id: "runtime-2", runtime: "claude" }], nextPageToken: "" }
      : { items: [runtime], nextPageToken: "page-2" });
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='register-binding']").trigger("click");

    expect(wrapper.findAll(".runtime-checks input")).toHaveLength(2);
    expect(api.listRuntimeImages).toHaveBeenNthCalledWith(1, "", 100);
    expect(api.listRuntimeImages).toHaveBeenNthCalledWith(2, "page-2", 100);
  });

  it("preserves safe input and reloads the authoritative Version after a conflict", async () => {
    const api = apiStub();
    vi.mocked(api.listRepositoryBindings)
      .mockResolvedValueOnce([binding])
      .mockResolvedValue([{ ...binding, version: 3 }]);
    vi.mocked(api.updateRepositoryBinding)
      .mockRejectedValueOnce(new ApiError("conflict", 412, "version_conflict", "request-1"))
      .mockResolvedValueOnce({ ...binding, version: 4 });
    const wrapper = await mountPanel(api, [{ role: "platform_administrator" }]);
    await wrapper.get("[data-testid='edit-binding-binding-1']").trigger("click");
    await wrapper.get("[data-testid='binding-name']").setValue("preserved-name");
    await wrapper.get("[data-testid='binding-form']").trigger("submit");
    await flushPromises();

    expect(wrapper.get("[role='alert']").text()).toContain("authoritative server Version was reloaded");
    expect((wrapper.get("[data-testid='binding-name']").element as HTMLInputElement).value).toBe("preserved-name");

    await wrapper.get("[data-testid='binding-form']").trigger("submit");
    await flushPromises();
    expect(vi.mocked(api.updateRepositoryBinding).mock.calls[1]![2]).toBe(3);
  });
});

async function mountPanel(api: PlatformApi, grants: Array<{ role: string; team_id?: string }>) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/studio", component: RepositoryBindingPanel }] });
  await router.push("/studio?team=team-1");
  await router.isReady();
  const state = ref<AuthState>({ kind: "authenticated", currentUser: {
    user_id: "user-1", organization: { id: "org-1", slug: "acme", name: "Acme" }, role_grants: grants,
    teams: [{ id: "team-1", slug: "platform", name: "Platform" }],
  } });
  const session: AuthSession = { state, accessToken: () => "token", initialize: vi.fn(), signIn: vi.fn(), signOut: vi.fn(), dispose: vi.fn() };
  const wrapper = mount(RepositoryBindingPanel, { global: { plugins: [router, createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: {
    [platformApiKey as symbol]: api, [authContextKey as symbol]: { session, isCallback: false },
  } } });
  await flushPromises();
  return wrapper;
}

function apiStub(): PlatformApi {
  return {
    listRuntimeImages: vi.fn(async () => ({ items: [runtime], nextPageToken: "" })), getRuntimeImage: vi.fn(), registerRuntimeImage: vi.fn(), changeRuntimeImageStatus: vi.fn(),
    listCredentialProfiles: vi.fn(async () => []), getCredentialProfile: vi.fn(), registerCredentialProfile: vi.fn(), changeCredentialProfileStatus: vi.fn(),
    listConfiguredModels: vi.fn(async () => [model]), getConfiguredModel: vi.fn(), registerConfiguredModel: vi.fn(), changeConfiguredModelStatus: vi.fn(),
    listSourceControlProviders: vi.fn(async () => [provider]), getSourceControlProvider: vi.fn(), registerSourceControlProvider: vi.fn(async () => provider), changeSourceControlProviderStatus: vi.fn(async () => provider),
    listRepositoryBindings: vi.fn(async () => [binding]), getRepositoryBinding: vi.fn(), registerRepositoryBinding: vi.fn(async () => binding), updateRepositoryBinding: vi.fn(async () => binding), validateRepositoryBinding: vi.fn(async () => binding),
    listAgents: vi.fn(async () => []), getAgent: vi.fn(), createAgent: vi.fn(), updateAgent: vi.fn(), listAgentDrafts: vi.fn(async () => []), getAgentDraft: vi.fn(), createAgentDraft: vi.fn(), updateAgentDraft: vi.fn(), validateAgentDraft: vi.fn(), getAgentDraftApproval: vi.fn(), requestAgentDraftApproval: vi.fn(), decideAgentDraftApproval: vi.fn(), publishAgentDraft: vi.fn(), listAgentReleases: vi.fn(async () => []), getAgentRelease: vi.fn(), deprecateAgentRelease: vi.fn(), blockAgentRelease: vi.fn(),
    listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [], prerequisite: "release" })), listCodingTasks: vi.fn(async () => []), getCodingTask: vi.fn(), createCodingTask: vi.fn(), getCodingTaskSession: vi.fn(), listRuns: vi.fn(async () => []), getRun: vi.fn(), listRunApprovals: vi.fn(async () => []), decideRunApproval: vi.fn(), controlRun: vi.fn(), streamRunEvents: vi.fn(async () => ({ cursor: 0, terminal: true })), listRunArtifacts: vi.fn(async () => []), getArtifactDownload: vi.fn(),
  };
}
