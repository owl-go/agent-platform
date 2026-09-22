// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantDetailPage from "./SmartAssistantDetailPage.vue";

const assistant: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", introduction: "你好，欢迎咨询产品问题。", scenario: "product-guide", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "enabled", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 2,
};

function apiStub() {
  return {
    getSmartAssistant: vi.fn(async () => ({ ...assistant })),
    listAssistantFAQs: vi.fn(async () => []),
    listApplicationKnowledgeBases: vi.fn(async () => []),
    listDigitalHumans: vi.fn(async () => []),
    updateSmartAssistant: vi.fn(async (_id, input, version) => ({ ...assistant, ...input, version: version + 1 })),
    createAssistantSession: vi.fn(async () => ({ id: "session-1", title: "产品助手", archived: false, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 1 })),
  } as unknown as PlatformApi;
}

describe("SmartAssistantDetailPage", () => {
  it("opens sharing from the card query and saves an explicit assistant payload", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1?share=1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantDetailPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(document.body.querySelector(".application-share-dialog")).not.toBeNull();
    await wrapper.get(".assistant-detail-tabs button:nth-child(2)").trigger("click");
    expect(wrapper.find(".faq-row").exists()).toBe(false);
    await wrapper.get(".assistant-detail-tabs button:nth-child(1)").trigger("click");
    await wrapper.get(".application-detail-card input").setValue("更新后的助手");
    await wrapper.get(".assistant-detail-actions .el-button--primary").trigger("click");
    await flushPromises();

    expect(api.updateSmartAssistant).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ name: "更新后的助手", introduction: assistant.introduction, digital_human_id: undefined }), 2);
    wrapper.unmount();
  });

  it("starts an enabled assistant session and carries the welcome message", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-detail-actions .el-button").trigger("click");
    await flushPromises();

    expect(api.createAssistantSession).toHaveBeenCalledWith("assistant-1");
    expect(router.currentRoute.value.path).toBe("/sessions");
    expect(router.currentRoute.value.query.assistant_welcome).toBe(assistant.introduction);
    wrapper.unmount();
  });
});
