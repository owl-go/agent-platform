// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type DigitalHuman, type PlatformApi } from "../api/client";
import { createAppRouter } from "../router";
import DigitalHumanDetailPage from "./DigitalHumanDetailPage.vue";

const human: DigitalHuman = {
  id: "human-1",
  name: "品牌讲解员",
  avatar_object_key: "data:image/png;base64,avatar",
  voice: "female",
  language: "zh-CN",
  expression_style: "friendly",
  scene_description: "站在产品展厅中",
  state: "enabled",
  created_at: "2026-09-22T00:00:00Z",
  updated_at: "2026-09-22T00:00:00Z",
  version: 2,
};

function apiStub(): PlatformApi {
  return {
    getDigitalHuman: vi.fn(async () => ({ ...human })),
    updateDigitalHuman: vi.fn(async (_id, input, version) => ({ ...human, ...input, version: version + 1 })),
    setDigitalHumanState: vi.fn(async (_id, state, version) => ({ ...human, state, version: version + 1 })),
    previewDigitalHuman: vi.fn(async () => ({ ...human, preview_text: "品牌讲解员 · zh-CN · female" })),
  } as unknown as PlatformApi;
}

describe("DigitalHumanDetailPage", () => {
  it("renders translated fields, an avatar upload, and no redundant detail title", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/digital-humans/human-1?mode=edit");
    const wrapper = mount(DigitalHumanDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    const card = wrapper.get(".application-detail-card");
    expect(card.text()).toContain("头像");
    expect(card.text()).toContain("声音");
    expect(card.text()).toContain("语言");
    expect(card.text()).toContain("表情风格");
    expect(card.text()).toContain("场景描述");
    expect(card.text()).not.toContain("aiApplications.digitalHuman.");
    expect(wrapper.find('[data-testid="digital-human-avatar-file"]').exists()).toBe(true);
    expect(wrapper.find(".application-detail-header .eyebrow").exists()).toBe(false);
    expect(wrapper.find(".application-detail-header h1").exists()).toBe(false);
    wrapper.unmount();
  });

  it("saves all presentation settings and supports deterministic preview", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/digital-humans/human-1?mode=edit");
    const api = apiStub();
    const wrapper = mount(DigitalHumanDetailPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    const avatarInput = wrapper.get('[data-testid="digital-human-avatar-file"]');
    const file = new File(["avatar"], "avatar.png", { type: "image/png" });
    Object.defineProperty(avatarInput.element, "files", { configurable: true, value: [file] });
    await avatarInput.trigger("change");
    await new Promise((resolve) => setTimeout(resolve, 0));
    await wrapper.get(".assistant-detail-actions .el-button--primary").trigger("click");
    await flushPromises();
    expect(api.updateDigitalHuman).toHaveBeenCalledWith("human-1", expect.objectContaining({ avatar_object_key: expect.stringMatching(/^data:image\/png;base64,/), voice: human.voice, language: human.language, expression_style: human.expression_style, scene_description: human.scene_description }), 2);

    await wrapper.get('[data-testid="digital-human-preview"]').trigger("click");
    await flushPromises();
    expect(api.previewDigitalHuman).toHaveBeenCalledWith("human-1");
    expect(wrapper.text()).toContain("品牌讲解员 · zh-CN · female");
    wrapper.unmount();
  });
});
