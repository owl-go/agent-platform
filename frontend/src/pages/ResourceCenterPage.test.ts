// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
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
    listKnowledgeBases: vi.fn(async () => [{ id: "knowledge-1", owner_id: "user-1", name: "交付手册", description: "部署和验收", visibility: "private", platform: false, deleted: false, created_at: "2026-09-28T00:00:00Z", updated_at: "2026-09-28T00:00:00Z", version: 1, document_count: 2, ready_document_count: 1, last_ready_at: "2026-09-28T00:00:00Z" }]),
  } as unknown as PlatformApi;
}

const auth: AuthContext = { isCallback: false, session: { state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "user@example.test", display_name: "User", administrator: false, settings_ready: true } }), accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn() } };

describe("ResourceCenterPage", () => {
  it("organizes all reusable resources into one availability-filtered library", async () => {
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
    expect(wrapper.get(".resource-center-filter-hint").text()).toContain("不推荐未测试");
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
    wrapper.unmount();
  });
});
