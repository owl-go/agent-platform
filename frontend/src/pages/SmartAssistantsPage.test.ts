// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { ApiError, platformApiKey, type AssistantConversation, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantsPage from "./SmartAssistantsPage.vue";

const source: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", description: "帮助用户解决产品问题", introduction: "", scenario: "product-guide", prompt: "回答用户问题", preprocess_prompt: "整理用户问题", provider_model_id: "model-1", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "draft", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-21T00:00:00Z", updated_at: "2026-09-21T00:00:00Z", version: 1,
};
const previousConversation: AssistantConversation = { id: "previous-conversation", assistant_id: "assistant-1", assistant_name: "产品助手", welcome: "欢迎使用产品助手", created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T01:00:00Z" };

function apiStub(): PlatformApi {
  return {
    listSmartAssistants: vi.fn(async () => [source]),
    listModelProviderConnections: vi.fn(async () => [{ id: "connection-1", name: "模型服务", provider_type: "openai", endpoint: "https://example.test/v1", protocols: ["openai_chat"], api_key_configured: true, models: [{ id: "model-1", model_id: "chat-model", display_name: "聊天模型", available: true }] }]),
    setSmartAssistantState: vi.fn(async (_id, state) => ({ ...source, state })),
    updateSmartAssistant: vi.fn(async (_id, input, version) => ({ ...source, ...input, version: version + 1 })),
    deleteSmartAssistant: vi.fn(async () => {}),
    createSmartAssistant: vi.fn(async () => source),
    uploadSmartAssistantIcon: vi.fn(async (_id, _file, version) => ({ ...source, icon: "ai-applications/assistant-icons/user-1/icon-1", version: version + 1 })),
    listAssistantConversations: vi.fn(async () => []),
    createAssistantConversation: vi.fn(async () => ({ id: "conversation-1", assistant_id: "assistant-1", assistant_name: "产品助手", welcome: "欢迎使用产品助手", created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z" })),
  } as unknown as PlatformApi;
}

describe("SmartAssistantsPage lifecycle", () => {
  it("opens the assistant creation form from the create action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const wrapper = mount(SmartAssistantsPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    await wrapper.get(".application-create-trigger").trigger("click");

    expect(document.body.querySelector(".application-create-dialog")).not.toBeNull();
    wrapper.unmount();
  });

  it("uploads the selected icon after creating the assistant draft", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    const wrapper = mount(SmartAssistantsPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.get(".application-create-trigger").trigger("click");
    const dialog = document.body.querySelector(".application-create-dialog") as HTMLElement;
    wrapper.findAllComponents({ name: "ElSelect" })[1].vm.$emit("update:modelValue", "model-1");
    const name = dialog.querySelector("input[type=text]") as HTMLInputElement;
    name.value = "新助手";
    name.dispatchEvent(new Event("input"));
    const file = new File(["png-content"], "assistant.png", { type: "image/png" });
    const input = dialog.querySelector('[data-testid="icon-picker-file"]') as HTMLInputElement;
    Object.defineProperty(input, "files", { configurable: true, value: [file] });
    input.dispatchEvent(new Event("change"));
    await flushPromises();
    (dialog.querySelector(".el-dialog__footer .el-button--primary") as HTMLButtonElement).click();
    await flushPromises();

    expect(api.createSmartAssistant).toHaveBeenCalledWith(expect.objectContaining({ name: "新助手", icon: "", provider_model_id: "model-1" }));
    expect(api.uploadSmartAssistantIcon).toHaveBeenCalledWith("assistant-1", file, 1);
    wrapper.unmount();
  });

  it("filters by name and opens sharing without entering the edit page", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    const wrapper = mount(SmartAssistantsPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-search input").setValue("不存在");
    expect(wrapper.findAll(".application-card")).toHaveLength(0);
    await wrapper.get(".assistant-search input").setValue("产品");
    await wrapper.get("[data-testid=assistant-more]").trigger("click");
    await flushPromises();
    (document.body.querySelectorAll(".assistant-card-action-menu .el-dropdown-menu__item")[1] as HTMLElement).click();
    await flushPromises();
    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants");
    expect(document.body.querySelector(".application-share-dialog")).not.toBeNull();
    expect(wrapper.get(".application-card p").text()).toBe(source.description);
    await wrapper.get(".application-share-dialog .el-dialog__footer .el-button--primary").trigger("click");
    await flushPromises();
    expect(api.updateSmartAssistant).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ description: source.description, provider_model_id: "model-1" }), 1);
  });

  it("keeps the catalog compact and exposes a search action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.find(".application-catalog-toolbar p").exists()).toBe(false);
    expect(wrapper.find(".assistant-search-button").text()).toBe("搜索");
    expect(wrapper.find(".assistant-search").classes()).toContain("assistant-search");
    expect(wrapper.findAll(".application-card-actions .el-button")).toHaveLength(2);
    expect(wrapper.get("[data-testid=assistant-chat]").attributes("aria-label")).toBe("开始对话");
    expect(wrapper.get("[data-testid=assistant-more]").attributes("aria-label")).toBe("更多");
    wrapper.unmount();
  });

  it("renders the more menu outside the clipping assistant card", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const wrapper = mount(SmartAssistantsPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-more]").trigger("click");
    await flushPromises();

    const menu = document.body.querySelector(".assistant-card-action-menu");
    expect(menu).not.toBeNull();
    expect(wrapper.get(".application-card").element.contains(menu)).toBe(false);
    (menu!.querySelectorAll(".el-dropdown-menu__item")[1] as HTMLElement).click();
    await flushPromises();
    expect(document.body.querySelector(".application-share-dialog")).not.toBeNull();
    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants");
    wrapper.unmount();
  });

  it("starts a conversation from the card chat action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    vi.mocked(api.listSmartAssistants).mockResolvedValue([{ ...source, state: "enabled", introduction: "欢迎使用产品助手" }]);
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-chat]").trigger("click");
    await flushPromises();

    expect(api.createAssistantConversation).toHaveBeenCalledWith("assistant-1");
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe("/ai-apps/assistants/assistant-1/conversations/conversation-1"));
    wrapper.unmount();
  });

  it("reopens the last saved conversation when entering the assistant again", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    vi.mocked(api.listSmartAssistants).mockResolvedValue([{ ...source, state: "enabled" }]);
    api.listAssistantConversations = vi.fn(async () => [previousConversation]);
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-chat]").trigger("click");
    await flushPromises();

    expect(api.listAssistantConversations).toHaveBeenCalledWith("assistant-1");
    expect(api.createAssistantConversation).not.toHaveBeenCalled();
    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants/assistant-1/conversations/previous-conversation");
    wrapper.unmount();
  });

  it("keeps saved conversation history readable after the assistant is disabled", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    vi.mocked(api.listSmartAssistants).mockResolvedValue([{ ...source, state: "disabled" }]);
    api.listAssistantConversations = vi.fn(async () => [previousConversation]);
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-chat]").trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants/assistant-1/conversations/previous-conversation");
    expect(api.createAssistantConversation).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("asks the user to enable a disabled assistant before chatting", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-chat]").trigger("click");
    await flushPromises();

    expect(api.createAssistantConversation).not.toHaveBeenCalled();
    expect(wrapper.get(".el-alert").text()).toContain("请先启用智能助手");
    wrapper.unmount();
  });

  it("explains when the selected model is unavailable", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    vi.mocked(api.listSmartAssistants).mockResolvedValue([{ ...source, state: "enabled" }]);
    vi.mocked(api.createAssistantConversation).mockRejectedValue(new ApiError("validation", 422, "assistant_model_unavailable"));
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get("[data-testid=assistant-chat]").trigger("click");
    await flushPromises();

    expect(wrapper.get(".el-alert").text()).toContain("请为智能助手选择支持 openai_chat 的可用模型");
    wrapper.unmount();
  });
});
