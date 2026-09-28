// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type AssistantConversation, type AssistantTurn, type PlatformApi, type SmartAssistantFAQ } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import SmartAssistantConversationPage from "./SmartAssistantConversationPage.vue";

const conversation: AssistantConversation = { id: "conversation-1", assistant_id: "assistant-1", assistant_name: "产品助手", welcome: "欢迎提问", created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z" };
const faq = { id: "faq-1", question: "怎么退款？", answer_markdown: "请联系支持", enabled: true } as SmartAssistantFAQ;
const completedTurn: AssistantTurn = { id: "turn-1", conversation_id: conversation.id, turn_number: 1, question: faq.question, answer: faq.answer_markdown, source: "faq", state: "completed", input_tokens: 0, output_tokens: 0, created_at: conversation.created_at, updated_at: conversation.updated_at };

function apiStub(): PlatformApi {
  return {
    getAssistantConversation: vi.fn(async () => ({ conversation, turns: [], faqs: [faq] })),
    listAssistantConversations: vi.fn(async () => [conversation]),
    createAssistantConversation: vi.fn(async () => ({ ...conversation, id: "conversation-2" })),
    streamAssistantTurn: vi.fn(async (_assistant, _conversation, _question, _faq, onEvent) => {
      onEvent({ type: "thinking", turn_id: "turn-1", message: "思考中..." });
      onEvent({ type: "delta", turn_id: "turn-1", text: "请联系支持" });
      onEvent({ type: "done", turn: { id: "turn-1", conversation_id: conversation.id, turn_number: 1, question: faq.question, answer: faq.answer_markdown, source: "faq", state: "completed", input_tokens: 0, output_tokens: 0, created_at: conversation.created_at, updated_at: conversation.updated_at } });
    }),
    cancelAssistantTurn: vi.fn(async () => {}),
  } as unknown as PlatformApi;
}

describe("SmartAssistantConversationPage", () => {
  it("renders the uploaded avatar beside the assistant welcome", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const originalCreateObjectURL = URL.createObjectURL;
    const originalRevokeObjectURL = URL.revokeObjectURL;
    URL.createObjectURL = vi.fn(() => "blob:assistant-avatar");
    URL.revokeObjectURL = vi.fn();
    const api = apiStub();
    api.getAssistantConversation = vi.fn(async () => ({ conversation, turns: [completedTurn], faqs: [faq] }));
    api.getSmartAssistantIcon = vi.fn(async () => new Blob(["avatar"], { type: "image/png" }));
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    try {
      await flushPromises();
      expect(api.getSmartAssistantIcon).toHaveBeenCalledWith("assistant-1", expect.any(AbortSignal));
      expect(wrapper.get(".assistant-conversation-message--welcome .assistant-conversation-avatar img").attributes("src")).toBe("blob:assistant-avatar");
      expect(wrapper.get(".assistant-conversation-turn .assistant-conversation-message--user").text()).toContain("怎么退款？");
      expect(wrapper.get(".assistant-conversation-turn .assistant-conversation-message--assistant .assistant-conversation-avatar img").attributes("src")).toBe("blob:assistant-avatar");
    } finally {
      wrapper.unmount();
      expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:assistant-avatar");
      URL.createObjectURL = originalCreateObjectURL;
      URL.revokeObjectURL = originalRevokeObjectURL;
    }
  });

  it("shows the welcome as the first assistant message in the conversation thread", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();
    expect(wrapper.get(".assistant-conversation-message--welcome").text()).toContain("欢迎提问");
    expect(wrapper.find(".assistant-conversation-welcome").exists()).toBe(false);
    wrapper.unmount();
  });

  it("distinguishes a failed reply from normal messages and preserves partial output", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const api = apiStub();
    api.getAssistantConversation = vi.fn(async () => ({ conversation, turns: [{ ...completedTurn, id: "failed-turn", state: "failed" as const, answer: "部分答案" }, completedTurn], faqs: [faq] }));
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    const failed = wrapper.get(".assistant-conversation-message--failed");
    expect(failed.get("[role='alert']").text()).toContain("回答失败");
    expect(failed.get(".markdown-body").text()).toContain("部分答案");
    expect(wrapper.findAll(".assistant-conversation-turn")[1]?.find(".assistant-conversation-message--failed").exists()).toBe(false);
    wrapper.unmount();
  });

  it("explains when the selected model credential is no longer accepted", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const api = apiStub();
    api.getAssistantConversation = vi.fn(async () => ({ conversation, turns: [{ ...completedTurn, id: "failed-auth-turn", state: "failed" as const, answer: "", failure_code: "model_authentication" }], faqs: [faq] }));
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.get(".assistant-conversation-message--failed [role='alert']").text()).toContain("模型服务凭证已失效");
    wrapper.unmount();
  });

  it("keeps the text input and send action together in the chat composer", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.get(".assistant-conversation-composer-shell textarea").attributes("placeholder")).toContain("输入问题");
    expect(wrapper.get(".assistant-conversation-composer-actions .assistant-conversation-send").text()).toBe("发送");
    wrapper.unmount();
  });

  it("shows the welcome and sends an FAQ selection through the dedicated conversation API", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.text()).toContain("欢迎提问");
    await wrapper.get(".assistant-conversation-faqs button").trigger("click");
    await flushPromises();

    expect(api.streamAssistantTurn).toHaveBeenCalledWith("assistant-1", "conversation-1", "怎么退款？", "faq-1", expect.any(Function), expect.any(AbortSignal));
    wrapper.unmount();
  });

  it("labels FAQ shortcuts and uses a compact question control", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.get(".assistant-conversation-faq-label").text()).toBe("常见问题");
    expect(wrapper.get(".assistant-conversation-faqs .assistant-conversation-faq").text()).toBe("怎么退款？");
    wrapper.unmount();
  });

  it("creates a new conversation without deleting the old audit history", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.get(".assistant-conversation-actions .el-button").trigger("click");
    await flushPromises();

    expect(api.createAssistantConversation).toHaveBeenCalledWith("assistant-1");
    expect(router.currentRoute.value.params.conversationId).toBe("conversation-2");
    wrapper.unmount();
  });

  it("shows history navigation even when only one saved conversation exists", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/assistants/assistant-1/conversations/conversation-1");
    const api = apiStub();
    const wrapper = mount(SmartAssistantConversationPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(api.listAssistantConversations).toHaveBeenCalledWith("assistant-1");
    expect(wrapper.get(".assistant-conversation-history-label").text()).toBe("历史对话");
    expect(wrapper.find(".assistant-conversation-history").exists()).toBe(true);
    wrapper.unmount();
  });
});
