// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";

describe("SkillsConnectorsPage", () => {
  it("shows failed enablement requests in the catalog", async () => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [] };
    const enableCLIConnector = vi.fn().mockRejectedValue(new Error("request_failed"));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), enableCLIConnector } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push("/resources?tab=connectors");
    const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
    await flushPromises();
    expect(enableCLIConnector).toHaveBeenCalledWith(definition.id);
    expect(wrapper.get('[role="alert"]').text()).toContain("操作失败");
    wrapper.unmount();
  });
});
