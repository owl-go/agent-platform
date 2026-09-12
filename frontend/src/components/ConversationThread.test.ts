// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import type { ConversationMessage } from "../conversationThread";
import ConversationThread from "./ConversationThread.vue";

const stage = { expert_id: "expert-1", expert_name: "架构专家", provider_model_name: "Model", runtime_engine: "codex" as const, position: 1, total: 1, state: "succeeded" as const, elapsed_ms: 1200, final_text: "答案" };

function mountThread(messages: ConversationMessage[]) {
  return mount(ConversationThread, {
    props: { messages, loadAttachment: vi.fn(async () => new Blob()) },
    global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
  });
}

afterEach(() => vi.restoreAllMocks());

describe("ConversationThread", () => {
  it("renders Session and Workflow-shaped messages through the same transcript", () => {
    const wrapper = mountThread([
      { id: "user-1", role: "user", content: "用户原文", state: "succeeded", timestamp: "2026-08-25T12:00:00Z" },
      { id: "assistant-1", role: "assistant", content: "答案", state: "succeeded", timestamp: "2026-08-25T12:00:01Z", stages: [stage] },
    ]);

    expect(wrapper.findAll(".message")).toHaveLength(2);
    expect(wrapper.get(".message.user p").text()).toBe("用户原文");
    expect(wrapper.get(".message.assistant .markdown-body").text()).toBe("答案");
    expect(wrapper.find(".expert-stage-list").exists()).toBe(false);
    wrapper.unmount();
  });

  it("keeps a non-duplicate stage expandable and copies its text", async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const wrapper = mountThread([{ id: "assistant-1", role: "assistant", content: "最终答案", state: "succeeded", timestamp: "2026-08-25T12:00:01Z", stages: [stage] }]);

    expect(wrapper.find(".expert-stage-list").exists()).toBe(true);
    await wrapper.get(".stage-copy").trigger("click");
    expect(writeText).toHaveBeenCalledWith("答案");
    expect(wrapper.get(".stage-copy").text()).toBe("已复制");
    wrapper.unmount();
  });
});
