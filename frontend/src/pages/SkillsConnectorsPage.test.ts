// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";

afterEach(() => { document.body.innerHTML = ""; });

describe("SkillsConnectorsPage", () => {
  it("toggles the personal resource filter from the page header", async () => {
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []) } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push("/resources");
    const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.get(".my-resource-toggle").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.query.scope).toBe("mine");
    expect(wrapper.get(".my-resource-toggle").text()).toContain("我的技能");
    expect(wrapper.find(".page-header").exists()).toBe(false);
    expect(wrapper.get(".resource-tabs-items").text()).toContain("技能");
    wrapper.unmount();
  });

  it("presents published and installed connectors in one catalog", async () => {
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => []),
      listCLIConnectorDefinitions: vi.fn(async () => []),
      listCLIConnectorEnablements: vi.fn(async () => []),
      listConnectorPublications: vi.fn(async () => [{ source: "feishu", active_revision_id: "revision-1", state: "available", version: 1, revision: { id: "revision-1", source: "feishu", package_version: "1.0.93", mode: "cli", sha256: "a".repeat(64), name: "飞书", description: "读取和发送飞书消息", icon: "plug", authentication_driver: "feishu", runtime_digests: ["sha256:" + "b".repeat(64)], conformance_available: true, required_scopes: ["im:message"] } }]),
      listConnectorInstallations: vi.fn(async () => []),
    } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push("/resources?tab=connectors");
    const wrapper = mount(SkillsConnectorsPage, { props: { showTabs: false }, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    const frame = wrapper.get(".resource-catalog-frame");
    expect(frame.find(".resource-child-actions").exists()).toBe(false);
    expect(frame.get(".extension-catalog-toolbar").text()).toContain("我的连接器");
    expect(frame.get(".extension-catalog-toolbar").text()).toContain("新建连接器");
    expect(frame.find(".connector-package-panel").exists()).toBe(false);
    expect(frame.findAll(".published-connector-card")).toHaveLength(1);
    expect(frame.get(".published-connector-card").text()).toContain("飞书");
    expect(frame.get(".published-connector-card").text()).toContain("可安装");
    wrapper.unmount();
  });

  it.each([["zh-CN", "启用", "操作失败"], ["en", "Enable", "Operation failed"]])("shows failed enablement requests in the catalog (%s)", async (language, enableLabel, errorTitle) => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [] };
    const enableCLIConnector = vi.fn().mockRejectedValue(new Error("request_failed"));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), enableCLIConnector } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push("/resources?tab=connectors");
    const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => language }, language)], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === enableLabel)!.trigger("click");
    await flushPromises();
    expect(enableCLIConnector).toHaveBeenCalledWith(definition.id);
    expect(document.body.querySelector('[role="alert"]')?.textContent).toContain(errorTitle);
    wrapper.unmount();
  });
});
