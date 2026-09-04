// @vitest-environment jsdom
import { ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type ModelProviderConnection, type PersonalSettings, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { authContextKey, type AuthContext } from "../auth/session";
import SettingsPage from "./SettingsPage.vue";

const connection: ModelProviderConnection = {
  id: "connection-1",
  name: "OpenAI",
  provider_type: "openai",
  endpoint: "https://api.openai.com/v1",
  protocols: ["openai_responses", "openai_chat"],
  api_key_configured: true,
  verification_status: "unverified",
  custom_endpoint: false,
  models: [],
  created_at: "2026-08-28T00:00:00Z",
  updated_at: "2026-08-28T00:00:00Z",
  version: 1,
};

function apiStub(): PlatformApi {
  return {
    getSettings: vi.fn(async () => ({ personality: "direct_efficient", personality_instructions: "", runtime_model_defaults: [], default_runtime_engine: "codex", language: "zh-CN", timezone: "Asia/Shanghai", version: 1 })),
    listModelProviderConnections: vi.fn(async () => [connection]),
    listModelProviderPresets: vi.fn(async () => [{ provider_type: "openai", display_name: "OpenAI", official_endpoint: "https://api.openai.com/v1", protocols: ["openai_responses", "openai_chat"] }]),
    listMCPServers: vi.fn(async () => []),
    listSkills: vi.fn(async () => []),
    listRuntimeEngines: vi.fn(async () => [{ name: "codex", available: true, native_resume: true, cli_version: "1.0.0" }]),
    updateModelProviderConnection: vi.fn(async () => ({ ...connection, endpoint: "http://model-gateway.internal/openai", version: 2 })),
  } as unknown as PlatformApi;
}

function authContext(administrator: boolean): AuthContext {
  return {
    isCallback: false,
    session: {
      state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "user@example.test", display_name: "User", administrator, settings_ready: true } }),
      accessToken: () => "token",
      initialize: vi.fn(async () => {}),
      signIn: vi.fn(async () => {}),
      signOut: vi.fn(async () => {}),
      dispose: vi.fn(),
    },
  };
}

async function openConnectionEditor(api: PlatformApi) {
  const wrapper = mount(SettingsPage, {
    global: {
      plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: authContext(true) },
    },
  });
  await flushPromises();
  await wrapper.findAll(".settings-nav button")[1].trigger("click");
  await wrapper.get('.provider-actions button[aria-label="编辑"]').trigger("click");
  return wrapper;
}

describe("SettingsPage model provider feedback", () => {
  it("updates the personality instructions when a preset is selected", async () => {
    const api = apiStub();
    api.getSettings = vi.fn(async (): Promise<PersonalSettings> => ({
      personality: "custom",
      personality_instructions: "My custom guidance",
      runtime_model_defaults: [],
      default_runtime_engine: "codex",
      language: "zh-CN",
      timezone: "Asia/Shanghai",
      version: 1,
    }));
    const wrapper = mount(SettingsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: authContext(true) },
      },
    });
    await flushPromises();

    const textarea = wrapper.get<HTMLTextAreaElement>('.block-label textarea');
    await wrapper.find<HTMLInputElement>('input[value="gentle_professional"]').setValue();
    expect(textarea.element.value).toContain("温和");
    await wrapper.find<HTMLInputElement>('input[value="direct_efficient"]').setValue();
    expect(textarea.element.value).toContain("直接");
    await wrapper.find<HTMLInputElement>('input[value="lively_friendly"]').setValue();
    expect(textarea.element.value).toContain("活泼");
    await wrapper.find<HTMLInputElement>('input[value="custom"]').setValue();
    expect(textarea.element.value).toBe("My custom guidance");
    wrapper.unmount();
  });

  it("does not repeat navigation labels or the Settings summary", async () => {
    const wrapper = mount(SettingsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: apiStub(), [authContextKey as symbol]: authContext(true) },
      },
    });
    await flushPromises();

    expect(wrapper.find(".page-header .eyebrow").exists()).toBe(false);
    expect(wrapper.findAll(".settings-nav button").map((button) => button.text())).toEqual(["个性", "模型设置", "扩展"]);
    expect(wrapper.get(".page-header").text()).not.toContain("定义你的默认个性");
    expect(wrapper.find(".settings-canvas .section-heading h2").exists()).toBe(false);
    await wrapper.findAll(".settings-nav button")[1]!.trigger("click");
    expect(wrapper.get(".settings-canvas .section-heading h2").text()).toBe("模型供应商");
    await wrapper.findAll(".settings-nav button")[2]!.trigger("click");
    await flushPromises();
    expect(wrapper.find(".settings-canvas .section-heading h2").exists()).toBe(false);
    await wrapper.findAll(".extension-manager .subtabs button")[2]!.trigger("click");
    expect(wrapper.find(".coming-soon h3").exists()).toBe(false);
    wrapper.unmount();
  });

  it("accepts an HTTP Endpoint and confirms that the connection was saved", async () => {
    const api = apiStub();
    const wrapper = await openConnectionEditor(api);
    await wrapper.get<HTMLInputElement>('.modal-card input[type="url"]').setValue("http://model-gateway.internal/openai");
    await wrapper.get(".modal-card").trigger("submit");
    await flushPromises();

    expect(api.updateModelProviderConnection).toHaveBeenCalled();
    expect(wrapper.find(".modal-layer").exists()).toBe(false);
    expect(wrapper.get(".app-toast.success").text()).toContain("模型供应商已保存");
    wrapper.unmount();
  });

  it("keeps a failed save open and shows the error inside the editor", async () => {
    const api = apiStub();
    api.updateModelProviderConnection = vi.fn(async () => { throw new Error("request rejected"); });
    const wrapper = await openConnectionEditor(api);
    await wrapper.get(".modal-card").trigger("submit");
    await flushPromises();

    expect(wrapper.find(".modal-layer").exists()).toBe(true);
    expect(wrapper.get('.app-toast.error[role="alert"]').text()).toContain("保存失败");
    wrapper.unmount();
  });

  it("classifies a rejected provider update as a validation error", async () => {
    const api = apiStub();
    api.updateModelProviderConnection = vi.fn(async () => { throw new ApiError("validation", 400, "invalid_request_body"); });
    const wrapper = await openConnectionEditor(api);
    await wrapper.get(".modal-card").trigger("submit");
    await flushPromises();

    expect(wrapper.get('.app-toast.error[role="alert"]').text()).toContain("请求参数无效");
    expect(wrapper.get('.app-toast.error[role="alert"]').text()).not.toContain("API Key");
    wrapper.unmount();
  });

  it("adds a model with one model field", async () => {
    const api = apiStub();
    api.createProviderModel = vi.fn(async (_connectionID, input) => ({ id: "model-1", connection_id: connection.id, model_id: input.model_id, display_name: input.model_id, available: true, manually_added: true, compatibility: [] }));
    const wrapper = await openConnectionEditor(api);
    await wrapper.get('.modal-actions button[type="button"]').trigger("click");
    await wrapper.get(".provider-actions .button:nth-child(2)").trigger("click");

    expect(wrapper.get(".modal-card").text()).toContain("模型");
    expect(wrapper.findAll(".modal-card input")).toHaveLength(1);
    await wrapper.get<HTMLInputElement>(".modal-card input").setValue("gpt-5.6-sol");
    await wrapper.get(".modal-card").trigger("submit");
    await flushPromises();

    expect(api.createProviderModel).toHaveBeenCalledWith(connection.id, { model_id: "gpt-5.6-sol" });
    expect(wrapper.find(".modal-card select").exists()).toBe(false);
    wrapper.unmount();
  });

  it("hides global model management from ordinary Users while keeping global models selectable", async () => {
    const api = apiStub();
    api.listModelProviderConnections = vi.fn(async () => [{ ...connection, models: [{ id: "model-1", connection_id: connection.id, model_id: "gpt-5.6-sol", display_name: "GPT 5.6", available: true, manually_added: false, compatibility: [] }] }]);
    const wrapper = mount(SettingsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: authContext(false) },
      },
    });
    await flushPromises();

    expect(wrapper.findAll(".settings-nav button").map((button) => button.text())).toEqual(["个性", "扩展"]);
    expect(wrapper.find('.runtime-defaults option[value="model-1"]').exists()).toBe(true);
    expect(wrapper.find(".provider-actions").exists()).toBe(false);
    wrapper.unmount();
  });
});
