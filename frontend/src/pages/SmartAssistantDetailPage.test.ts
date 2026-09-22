// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantDetailPage from "./SmartAssistantDetailPage.vue";

const assistant: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", description: "帮助用户解决产品问题", introduction: "你好，欢迎咨询产品问题。", scenario: "product-guide", prompt: "回答用户问题", preprocess_prompt: "整理用户问题", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "enabled", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 2,
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
  it("shows the reduced basic form and saves the new assistant fields", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantDetailPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.get(".application-detail-card").text()).toContain("助手简介");
    expect(wrapper.get(".application-detail-card").text()).toContain("提示词");
    expect(wrapper.get(".application-detail-card").text()).toContain("预处理提示词");
    expect(wrapper.get(".application-detail-card").text()).not.toContain("服务目标");
    expect(wrapper.get(".application-detail-card").text()).not.toContain("回答范围");
    expect(wrapper.get(".application-detail-card").text()).not.toContain("回答规则");
    expect(wrapper.findAll(".icon-picker-option")).toHaveLength(1);
    await wrapper.get(".assistant-detail-tabs button:nth-child(2)").trigger("click");
    expect(wrapper.find(".faq-row").exists()).toBe(false);
    expect(wrapper.get(".faq-list-header strong").text()).toBe("常见问题");
    expect(wrapper.get(".faq-list-header .el-button").text()).toBe("添加问题");
    await wrapper.get(".assistant-detail-tabs button:nth-child(1)").trigger("click");
    await wrapper.get(".application-detail-card input").setValue("更新后的助手");
    await wrapper.get(".assistant-detail-actions .el-button--primary").trigger("click");
    await flushPromises();

    expect(api.updateSmartAssistant).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ name: "更新后的助手", description: assistant.description, prompt: assistant.prompt, preprocess_prompt: assistant.preprocess_prompt, digital_human_id: undefined }), 2);
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

  it("enables a complete draft before creating its session", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const draftAssistant = { ...assistant, state: "draft" as const };
    const api = apiStub();
    vi.mocked(api.getSmartAssistant).mockResolvedValue(draftAssistant);
    const wrapper = mount(SmartAssistantDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-detail-actions .el-button").trigger("click");
    await flushPromises();

    expect(api.updateSmartAssistant).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ state: "enabled", prompt: assistant.prompt, preprocess_prompt: assistant.preprocess_prompt }), 2);
    expect(api.createAssistantSession).toHaveBeenCalledWith("assistant-1");
    expect(router.currentRoute.value.path).toBe("/sessions");
    wrapper.unmount();
  });

  it("requires an uploaded icon before starting an assistant", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const draftAssistant = { ...assistant, state: "enabled" as const, icon: "" };
    const api = apiStub();
    vi.mocked(api.getSmartAssistant).mockResolvedValue(draftAssistant);
    const wrapper = mount(SmartAssistantDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-detail-actions .el-button").trigger("click");
    await flushPromises();

    expect(api.updateSmartAssistant).not.toHaveBeenCalled();
    expect(api.createAssistantSession).not.toHaveBeenCalled();
    expect(wrapper.find(".el-alert").text()).toContain("图标");
    wrapper.unmount();
  });
});
