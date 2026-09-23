// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import * as XLSX from "xlsx";
import { createAppI18n } from "../i18n";
import { ApiError, platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppRouter } from "../router";
import SmartAssistantDetailPage from "./SmartAssistantDetailPage.vue";

const assistant: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", description: "帮助用户解决产品问题", introduction: "你好，欢迎咨询产品问题。", scenario: "product-guide", prompt: "回答用户问题", preprocess_prompt: "整理用户问题", service_goal: "回答产品问题", answer_scope: "", operating_rules: "", response_style: "", knowledge_base_ids: [], state: "enabled", share: { enabled: false, width: "100%", height: 600 }, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 2,
};

const existingFAQ = { id: "faq-1", assistant_id: "assistant-1", question: "已有问题", answer_markdown: "旧答案", display_order: 0, category: "", tag: "", icon: "", enabled: true, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 2 };

function apiStub() {
  return {
    getSmartAssistant: vi.fn(async () => ({ ...assistant })),
    listAssistantFAQs: vi.fn(async () => [existingFAQ]),
    listApplicationKnowledgeBases: vi.fn(async () => []),
    listDigitalHumans: vi.fn(async () => []),
    uploadSmartAssistantIcon: vi.fn(async (_id, _file, version) => ({ ...assistant, icon: "ai-applications/assistant-icons/user-1/icon-1", version: version + 1 })),
    updateSmartAssistant: vi.fn(async (_id, input, version) => ({ ...assistant, ...input, version: version + 1 })),
    createAssistantFAQ: vi.fn(async (_id, input) => ({ ...existingFAQ, ...input, id: "faq-2", question: input.question, answer_markdown: input.answer_markdown, version: 1 })),
    updateAssistantFAQ: vi.fn(async (_id, faqID, input, version) => ({ ...existingFAQ, ...input, id: faqID, version: version + 1 })),
    deleteAssistantFAQ: vi.fn(async () => undefined),
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
    expect(wrapper.find(".icon-picker-current").exists()).toBe(false);
    expect(wrapper.findAll(".icon-picker-option")).toHaveLength(1);
    await wrapper.findAll("textarea")[1].setValue("尚未保存的简介");
    const iconInput = wrapper.get('[data-testid="icon-picker-file"]');
    const iconFile = new File(["png-content"], "assistant.png", { type: "image/png" });
    Object.defineProperty(iconInput.element, "files", { configurable: true, value: [iconFile] });
    await iconInput.trigger("change");
    await flushPromises();
    expect(api.uploadSmartAssistantIcon).toHaveBeenCalledWith("assistant-1", iconFile, 2);
    expect(api.updateSmartAssistant).not.toHaveBeenCalled();
    expect((wrapper.findAll("textarea")[1].element as HTMLTextAreaElement).value).toBe("尚未保存的简介");
    await wrapper.get(".assistant-detail-tabs button:nth-child(2)").trigger("click");
    expect(wrapper.find(".faq-table").exists()).toBe(true);
    expect(wrapper.find(".faq-create-form").exists()).toBe(false);
    expect(wrapper.get(".faq-table").text()).toContain("已有问题");
    expect(wrapper.get(".faq-list-header strong").text()).toBe("常见问题");
    expect(wrapper.get("[data-testid=faq-add]").text()).toBe("添加问题");
    await wrapper.get("[data-testid=faq-add]").trigger("click");
    expect(wrapper.find("[data-testid=faq-dialog]").exists()).toBe(true);
    await wrapper.get("[data-testid=faq-dialog-question] input").setValue("新问题");
    await wrapper.get("[data-testid=faq-dialog-answer] textarea").setValue("新答案");
    await wrapper.get("[data-testid=faq-dialog-submit]").trigger("click");
    await flushPromises();
    expect(api.createAssistantFAQ).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ question: "新问题", answer_markdown: "新答案" }));
    expect(wrapper.find(".faq-create-form").exists()).toBe(false);
    await wrapper.get(".assistant-detail-tabs button:nth-child(1)").trigger("click");
    await wrapper.get(".application-detail-card input").setValue("更新后的助手");
    await wrapper.get(".assistant-detail-actions .el-button--primary").trigger("click");
    await flushPromises();

    expect(api.updateSmartAssistant).toHaveBeenLastCalledWith("assistant-1", expect.objectContaining({ name: "更新后的助手", description: "尚未保存的简介", prompt: assistant.prompt, preprocess_prompt: assistant.preprocess_prompt, digital_human_id: undefined }), 3);
    wrapper.unmount();
  });

  it("imports and exports only question and answer columns, overwriting duplicate questions", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.get(".assistant-detail-tabs button:nth-child(2)").trigger("click");

    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([["问题", "答案"], ["已有问题", "覆盖后的答案"], ["新导入问题", "新导入答案"]]), "FAQ");
    const workbookBytes = XLSX.write(workbook, { type: "array", bookType: "xlsx" });
    const file = new File([workbookBytes], "faq.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
    Object.defineProperty(file, "arrayBuffer", { configurable: true, value: async () => workbookBytes });
    const input = wrapper.get('[data-testid="faq-import-file"]');
    Object.defineProperty(input.element, "files", { configurable: true, value: [file] });
    await input.trigger("change");
    await flushPromises();
    expect(api.updateAssistantFAQ).toHaveBeenCalledWith("assistant-1", "faq-1", expect.objectContaining({ question: "已有问题", answer_markdown: "覆盖后的答案" }), 2);
    expect(api.createAssistantFAQ).toHaveBeenCalledWith("assistant-1", expect.objectContaining({ question: "新导入问题", answer_markdown: "新导入答案" }));

    vi.stubGlobal("URL", { ...URL, createObjectURL: vi.fn(() => "blob:faq"), revokeObjectURL: vi.fn() });
    await wrapper.get('[data-testid="faq-export"]').trigger("click");
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    vi.unstubAllGlobals();
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

  it("does not report a session creation failure as an unsaved or disabled assistant", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1");
    const api = apiStub();
    vi.mocked(api.createAssistantSession).mockRejectedValue(new ApiError("unavailable", 500, "assistant_session_create_failed"));
    const wrapper = mount(SmartAssistantDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-detail-actions .el-button").trigger("click");
    await flushPromises();

    expect(wrapper.get(".el-alert").text()).not.toContain("请先保存并启用智能助手");
    expect(wrapper.get(".el-alert").text()).toContain("创建对话失败");
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
