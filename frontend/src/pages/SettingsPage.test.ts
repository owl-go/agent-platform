// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type ModelProviderConnection, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
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

async function openConnectionEditor(api: PlatformApi) {
  const wrapper = mount(SettingsPage, {
    global: {
      plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api },
    },
  });
  await flushPromises();
  await wrapper.findAll(".settings-nav button")[1].trigger("click");
  await wrapper.get('.provider-actions button[aria-label="编辑"]').trigger("click");
  return wrapper;
}

describe("SettingsPage model provider feedback", () => {
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
});
