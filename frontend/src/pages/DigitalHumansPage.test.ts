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
    listDigitalHumans: vi.fn(async () => [{ id: "human-1", name: "品牌讲解员", avatar_object_key: "", voice: "female", language: "zh-CN", expression_style: "", scene_description: "", state: "enabled", created_at: "2026-09-22T08:30:00Z", updated_at: "2026-09-22T08:30:00Z", version: 1 }]),
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

  it("renders digital humans as a list with edit, detail, and delete actions", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/ai-apps/digital-humans");
    const wrapper = mount(DigitalHumansPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: apiStub() } } });
    await flushPromises();

    expect(wrapper.find(".digital-human-table").exists()).toBe(true);
    expect(wrapper.find(".application-card-grid").exists()).toBe(false);
    expect(wrapper.text()).toContain("品牌讲解员");
    expect(wrapper.text()).toContain("创建时间");
    expect(wrapper.text()).toContain("编辑");
    expect(wrapper.text()).toContain("详情");
    expect(wrapper.text()).toContain("删除");

    const buttons = wrapper.findAll(".digital-human-actions .el-button");
    expect(buttons.map((button) => button.text())).toEqual(["编辑", "详情", "删除"]);
    await buttons.find((button) => button.text() === "编辑")!.trigger("click");
    await flushPromises();
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.currentRoute.value.fullPath).toBe("/ai-apps/digital-humans/human-1?mode=edit");
    await router.push("/ai-apps/digital-humans");
    await flushPromises();
    await buttons[1]!.trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.fullPath).toBe("/ai-apps/digital-humans/human-1");
    wrapper.unmount();
  });
});
