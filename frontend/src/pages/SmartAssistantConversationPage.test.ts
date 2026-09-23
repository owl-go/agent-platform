// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type AssistantConversation, type PlatformApi, type SmartAssistantFAQ } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import SmartAssistantConversationPage from "./SmartAssistantConversationPage.vue";

const conversation: AssistantConversation = { id: "conversation-1", assistant_id: "assistant-1", assistant_name: "产品助手", welcome: "欢迎提问", created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z" };
const faq = { id: "faq-1", question: "怎么退款？", answer_markdown: "请联系支持", enabled: true } as SmartAssistantFAQ;

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
});
