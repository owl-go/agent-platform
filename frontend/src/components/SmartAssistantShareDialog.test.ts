// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
import { createAppI18n } from "../i18n";
import SmartAssistantShareDialog from "./SmartAssistantShareDialog.vue";

const assistant: SmartAssistant = {
  id: "assistant-1", name: "产品助手", icon: "sparkles", description: "", introduction: "欢迎提问", scenario: "product-guide", prompt: "回答问题", preprocess_prompt: "分类", provider_model_id: "model-1", response_style: "简洁", knowledge_base_ids: [], state: "enabled",
  share: { enabled: true, allowed_origins: ["https://support.example.test"], width: "100%", height: 600, free_text_enabled: true, daily_call_limit: 100, data_processing_acknowledged: true },
  created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 4,
};

function apiStub(): PlatformApi {
  return {
    listAssistantFAQs: vi.fn(async () => [{ id: "faq-1", assistant_id: "assistant-1", question: "如何退款？", answer_markdown: "在订单页申请。", display_order: 0, category: "", tag: "", icon: "", enabled: true, created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 1 }]),
    getAssistantPublicationStats: vi.fn(async () => ({ window_days: 30, external_conversations: 2, free_text_calls: 3, faq_answers: 1, model_answers: 2, failed_or_cancelled_answers: 1, safety_refusals: 1, credit_consumed_hundredths: 125 })),
    updateSmartAssistant: vi.fn(async (_id, input) => ({ ...assistant, ...input, share: { ...assistant.share, ...input.share }, version: 5 })),
    regenerateAssistantShareToken: vi.fn(async () => ({ token: "new-token", assistant: { ...assistant, version: 5 } })),
  } as unknown as PlatformApi;
}

describe("SmartAssistantShareDialog", () => {
  it("shows a history-free visitor preview and privacy-bounded aggregate statistics", async () => {
    const api = apiStub();
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body,
      props: { modelValue: true, assistant: structuredClone(assistant) },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    await flushPromises();
    const dialog = document.body.querySelector(".application-share-dialog") as HTMLElement;
    const previewButton = Array.from(dialog.querySelectorAll("button")).find((item) => item.textContent?.includes("以访客身份预览")) as HTMLButtonElement;
    previewButton.click();
    await flushPromises();

    expect(dialog.querySelector("[data-testid=share-visitor-preview]")?.textContent).toContain("如何退款？");
    expect(dialog.querySelector("[data-testid=share-visitor-preview]")?.textContent).toContain("不调用模型、不创建对话");
    expect(dialog.querySelector("[data-testid=share-aggregate-stats]")?.textContent).toContain("消耗积分 1.25");
    expect(dialog.textContent).toContain("不展示访客问题正文");
    wrapper.unmount();
  });

  it("refuses unrestricted sharing before calling the API", async () => {
    const api = apiStub();
    const unsafe = structuredClone(assistant);
    unsafe.share.allowed_origins = [];
    unsafe.share.daily_call_limit = 0;
    unsafe.share.data_processing_acknowledged = false;
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body,
      props: { modelValue: true, assistant: unsafe },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    await flushPromises();
    const dialog = document.body.querySelector(".application-share-dialog") as HTMLElement;
    (dialog.querySelector("[data-testid=share-save]") as HTMLButtonElement).click();
    await flushPromises();

    expect(api.updateSmartAssistant).not.toHaveBeenCalled();
    expect(wrapper.emitted("error")?.[0]?.[0]).toContain("必须填写允许来源");
    wrapper.unmount();
  });
});
