// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import AssistantEmbedPreviewPage from "./AssistantEmbedPreviewPage.vue";

function preview() {
  return mount(AssistantEmbedPreviewPage, { global: { plugins: [createAppI18n({ getItem: () => null }, "zh-CN")] } });
}
describe("Assistant embed preview", () => {
  it("runs pasted scripts only in the preview document after an explicit run", async () => {
    const wrapper = preview();
    try {
      const code = '<script>window.chatPreview="中文";</script><iframe title="聊天" src="https://example.test/chat"></iframe>';
      await wrapper.get("textarea").setValue(code);
      expect(wrapper.find("iframe").exists()).toBe(false);
      await wrapper.get("[data-testid=run-preview]").trigger("click");
      await flushPromises();
      const frame = wrapper.get("[data-testid=embed-result]");
      expect(frame.attributes("srcdoc")).toContain(code);
      expect(wrapper.find("script").exists()).toBe(false);
      await wrapper.get("textarea").setValue("changed but not run");
      expect(frame.attributes("srcdoc")).toContain(code);
      const clear = wrapper.findAll("button").find((button) => button.text() === "清空")!;
      await clear.trigger("click");
      expect(wrapper.find("iframe").exists()).toBe(false);
      expect((wrapper.get("textarea").element as HTMLTextAreaElement).value).toBe("");
    } finally { wrapper.unmount(); }
  });
  it("explains empty input and the host origin in both locales", async () => {
    const wrapper = preview();
    try {
      await wrapper.get("[data-testid=run-preview]").trigger("click");
      expect(wrapper.text()).toContain("请先粘贴嵌入代码。");
      expect(wrapper.text()).toContain(window.location.origin);
      expect(wrapper.find("iframe").exists()).toBe(false);
    } finally { wrapper.unmount(); }
    const english = mount(AssistantEmbedPreviewPage, { global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")] } });
    try { expect(english.text()).toContain("Run preview"); expect(english.text()).not.toContain("embedPreview."); }
    finally { english.unmount(); }
  });
});
