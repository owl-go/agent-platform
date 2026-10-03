// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import type { PublicAssistantApi, PublicAssistantEvent } from "../api/publicAssistant";
import PublicAssistantConversationPage from "./PublicAssistantConversationPage.vue";

function stub(): PublicAssistantApi {
  return { iconURL: "/public/icon", profile: vi.fn(async () => ({ name: "项目分析助手", introduction: "欢迎提问", faqs: [{ id: "faq-1", question: "运行引擎是什么？" }], free_text_enabled: true, width: "640px", height: 600 })), stream: vi.fn() };
}
function mountPage(api: PublicAssistantApi, embedded = false) {
  return mount(PublicAssistantConversationPage, { props: { api, width: "640px", height: 600, embedded }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
}
describe("PublicAssistantConversationPage", () => {
  it("uses the authenticated chat layout, saved dimensions, avatar, FAQ chips, and incremental Markdown replies", async () => {
    const api = stub();
    let emit!: (event: PublicAssistantEvent) => void;
    let finish!: () => void;
    api.stream = vi.fn((_question, _conversation, _faq, onEvent) => {
      emit = onEvent;
      return new Promise<void>((resolve) => { finish = resolve; });
    });
    const wrapper = mountPage(api);
    try {
      await flushPromises();
      expect(wrapper.attributes("style")).toContain("width: 640px; height: 600px");
      expect(wrapper.find(".assistant-conversation-header").exists()).toBe(false);
      expect(wrapper.text()).not.toContain("清空并新建对话");
      expect(wrapper.get(".assistant-conversation-message--welcome").text()).toContain("欢迎提问");
      expect(wrapper.get(".assistant-conversation-avatar img").attributes("src")).toBe(api.iconURL);
      expect(wrapper.get(".assistant-conversation-faq-label").text()).toBe("常见问题");
      await wrapper.get("textarea").setValue("分析这个客户");
      await wrapper.get(".assistant-conversation-send").trigger("click");
      emit({ type: "thinking", turn_id: "turn-1", conversation_id: "visitor-conversation" });
      emit({ type: "delta", text: "**继续跟进**" });
      await flushPromises();
      expect(wrapper.get(".assistant-conversation-turn .markdown-body strong").text()).toBe("继续跟进");
      expect(wrapper.get(".assistant-conversation-message--user").text()).toBe("分析这个客户");
      emit({ type: "delta", text: "，维护关系。" });
      await flushPromises();
      expect(wrapper.get(".markdown-body").text()).toContain("继续跟进，维护关系。");
      emit({ type: "done", answer: "**继续跟进**，维护关系。", state: "completed" });
      finish();
      await flushPromises();
      await wrapper.get(".assistant-conversation-faq").trigger("click");
      expect(api.stream).toHaveBeenLastCalledWith("运行引擎是什么？", "visitor-conversation", "faq-1", expect.any(Function), expect.any(AbortSignal));
    } finally { finish?.(); await flushPromises(); wrapper.unmount(); }
  });

  it("keeps partial output on stop and continues the visitor conversation without a header", async () => {
    const api = stub();
    let emit!: (event: PublicAssistantEvent) => void;
    api.stream = vi.fn((_q, _c, _f, onEvent, signal) => {
      emit = onEvent;
      return new Promise<void>((_resolve, reject) => signal.addEventListener("abort", () => reject(new DOMException("Stopped", "AbortError"))));
    });
    const wrapper = mountPage(api, true);
    await flushPromises();
    expect(wrapper.attributes("style")).toContain("width: 100%; height: 100dvh");
    expect(wrapper.find(".assistant-conversation-header").exists()).toBe(false);
    await wrapper.get("textarea").setValue("分析客户");
    await wrapper.get(".assistant-conversation-send").trigger("click");
    emit({ type: "thinking", turn_id: "turn", conversation_id: "visitor-conversation" });
    emit({ type: "delta", text: "部分回答" });
    await flushPromises();
    await wrapper.get(".assistant-conversation-send").trigger("click");
    await flushPromises();
    expect(wrapper.get(".markdown-body").text()).toBe("部分回答");
    await wrapper.get(".assistant-conversation-faq").trigger("click");
    expect(api.stream).toHaveBeenLastCalledWith("运行引擎是什么？", "visitor-conversation", "faq-1", expect.any(Function), expect.any(AbortSignal));
    wrapper.unmount();
    await flushPromises();
  });

  it("allows free questions and FAQs even when legacy metadata disables free text", async () => {
    const api = stub();
    api.profile = vi.fn(async () => ({ name: "助手", introduction: "欢迎", faqs: [{ id: "faq", question: "是什么？" }], free_text_enabled: false, width: "100%", height: 600 }));
    api.stream = vi.fn(async (_q, _c, _f, emit) => { emit({ type: "done", answer: "固定答案", state: "completed" }); });
    const wrapper = mountPage(api);
    await flushPromises();
    expect(wrapper.get("textarea").attributes("disabled")).toBeUndefined();
    await wrapper.get("textarea").setValue("这个客户的电话");
    await wrapper.get("textarea").trigger("keydown", { key: "Enter" });
    await flushPromises();
    expect(api.stream).toHaveBeenCalledWith("这个客户的电话", "", undefined, expect.any(Function), expect.any(AbortSignal));
    await wrapper.get(".assistant-conversation-faq").trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".markdown-body").at(-1)?.text()).toBe("固定答案");
    wrapper.unmount();
  });
});
