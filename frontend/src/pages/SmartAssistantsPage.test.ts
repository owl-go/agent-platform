// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantsPage from "./SmartAssistantsPage.vue";

const source: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", description: "帮助用户解决产品问题", introduction: "", scenario: "product-guide", prompt: "回答用户问题", preprocess_prompt: "整理用户问题", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "draft", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-21T00:00:00Z", updated_at: "2026-09-21T00:00:00Z", version: 1,
};

function apiStub(): PlatformApi {
  return {
    listSmartAssistants: vi.fn(async () => [source]),
    setSmartAssistantState: vi.fn(async (_id, state) => ({ ...source, state })),
    updateSmartAssistant: vi.fn(async (_id, input, version) => ({ ...source, ...input, version: version + 1 })),
    deleteSmartAssistant: vi.fn(async () => {}),
    createSmartAssistant: vi.fn(async () => source),
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

  it("filters by name and opens sharing without entering the edit page", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    const wrapper = mount(SmartAssistantsPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-search input").setValue("不存在");
    expect(wrapper.findAll(".application-card")).toHaveLength(0);
    await wrapper.get(".assistant-search input").setValue("产品");
    await wrapper.get(".application-card-actions .el-button:nth-child(2)").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants");
    expect(document.body.querySelector(".application-share-dialog")).not.toBeNull();
    expect(wrapper.get(".application-card p").text()).toBe(source.description);
    await wrapper.get(".application-share-dialog .el-dialog__footer .el-button--primary").trigger("click");
    await flushPromises();
    expect(api.updateSmartAssistant).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ description: source.description }), 1);
  });

  it("keeps the catalog compact and exposes a search action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.find(".application-catalog-toolbar p").exists()).toBe(false);
    expect(wrapper.find(".assistant-search-button").text()).toBe("搜索");
    expect(wrapper.find(".assistant-search").classes()).toContain("assistant-search");
    wrapper.unmount();
  });
});
