// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { authContextKey, type AuthContext } from "../auth/session";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import ExpertEditorPage from "./ExpertEditorPage.vue";
import ExpertTeamEditorPage from "./ExpertTeamEditorPage.vue";

const experts: Expert[] = ["架构师", "开发工程师", "测试工程师"].map((name, index) => ({
  id: `expert-${index + 1}`, name, guidance: `# ${name}\n\n独立指引`, icon: "sparkles", icon_background: "sage", introduction: `${name}简介`, core_capability: `${name}能力`, operating_procedure: `${name}工作流程`, output_standard: `${name}输出规范`, cautions: "",
  complete: true, compatibility: "verified",
  expertise_tags: [], mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [], available: true,
  created_at: "2026-09-03T00:00:00Z", updated_at: "2026-09-03T00:00:00Z", version: 1,
}));

function mountOptions(api: PlatformApi, router: ReturnType<typeof createAppRouter>, administrator = false) {
  const auth = { session: { state: { value: { kind: "authenticated", currentUser: { administrator } } } } } as unknown as AuthContext;
  return { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth } } };
}

describe("Expert editors", () => {
  it("routes direct Expert creation through the Create Expert Skill", async () => {
    const api = { createExpert: vi.fn() } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts/new");
    const wrapper = mount(ExpertEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(router.currentRoute.value.path).toBe("/sessions");
    expect(router.currentRoute.value.query.create_expert).toBe("true");
    expect(api.createExpert).not.toHaveBeenCalled();
    expect(wrapper.find("textarea").exists()).toBe(false);
    wrapper.unmount();
  });

  it("opens the same compact Team settings on the direct creation route", async () => {
    const api = { listExperts: vi.fn(async () => experts) } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory()); await router.push("/expert-teams/new");
    const wrapper = mount(ExpertTeamEditorPage, mountOptions(api, router, true)); await flushPromises();
    expect(wrapper.findComponent({name: "ExpertTeamSettings"}).exists()).toBe(true);
    expect(wrapper.find('input[aria-label="专家团名称"]').exists()).toBe(true);
    expect(wrapper.find('textarea[aria-label="团队描述"]').exists()).toBe(true);
    expect(wrapper.find(".member-guidance").exists()).toBe(false); wrapper.unmount();
  });

  it("does not expose Team creation controls to an ordinary User opening the route", async () => {
    const listExperts = vi.fn(async () => experts);
    const api = { listExperts } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/expert-teams/new");
    const wrapper = mount(ExpertTeamEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(wrapper.find("form").exists()).toBe(false);
    expect(listExperts).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});

it("opens existing Team editing with name, description, members and lead", async () => {
 const team = {id: "team", name: "Review team", mutable: true, version: 1, lead_member_id: "lead", introduction: "Review together", core_capability: "Review", starter_prompts: ["Review a plan"], members: [{id: "lead", name: "Lead", labels: ["Coordination"], expert: experts[0]}, {id: "reviewer", name: "Reviewer", labels: ["Review"], expert: experts[1]}]};
 const api = {listExperts: vi.fn(async () => experts), getExpertTeam: vi.fn(async () => team), listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => [])} as unknown as PlatformApi;
 const router = createAppRouter(createMemoryHistory()); await router.push("/expert-teams/team");
 const wrapper = mount(ExpertTeamEditorPage, mountOptions(api, router, true)); await flushPromises();
 expect(api.getExpertTeam).toHaveBeenCalledWith("team"); expect(api.listExperts).toHaveBeenCalledTimes(1); expect(api.listSkills).not.toHaveBeenCalled();
 expect(wrapper.get('input[aria-label="专家团名称"]').element).toHaveProperty("value", "Review team"); expect(wrapper.get('textarea[aria-label="团队描述"]').element).toHaveProperty("value", "Review together"); expect(wrapper.findAllComponents({name: "ElSelect"})).toHaveLength(2); wrapper.unmount();
});
