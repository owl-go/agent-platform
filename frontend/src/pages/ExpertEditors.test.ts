// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { authContextKey, type AuthContext } from "../auth/session";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type ExpertInput, type ExpertTeamInput, type PlatformApi } from "../api/client";
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
  it("saves one authoritative Markdown document without execution settings", async () => {
    const createExpert = vi.fn(async (input: ExpertInput) => ({ ...experts[0], ...input }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), createExpert } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts/new");
    const wrapper = mount(ExpertEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(wrapper.find(".editor-section > div:first-child > span").exists()).toBe(false);
    expect(wrapper.find(".extension-manager").exists()).toBe(false);
    expect(wrapper.find(".expert-resource-selector").exists()).toBe(true);
    expect(wrapper.find(".form-grid > label .icon-picker-upload").exists()).toBe(false);
    const inputs = wrapper.findAll('input[type="text"]');
    const textareas = wrapper.findAll("textarea");
    await inputs[0]!.setValue("架构专家");
    await wrapper.get('button[aria-label="code"]').trigger("click");
    await textareas[0]!.setValue("展示用简介");
    expect(textareas).toHaveLength(2);
    await textareas[1]!.setValue("# 架构设计\n\n先分析约束，再提出方案。");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createExpert).toHaveBeenCalledWith(expect.objectContaining({ icon: "code", introduction: "展示用简介", guidance: "# 架构设计\n\n先分析约束，再提出方案。" }));
    expect(wrapper.text()).not.toContain("运行引擎");
    wrapper.unmount();
  });

  it("keeps Expert Team member order when using accessible reorder controls", async () => {
    const createExpertTeam = vi.fn(async (input: ExpertTeamInput) => ({ id: "team-1", ...input, experts, available: true, created_at: "", updated_at: "", version: 1 }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), listExperts: vi.fn(async () => experts), listModelProviderConnections: vi.fn(async () => []), createExpertTeam } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/expert-teams/new");
    const wrapper = mount(ExpertTeamEditorPage, mountOptions(api, router, true));
    await flushPromises();
    expect(wrapper.find(".editor-section > div:first-child > span").exists()).toBe(false);
    await wrapper.get('button[aria-label="compass"]').trigger("click");
    const select = wrapper.get(".member-picker select");
    for (const expert of experts) {
      await select.setValue(expert.id);
      await wrapper.get(".member-picker button").trigger("click");
    }
    const lead = wrapper.get<HTMLSelectElement>(".lead-selector");
    await lead.setValue(lead.findAll<HTMLOptionElement>("option")[3]!.element.value);
    await wrapper.get('[aria-label="上移 测试工程师"]').trigger("click");
    expect(wrapper.findAll<HTMLInputElement>('.member-fields input[aria-label="成员名称"]').map((item) => item.element.value)).toEqual(["架构师", "测试工程师", "开发工程师"]);
    await wrapper.get(".member-guidance textarea").setValue("# 团队独立指引");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createExpertTeam).toHaveBeenCalledWith(expect.objectContaining({ icon: "compass" }));
    const sent = createExpertTeam.mock.calls[0]![0];
    expect(sent.members[0]!.definition?.guidance).toBe("# 团队独立指引");
    expect(experts[0]!.guidance).toBe("# 架构师\n\n独立指引");
    expect(sent.lead_member_id).toBe(sent.members[1]!.id);
    wrapper.unmount();
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
