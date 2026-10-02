// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import ExpertsPage from "./ExpertsPage.vue";
import ResourceCenterPage from "./ResourceCenterPage.vue";

function api(): PlatformApi {
  return {
    listExperts: vi.fn(async () => []),
    listExpertTeams: vi.fn(async () => []),
    listMCPServers: vi.fn(async () => []),
    listSkills: vi.fn(async () => []),
    listCLIConnectorDefinitions: vi.fn(async () => []),
    listCLIConnectorEnablements: vi.fn(async () => []),
    listConnectorPublications: vi.fn(async () => []),
    listConnectorInstallations: vi.fn(async () => []),
    listKnowledgeCategories: vi.fn(async () => []),
    listKnowledgeDocuments: vi.fn(async () => []),
    listKnowledgeBases: vi.fn(async () => [{ id: "knowledge-1", owner_id: "user-1", name: "交付手册", description: "部署和验收", visibility: "private", platform: false, deleted: false, created_at: "2026-09-28T00:00:00Z", updated_at: "2026-09-28T00:00:00Z", version: 1, document_count: 2, ready_document_count: 1, last_ready_at: "2026-09-28T00:00:00Z" }]),
  } as unknown as PlatformApi;
}

const auth: AuthContext = { isCallback: false, session: { state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "user@example.test", display_name: "User", administrator: false, settings_ready: true } }), accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn() } };

describe("ResourceCenterPage", () => {
  it.each([
    ["experts", "listExpertTeams"],
    ["skills", "listSkills"],
    ["connectors", "listConnectorPublications"],
    ["knowledge", "listKnowledgeBases"],
  ] as const)("shows loading instead of empty content while opening %s", async (tab, method) => {
    let resolve!: (items: []) => void;
    const request = new Promise<[]>((done) => { resolve = done; });
    const testApi = api();
    Object.assign(testApi, { [method]: vi.fn(() => request) });
    const router = createAppRouter(createMemoryHistory());
    await router.push(`/resources?tab=${tab}`);
    const wrapper = mount(ResourceCenterPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: testApi, [authContextKey as symbol]: auth },
      },
    });
    try {
      await flushPromises();
      expect(testApi[method]).toHaveBeenCalledOnce();
      expect(wrapper.find(".el-skeleton").exists()).toBe(true);
      expect(wrapper.get('[role="status"]').attributes("aria-label")).toBe("加载中…");
      expect(wrapper.find(".el-empty").exists()).toBe(false);
      resolve([]);
      await flushPromises();
      expect(wrapper.find(".el-skeleton").exists()).toBe(false);
      expect(wrapper.find(".el-empty").exists()).toBe(true);
    } finally {
      resolve([]);
      wrapper.unmount();
    }
  });

  it.each([
    ["experts", "listExperts", "加载专家目录失败"],
    ["skills", "listSkills", "操作失败"],
    ["connectors", "listConnectorInstallations", "操作失败"],
    ["knowledge", "listKnowledgeBases", "知识库加载失败"],
  ] as const)("ends loading and displays the error when %s fails", async (tab, method, message) => {
    const testApi = api();
    Object.assign(testApi, { [method]: vi.fn().mockRejectedValue(new Error("request_failed")) });
    const router = createAppRouter(createMemoryHistory());
    await router.push(`/resources?tab=${tab}`);
    const wrapper = mount(ResourceCenterPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: testApi, [authContextKey as symbol]: auth },
      },
    });
    try {
      await flushPromises();
      expect(wrapper.find(".el-skeleton").exists()).toBe(false);
      expect(`${wrapper.text()} ${document.body.textContent}`).toContain(message);
    } finally {
      wrapper.unmount();
    }
  });

  it("organizes all reusable resources into one searchable library", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/resources");
    await router.isReady();
    const wrapper = mount(ResourceCenterPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api(), [authContextKey as symbol]: auth },
      },
    });
    await flushPromises();

    expect(wrapper.find(".resource-center-tabs").exists()).toBe(false);
    expect(wrapper.find(".resource-center-toolbar .el-input").exists()).toBe(true);
    expect(wrapper.find(".resource-center-toolbar .el-radio-group").exists()).toBe(false);
    expect(wrapper.find(".resource-center-filter-hint").exists()).toBe(false);
    expect(wrapper.getComponent(ExpertsPage).props("availableOnly")).toBe(false);
    expect(wrapper.find(".expert-catalog").exists()).toBe(true);
    expect(wrapper.find(".resource-tabs").exists()).toBe(false);

    await router.push("/resources?tab=skills");
    await flushPromises();

    expect(router.currentRoute.value.query.tab).toBe("skills");
    expect(wrapper.find(".expert-catalog").exists()).toBe(false);
    expect(wrapper.find(".resource-child-actions").exists()).toBe(false);
    expect(wrapper.get(".extension-catalog-toolbar").text()).toContain("我的技能");
    expect(wrapper.get(".extension-catalog-toolbar").text()).toContain("添加技能");
    expect(wrapper.find(".resource-tabs").exists()).toBe(false);

    await router.push("/resources?tab=knowledge");
    await flushPromises();
    expect(router.currentRoute.value.query.tab).toBe("knowledge");
    expect(wrapper.get(".knowledge-card").text()).toContain("交付手册");
    expect(wrapper.get(".knowledge-card").text()).toContain("个人创建");
    expect(wrapper.get(".knowledge-card").text()).toContain("仅我可见");
    expect(wrapper.get(".knowledge-card").text()).toContain("可检索");
    expect(wrapper.get(".knowledge-card").text()).toContain("1/2 份文档可检索");
    await wrapper.get(".knowledge-card").trigger("click");
    await flushPromises();
    expect(wrapper.find(".resource-center-toolbar").exists()).toBe(false);
    await wrapper.get(".knowledge-back").trigger("click");
    expect(wrapper.find(".resource-center-toolbar").exists()).toBe(true);
    await wrapper.get(".knowledge-card").trigger("click");
    await router.push("/resources?tab=experts");
    await flushPromises();
    await router.push("/resources?tab=knowledge");
    await flushPromises();
    expect(wrapper.find(".resource-center-toolbar").exists()).toBe(true);
    wrapper.unmount();
  });
});
