// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import ExpertsPage from "./ExpertsPage.vue";

describe("ExpertsPage", () => {
  it("renders Experts whose optional binding arrays are omitted by JSON", async () => {
    const expert = {
      id: "expert-1",
      name: "代码专家",
      description: "分析代码",
      created_at: "2026-08-29T00:00:00Z",
      updated_at: "2026-08-29T00:00:00Z",
      version: 1,
    } as Expert;
    const api = {
      listExperts: vi.fn(async () => [expert]),
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => []),
    } as unknown as PlatformApi;
    const wrapper = mount(ExpertsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });

    await flushPromises();

    expect(wrapper.get(".expert-card h2").text()).toBe("代码专家");
    expect(wrapper.get(".tag-row").text()).toContain("未绑定扩展");
    wrapper.unmount();
  });
});
