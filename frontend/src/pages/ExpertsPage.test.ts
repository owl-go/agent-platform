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
    const unverifiedExpert = { ...expert, id: "unverified-expert", name: "待验证专家", compatibility: "unverified" as const };
    const testApi = api();
    testApi.listExperts = vi.fn(async () => [platformExpert, expert, unverifiedExpert]);
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: testApi } } });
    await flushPromises();

    const groups = wrapper.findAll(".catalog-group");
    expect(groups).toHaveLength(2);
    expect(groups[0]!.text()).toContain("平台专家");
    expect(groups[0]!.text()).toContain("平台研究专家");
    expect(groups[0]!.text()).toContain("平台发布");
    expect(groups[0]!.text()).toContain("可用");
    expect(groups[1]!.text()).toContain("我的专家");
    expect(groups[1]!.text()).toContain("架构专家");
    expect(groups[1]!.text()).toContain("仅我可见");
    expect(wrapper.text()).toContain("待验证专家");
  });

  it("shows searchable Expert cards without automatic tags or categories", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts?scope=mine");
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api() } } });
    await flushPromises();

    expect(wrapper.get(".expert-card h2").text()).toBe("架构专家");
    expect(wrapper.get(".expert-card").text()).toContain("负责系统架构与边界设计");
    expect(wrapper.get(".expert-card").text()).toContain("架构");
    expect(wrapper.get(".expert-card").text()).toContain("0 个技能 · 0 个连接器");
    expect(wrapper.get(".expert-card").text()).not.toContain("Codex");
    expect(wrapper.find(".expert-card > .el-card__body").exists()).toBe(true);
    expect(wrapper.find(".expert-card-heading .profile-icon").exists()).toBe(true);
    expect(wrapper.find(".expert-category").exists()).toBe(false);
    expect(wrapper.find(".filter-label").exists()).toBe(false);
    expect(wrapper.find(".tag-filter").exists()).toBe(false);
    expect(wrapper.find(".expert-tags").exists()).toBe(false);
    expect(wrapper.find("a[href='/experts/new']").exists()).toBe(false);
    expect(wrapper.get(".catalog-head-actions").text()).toContain("添加专家");
    expect(wrapper.get(".catalog-head-actions").findAll("button")).toHaveLength(2);
    expect(wrapper.find(".package-copy-name").exists()).toBe(false);
    expect(wrapper.get(".catalog-head-actions").find(".el-select").exists()).toBe(false);
  });

  it("reveals only my Experts from the header action", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts");
    const platformExpert = { ...expert, id: "platform-expert", name: "平台研究专家", platform: true };
    const testApi = api();
    testApi.listExperts = vi.fn(async () => [platformExpert, expert]);
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: testApi } } });
    await flushPromises();
    await wrapper.get(".my-resource-toggle").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.query.scope).toBe("mine");
    expect(wrapper.findAll(".catalog-group")).toHaveLength(1);
    expect(wrapper.get(".catalog-group").text()).toContain("架构专家");
    expect(wrapper.get(".catalog-group").text()).not.toContain("平台研究专家");
  });

  it("switches to Expert Teams and discloses independent members", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts?tab=teams");
    const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api() } } });
    await flushPromises();

    expect(wrapper.get(".expert-team-card").text()).toContain("交付专家团");
    expect(wrapper.get(".expert-team-card").text()).toContain("架构专家");
    expect(wrapper.get(".expert-team-card").text()).not.toContain("Codex");
    expect(wrapper.get(".expert-team-card").text()).toContain("2 位成员");
    expect(wrapper.find("a[href='/expert-teams/new']").exists()).toBe(false);
  });
});

for (const [path, field, id] of [["/experts?scope=mine", "expert_id", "expert-1"], ["/experts?tab=teams", "expert_team_id", "team-1"]]) {
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

it("imports an Expert ZIP and shows its saved definition without starting a Session", async () => {
 const router=createAppRouter(createMemoryHistory());await router.push("/experts");
 const testApi=api();testApi.importExpertPackage=vi.fn(async()=>({expert:{...expert,guidance:"# Imported"},package_id:"example.expert",package_version:"1.0.0",replayed:false}));
 const wrapper=mount(ExpertsPage,{global:{plugins:[router,createAppI18n({getItem:()=>"zh-CN"},"zh-CN")],provide:{[platformApiKey as symbol]:testApi}}});await flushPromises();
 const file=new File(["fixture"],"expert.zip",{type:"application/zip"});
 const input=wrapper.get<HTMLInputElement>("input.package-import-file");Object.defineProperty(input.element,"files",{value:[file]});await input.trigger("change");await flushPromises();
 expect(testApi.importExpertPackage).toHaveBeenCalledWith(file);
 expect(wrapper.findComponent({name:"CatalogDetails"}).props("expert")?.guidance).toBe("# Imported");expect(router.currentRoute.value.path).toBe("/resources");wrapper.unmount();
});

it("creates through the Create Expert Skill from the add menu", async () => {
 const router = createAppRouter(createMemoryHistory()); await router.push("/experts");
 const testApi = api(); const wrapper = mount(ExpertsPage, { global: { plugins: [router, createAppI18n({getItem: () => "zh-CN"}, "zh-CN")], provide: {[platformApiKey as symbol]: testApi} } }); await flushPromises();
 wrapper.getComponent({name: "ElDropdown"}).vm.$emit("command", "create"); await flushPromises();
 expect(router.currentRoute.value.path).toBe("/sessions"); expect(router.currentRoute.value.query.create_expert).toBe("true"); expect(router.currentRoute.value.query.new).toBeTruthy(); wrapper.unmount();
});
it("rejects invalid ZIP files before upload and preserves the catalog on server rejection", async () => {
 const router = createAppRouter(createMemoryHistory()); await router.push("/experts");
 const testApi = api(); testApi.importExpertPackage = vi.fn(async () => { throw new Error("invalid profile"); });
 const wrapper = mount(ExpertsPage, {global: {plugins: [router, createAppI18n({getItem: () => "zh-CN"}, "zh-CN")], provide: {[platformApiKey as symbol]: testApi}}}); await flushPromises();
 const input = wrapper.get<HTMLInputElement>("input.package-import-file");
 Object.defineProperty(input.element, "files", {value: [new File(["invalid"], "expert.json")], configurable: true}); await input.trigger("change"); await flushPromises();
 expect(testApi.importExpertPackage).not.toHaveBeenCalled(); expect(wrapper.text()).toContain("100 MiB");
 Object.defineProperty(input.element, "files", {value: [new File(["invalid"], "expert.zip")], configurable: true}); await input.trigger("change"); await flushPromises();
 expect(testApi.importExpertPackage).toHaveBeenCalledTimes(1); expect(wrapper.text()).toContain("导入失败"); expect(wrapper.get(".expert-card h2").text()).toBe("架构专家"); expect(input.element.value).toBe(""); wrapper.unmount();
});
