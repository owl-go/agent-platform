// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";

afterEach(() => { document.body.innerHTML = ""; });

describe("SkillsConnectorsPage", () => {
  it.each(["teambition_auth", "connector_auth", "linear_auth"])("completes a browser callback without relying on its original tab (%s)", async (returnKey) => {
    const flowID = "11111111-1111-4111-8111-111111111111";
    const complete = vi.fn(async () => ({ id: flowID, installation_id: "installation", identity: "user", scopes: [], state: "completed" }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), listConnectorPublications: vi.fn(async () => []), listConnectorInstallations: vi.fn(async () => []), completeConnectorAuthorizationFlow: complete } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push(`/resources?tab=connectors&${returnKey}=${flowID}`);
    const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    expect(complete).toHaveBeenCalledWith(flowID);
    expect(router.currentRoute.value.query[returnKey]).toBeUndefined();
    expect(api.listConnectorInstallations).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });

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
    expect(frame.get(".extension-catalog-toolbar").text()).toContain("已安装");
    expect(frame.get(".extension-catalog-toolbar").text()).toContain("新建连接器");
    expect(frame.find(".connector-package-panel").exists()).toBe(false);
    expect(frame.findAll(".published-connector-card")).toHaveLength(1);
    expect(frame.get(".published-connector-card").text()).toContain("飞书");
    expect(frame.find('.published-connector-card button[aria-label="安装"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it.each([["zh-CN", "启用", "操作失败"], ["en", "Enable", "Operation failed"]])("shows failed enablement requests in the catalog (%s)", async (language, enableLabel, errorTitle) => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [], conformance_runtime_digests: ["sha256:" + "a".repeat(64)] };
    const enableCLIConnector = vi.fn().mockRejectedValue(new Error("request_failed"));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), enableCLIConnector } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
    await router.push("/resources?tab=connectors");
    const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => language }, language)], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await wrapper.get(`button[aria-label="${enableLabel}"]`).trigger("click");
    await flushPromises();
    expect(enableCLIConnector).toHaveBeenCalledWith(definition.id);
    expect(document.body.querySelector('[role="alert"]')?.textContent).toContain(errorTitle);
    wrapper.unmount();
  });
});


it("lets the old personal connector URL return to the market", async () => {
  const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), listConnectorPublications: vi.fn(async () => [{ source: "feishu", state: "available", revision: { name: "飞书", conformance_available: true } }]), listConnectorInstallations: vi.fn(async () => []) } as unknown as PlatformApi;
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/resources", component: SkillsConnectorsPage }] });
  await router.push("/resources?tab=connectors&scope=mine");
  const wrapper = mount(SkillsConnectorsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
  await flushPromises();
  expect(wrapper.find(".published-connector-card").exists()).toBe(false);
  const tabs = wrapper.get(".connector-view-tabs").findAll("button");
  expect(tabs[1]!.attributes("aria-pressed")).toBe("true");
  await tabs[0]!.trigger("click");
  expect(wrapper.get(".published-connector-card").text()).toContain("飞书");
  wrapper.unmount();
});
