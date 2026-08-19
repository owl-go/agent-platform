import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type CredentialProfile, type ConfiguredModel, type PlatformApi } from "../api/client";
import { authContextKey, type AuthSession, type AuthState } from "../auth/session";
import { createAppI18n } from "../i18n";
import ModelCatalogPanel from "./ModelCatalogPanel.vue";

const credential: CredentialProfile = { id: "credential-1", name: "primary-key", kind: "model", secret_configured: true, enabled: true, version: 1 };
const model: ConfiguredModel = { id: "model-1", name: "primary", model_id: "model-v1", endpoint: "https://models.example.test/v1", credential_profile_id: "credential-1", enabled: true, version: 1 };

describe("ModelCatalogPanel", () => {
  it("renders only safe credential metadata and real model associations", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();

    expect(wrapper.text()).toContain("primary-key");
    expect(wrapper.text()).toContain("Safely configured");
    expect(wrapper.text()).toContain("https://models.example.test/v1");
    expect(wrapper.text()).not.toContain("vault://");
    expect(wrapper.text()).toContain("does not verify a model Provider's retention");
  });

  it("hides writes from Team-scoped administrators", async () => {
    const wrapper = mountPanel(apiStub(), [{ role: "platform_administrator", team_id: "team-1" }]);
    await flushPromises();
    expect(wrapper.text()).toContain("Read-only catalog");
    expect(wrapper.find("[data-testid='register-credential']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='toggle-model-model-1']").exists()).toBe(false);
  });

  it("registers a safe reference with a stable key across retry", async () => {
    const api = apiStub();
    vi.mocked(api.registerCredentialProfile).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(credential);
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();
    await wrapper.get("[data-testid='register-credential']").trigger("click");
    await wrapper.get("[data-testid='credential-name']").setValue("primary-key");
    await wrapper.get("[data-testid='credential-secret-ref']").setValue("vault://platform/model-primary");
    await wrapper.get("[data-testid='credential-form']").trigger("submit");
    await flushPromises();
    await wrapper.get("[data-testid='credential-form']").trigger("submit");
    await flushPromises();

    const calls = vi.mocked(api.registerCredentialProfile).mock.calls;
    expect(calls[0]![0]).toMatchObject({ kind: "model", secret_ref: "vault://platform/model-primary" });
    expect(calls[1]![1]).toBe(calls[0]![1]);
  });

  it("reloads dependent models after disabling a Credential Profile", async () => {
    const api = apiStub();
    vi.mocked(api.listConfiguredModels).mockResolvedValueOnce([model]).mockResolvedValueOnce([{ ...model, enabled: false, version: 2 }]);
    vi.mocked(api.changeCredentialProfileStatus).mockResolvedValueOnce({ ...credential, enabled: false, version: 2 });
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();
    await wrapper.get("[data-testid='toggle-credential-credential-1']").trigger("click");
    await flushPromises();

    expect(api.changeCredentialProfileStatus).toHaveBeenCalledWith("credential-1", false, 1, expect.any(String));
    expect(wrapper.get("[data-testid='model-model-1']").text()).toContain("Disabled");
  });

  it("localizes catalog section labels and modal controls", async () => {
    const wrapper = mountPanel(apiStub(), [{ role: "platform_administrator" }], "zh-CN");
    await flushPromises();
    expect(wrapper.text()).toContain("01 / 凭证");
    await wrapper.get("[data-testid='register-credential']").trigger("click");
    expect(wrapper.get("[data-testid='credential-form'] button[aria-label='关闭']").attributes("aria-label")).toBe("关闭");
  });
});

function mountPanel(api: PlatformApi, roleGrants: Array<{ role: string; team_id?: string }>, locale = "en-US") {
  const state = ref<AuthState>({ kind: "authenticated", currentUser: {
    user_id: "user-1", organization: { id: "org-1", slug: "acme", name: "Acme" }, role_grants: roleGrants,
    teams: [{ id: "team-1", slug: "platform", name: "Platform" }],
  } });
  const session: AuthSession = { state, accessToken: () => "token", initialize: vi.fn(), signIn: vi.fn(), signOut: vi.fn(), dispose: vi.fn() };
  return mount(ModelCatalogPanel, { global: { plugins: [createAppI18n({ getItem: () => locale }, locale)], provide: {
    [platformApiKey as symbol]: api, [authContextKey as symbol]: { session, isCallback: false },
  } } });
}

function apiStub(): PlatformApi {
  return {
    listRuntimeImages: vi.fn(async () => ({ items: [], nextPageToken: "" })), getRuntimeImage: vi.fn(), registerRuntimeImage: vi.fn(), changeRuntimeImageStatus: vi.fn(),
    listCredentialProfiles: vi.fn(async () => [credential]), getCredentialProfile: vi.fn(async () => credential), registerCredentialProfile: vi.fn(async () => credential), changeCredentialProfileStatus: vi.fn(async () => credential),
    listConfiguredModels: vi.fn(async () => [model]), getConfiguredModel: vi.fn(async () => model), registerConfiguredModel: vi.fn(async () => model), changeConfiguredModelStatus: vi.fn(async () => model),
    listSourceControlProviders: vi.fn(async () => []), getSourceControlProvider: vi.fn(), registerSourceControlProvider: vi.fn(), changeSourceControlProviderStatus: vi.fn(),
    listRepositoryBindings: vi.fn(async () => []), getRepositoryBinding: vi.fn(), registerRepositoryBinding: vi.fn(), updateRepositoryBinding: vi.fn(), validateRepositoryBinding: vi.fn(),
    listAgents: vi.fn(async () => []), getAgent: vi.fn(), createAgent: vi.fn(), updateAgent: vi.fn(), listAgentDrafts: vi.fn(async () => []), getAgentDraft: vi.fn(), createAgentDraft: vi.fn(), updateAgentDraft: vi.fn(), validateAgentDraft: vi.fn(),
  };
}
