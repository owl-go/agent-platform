// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import ResourceCenterPage from "./ResourceCenterPage.vue";

function api(): PlatformApi {
  return {
    listExperts: vi.fn(async () => []),
    listExpertTeams: vi.fn(async () => []),
    listMCPServers: vi.fn(async () => []),
    listSkills: vi.fn(async () => []),
    listCLIConnectorDefinitions: vi.fn(async () => []),
    listCLIConnectorEnablements: vi.fn(async () => []),
  } as unknown as PlatformApi;
}

describe("ResourceCenterPage", () => {
  it("organizes Experts, Skills, and Connectors into top-level tabs", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/resources");
    await router.isReady();
    const wrapper = mount(ResourceCenterPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api() },
      },
    });
    await flushPromises();

    expect(wrapper.findAll(".resource-center-tabs .el-tabs__item").map((tab) => tab.text())).toEqual(["专家", "技能", "连接器"]);
    expect(wrapper.find(".expert-catalog").exists()).toBe(true);
    expect(wrapper.find(".resource-tabs").exists()).toBe(false);

    await wrapper.findAll(".resource-center-tabs .el-tabs__item")[1]!.trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.query.tab).toBe("skills");
    expect(wrapper.find(".expert-catalog").exists()).toBe(false);
    expect(wrapper.find(".resource-child-actions").exists()).toBe(true);
    expect(wrapper.find(".resource-tabs").exists()).toBe(false);
    wrapper.unmount();
  });
});
