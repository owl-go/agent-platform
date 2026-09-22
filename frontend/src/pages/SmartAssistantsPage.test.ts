// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantsPage from "./SmartAssistantsPage.vue";

const source: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", introduction: "", scenario: "product-guide", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "draft", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-21T00:00:00Z", updated_at: "2026-09-21T00:00:00Z", version: 1,
};

function apiStub(): PlatformApi {
  return {
    listSmartAssistants: vi.fn(async () => [source]),
    copySmartAssistant: vi.fn(async () => ({ ...source, id: "assistant-2", name: "产品助手副本" })),
    setSmartAssistantState: vi.fn(async (_id, state) => ({ ...source, state })),
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

  it("filters by name and copies an assistant without opening its detail page", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants");
    const api = apiStub();
    const wrapper = mount(SmartAssistantsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-search input").setValue("不存在");
    expect(wrapper.findAll(".application-card")).toHaveLength(0);
    await wrapper.get(".assistant-search input").setValue("产品");
    await wrapper.get('[aria-label="复制"]') .trigger("click");
    await flushPromises();
    expect(api.copySmartAssistant).toHaveBeenCalledWith("assistant-1");
    expect(router.currentRoute.value.path).toBe("/ai-apps/assistants");
  });
});
