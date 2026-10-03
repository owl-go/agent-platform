// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type PlatformApi, type SmartAssistant } from "../api/client";
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
  it("removes question restrictions and saves legacy sharing without a daily cap", async () => {
    const api = apiStub();
    const saved = structuredClone(assistant);
    saved.share.free_text_enabled = false;
    saved.share.daily_call_limit = 0;
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: saved },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      const dialog = document.body.querySelector(".application-share-dialog") as HTMLElement;
      expect(dialog.textContent).not.toContain("每日自由提问上限");
      expect(dialog.textContent).not.toContain("允许自由提问");
      (dialog.querySelector("[data-testid=share-save]") as HTMLButtonElement).click();
      await flushPromises();
      expect(api.updateSmartAssistant).toHaveBeenCalledWith(saved.id, expect.objectContaining({ share: expect.objectContaining({ free_text_enabled: true, daily_call_limit: 0 }) }), saved.version);
      expect(wrapper.emitted("error")).toBeUndefined();
      expect(saved.share.free_text_enabled).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("saves floating defaults and uploads its independent icon only when saving", async () => {
    const api = apiStub();
    api.uploadSmartAssistantWidgetIcon = vi.fn(async (_id, _file, input) => ({ ...assistant, ...input, share: { ...assistant.share, ...input.share, widget_icon: "uploaded-icon" }, version: 5 }));
    const NativeURL = URL;
    vi.stubGlobal("URL", class extends NativeURL { static createObjectURL() { return "blob:chat-preview"; } static revokeObjectURL() {} });
    const saved = structuredClone(assistant);
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: saved },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      const floating = document.body.querySelector('[data-testid="embed-type"] input[value="floating"]') as HTMLInputElement;
      floating.click(); await flushPromises();
      const open = document.body.querySelector('[data-testid="widget-default-open"] input') as HTMLInputElement;
      open.click(); await flushPromises();
      const file = new File(["png"], "chat.png", { type: "image/png" });
      const input = document.body.querySelector('[data-testid="widget-icon-input"]') as HTMLInputElement;
      Object.defineProperty(input, "files", { value: [file], configurable: true });
      input.dispatchEvent(new Event("change", { bubbles: true })); await flushPromises();
      expect(api.uploadSmartAssistantWidgetIcon).not.toHaveBeenCalled();
      expect(document.body.querySelector('.share-widget-icon img')?.getAttribute("src")).toBe("blob:chat-preview");
      expect(saved.share.embed_type).toBeUndefined();
      (document.body.querySelector('[data-testid="share-save"]') as HTMLButtonElement).click(); await flushPromises();
      expect(api.uploadSmartAssistantWidgetIcon).toHaveBeenCalledWith(saved.id, file, expect.objectContaining({ share: expect.objectContaining({ embed_type: "floating", widget_default_open: true }) }), saved.version);
      expect(api.updateSmartAssistant).not.toHaveBeenCalled();
      expect(saved.icon).toBe("sparkles");
      expect(wrapper.emitted("update:modelValue")).toBeUndefined();
      const updated = wrapper.emitted("updated")![0]![0] as SmartAssistant;
      await wrapper.setProps({ assistant: updated });
      const generate = Array.from(document.body.querySelectorAll(".application-share-dialog button")).find((button) => button.textContent?.trim() === "生成新的分享 Token") as HTMLButtonElement;
      generate.click(); await flushPromises();
      expect(api.regenerateAssistantShareToken).toHaveBeenCalledWith(saved.id, updated.version);
      expect((document.body.querySelector(".share-snippet textarea") as HTMLTextAreaElement).value).toContain("/widget-icon");
    } finally { wrapper.unmount(); vi.unstubAllGlobals(); }
  });

  it("cancels a selected widget icon without uploading or changing saved settings", async () => {
    const api = apiStub(); api.uploadSmartAssistantWidgetIcon = vi.fn();
    const NativeURL = URL;
    vi.stubGlobal("URL", class extends NativeURL { static createObjectURL() { return "blob:preview"; } static revokeObjectURL() {} });
    const saved = structuredClone(assistant); saved.share.embed_type = "floating";
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: saved },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      const input = document.body.querySelector('[data-testid="widget-icon-input"]') as HTMLInputElement;
      Object.defineProperty(input, "files", { value: [new File(["png"], "chat.png", { type: "image/png" })] });
      input.dispatchEvent(new Event("change", { bubbles: true })); await flushPromises();
      const cancel = Array.from(document.body.querySelectorAll(".application-share-dialog button")).find((button) => button.textContent?.trim() === "取消") as HTMLButtonElement;
      cancel.click(); await flushPromises();
      expect(api.uploadSmartAssistantWidgetIcon).not.toHaveBeenCalled();
      expect(saved.share.widget_icon).toBeUndefined();
    } finally { wrapper.unmount(); vi.unstubAllGlobals(); }
  });

  it.each([
    "http://public.example.test", "https://example.test/page", "https://example.test?query=1",
    "https://example.test#section", "https://user:password@example.test", "https://*.example.test", "example.test",
  ])("explains an invalid origin before sending a save: %s", async (origin) => {
    const api = apiStub();
    const draft = structuredClone(assistant);
    draft.share.allowed_origins = [origin];
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: draft },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      (document.body.querySelector(".application-share-dialog [data-testid=share-save]") as HTMLButtonElement).click();
      await flushPromises();
      expect(api.updateSmartAssistant).not.toHaveBeenCalled();
      expect(wrapper.emitted("error")?.[0]?.[0]).toContain("HTTPS");
    } finally { wrapper.unmount(); }
  });

  it("saves an HTTPS site address ending in a slash as an embed origin", async () => {
    const api = apiStub();
    api.updateSmartAssistant = vi.fn(async (_id, input) => {
      // The server's origin contract rejects a URL path, including a root slash.
      if (input.share?.allowed_origins?.some((origin: string) => origin.endsWith("/"))) throw new ApiError("validation", 422, "invalid_input");
      return { ...assistant, ...input, share: { ...assistant.share, ...input.share, token: "new-token" }, version: 5 };
    });
    const draft = structuredClone(assistant);
    draft.share.allowed_origins = ["https://support.example.test/"];
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: draft },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      (document.body.querySelector(".application-share-dialog [data-testid=share-save]") as HTMLButtonElement).click();
      await flushPromises();
      expect(wrapper.emitted("error")).toBeUndefined();
      expect(wrapper.emitted("updated")?.[0]?.[0]).toMatchObject({ share: { allowed_origins: ["https://support.example.test"], token: "new-token" } });
      expect((document.body.querySelector(".share-snippet textarea") as HTMLTextAreaElement).value).toContain("/embed/assistant/new-token");
    } finally { wrapper.unmount(); }
  });

  it("keeps rejected share edits in the dialog without changing the saved assistant", async () => {
    const api = apiStub();
    api.updateSmartAssistant = vi.fn(async () => { throw new ApiError("unknown", 500, "request_failed"); });
    const saved = structuredClone(assistant);
    const wrapper = mount(SmartAssistantShareDialog, {
      attachTo: document.body, props: { modelValue: true, assistant: saved },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } },
    });
    try {
      await flushPromises();
      const origins = document.body.querySelector(".application-share-dialog textarea") as HTMLTextAreaElement;
      origins.value = "https://changed.example.test";
      origins.dispatchEvent(new Event("input", { bubbles: true }));
      (document.body.querySelector(".application-share-dialog [role=switch]") as HTMLButtonElement).click();
      await flushPromises();
      (document.body.querySelector(".application-share-dialog [data-testid=share-save]") as HTMLButtonElement).click();
      await flushPromises();
      expect(saved.share.allowed_origins).toEqual(assistant.share.allowed_origins);
      expect(saved.share.enabled).toBe(true);
      expect(document.body.querySelector(".application-share-dialog [role=alert]")?.textContent).toContain("保存");
      expect(wrapper.emitted("updated")).toBeUndefined();
    } finally { wrapper.unmount(); }
  });

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

  it("requires allowed origins and data acknowledgement before sharing", async () => {
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
