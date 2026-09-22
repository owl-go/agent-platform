// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppRouter } from "../router";
import DigitalHumansPage from "./DigitalHumansPage.vue";

function apiStub(): PlatformApi {
  return {
    listDigitalHumans: vi.fn(async () => []),
    createDigitalHuman: vi.fn(async () => ({ id: "human-1", name: "品牌讲解员", avatar_object_key: "", voice: "", language: "zh-CN", expression_style: "", scene_description: "", state: "enabled", created_at: "2026-09-22T00:00:00Z", updated_at: "2026-09-22T00:00:00Z", version: 1 })),
  } as unknown as PlatformApi;
}

describe("DigitalHumansPage lifecycle", () => {
  it("opens the digital human creation form from the create action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/digital-humans");
    const wrapper = mount(DigitalHumansPage, { attachTo: document.body, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    await wrapper.get(".application-create-trigger").trigger("click");

    expect(document.body.querySelector(".application-create-dialog")).not.toBeNull();
    wrapper.unmount();
  });

  it("does not render the redundant catalog description", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/digital-humans");
    const wrapper = mount(DigitalHumansPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.find(".application-catalog-toolbar p").exists()).toBe(false);
    wrapper.unmount();
  });
});
