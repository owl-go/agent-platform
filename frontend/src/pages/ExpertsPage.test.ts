// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type ExpertTeam, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import ExpertsPage from "./ExpertsPage.vue";

const expert: Expert = {
  id: "expert-1", name: "架构专家", icon: "sparkles", icon_background: "sage", introduction: "负责系统架构与边界设计", core_capability: "架构设计", operating_procedure: "审查需求", output_standard: "给出方案", cautions: "",
  complete: true, compatibility: "verified",
  expertise_tags: ["架构", "Go"], mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [], available: true,
  created_at: "2026-09-02T00:00:00Z", updated_at: "2026-09-02T00:00:00Z", version: 1,
};
const team: ExpertTeam = {
  id: "team-1", name: "交付专家团", icon: "users", icon_background: "sage", introduction: "按顺序完成设计与交付", core_capability: "交付", members: [], capability_introduction: "按顺序完成设计与交付", expertise_tags: ["交付"], experts: [expert, { ...expert, id: "expert-2", name: "开发专家" }], available: true,
  created_at: "2026-09-02T00:00:00Z", updated_at: "2026-09-02T00:00:00Z", version: 1,
};

function api(): PlatformApi {
  return { listExperts: vi.fn(async () => [expert]), listExpertTeams: vi.fn(async () => [team]), listModelProviderConnections: vi.fn(async () => [{ id: "connection-1", name: "OpenAI", models: [{ id: "model-1", display_name: "GPT 5", available: true, compatibility: [] }] }]) } as unknown as PlatformApi;
}

describe("ExpertsPage", () => {
  it("separates Administrator Experts from the current User's Experts", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts");
    const platformExpert = { ...expert, id: "platform-expert", name: "平台研究专家", platform: true };
    const testApi = api();
    testApi.listExperts = vi.fn(async () => [platformExpert, expert]);
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: testApi } } });
    await flushPromises();

    const groups = wrapper.findAll(".catalog-group");
    expect(groups).toHaveLength(2);
    expect(groups[0]!.text()).toContain("平台专家");
    expect(groups[0]!.text()).toContain("平台研究专家");
    expect(groups[0]!.text()).not.toContain("架构专家");
    expect(groups[1]!.text()).toContain("我的专家");
    expect(groups[1]!.text()).toContain("架构专家");
  });

  it("shows searchable Expert cards with capability and expertise tags", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts");
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api() } } });
    await flushPromises();

    expect(wrapper.get(".expert-card h2").text()).toBe("架构专家");
    expect(wrapper.get(".expert-card").text()).toContain("负责系统架构与边界设计");
    expect(wrapper.get(".expert-card").text()).toContain("架构");
    expect(wrapper.get(".expert-card").text()).toContain("0 个技能 · 0 个连接器");
    expect(wrapper.get(".expert-card").text()).not.toContain("Codex");
    expect(wrapper.find(".expert-card > .el-card__body").exists()).toBe(true);
    expect(wrapper.get(".tag-row .el-tag").classes()).toContain("is-round");
    expect(wrapper.find("a[href='/experts/new']").exists()).toBe(true);
  });

  it("switches to Expert Teams and discloses ordered per-turn members", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts?tab=teams");
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api() } } });
    await flushPromises();

    expect(wrapper.get(".expert-team-card").text()).toContain("交付专家团");
    expect(wrapper.get(".expert-team-card").text()).toContain("架构专家");
    expect(wrapper.get(".expert-team-card").text()).not.toContain("Codex");
    expect(wrapper.get(".expert-team-card").text()).toContain("2 位专家 / 每轮");
  });
});

for (const [path, field, id] of [["/experts", "expert_id", "expert-1"], ["/experts?tab=teams", "expert_team_id", "team-1"]]) {
  it(`summons from ${path} without also opening card details`, async () => {
    const router = createAppRouter(createMemoryHistory()); await router.push(path!);
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api() } } });
    await flushPromises();
    await wrapper.get(".catalog-launch").trigger("keydown", { key: "Enter" });
    await wrapper.get(".catalog-launch").trigger("click"); await flushPromises();
    expect(router.currentRoute.value.path).toBe("/sessions"); expect(router.currentRoute.value.query[field!]).toBe(id); expect(router.currentRoute.value.query.new).toBeTruthy();
    expect(wrapper.findComponent({ name: "CatalogDetails" }).props("expert")).toBeUndefined();
    expect(wrapper.findComponent({ name: "CatalogDetails" }).props("team")).toBeUndefined(); wrapper.unmount();
  });
}
