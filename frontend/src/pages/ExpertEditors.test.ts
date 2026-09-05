// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type ExpertInput, type ExpertTeamInput, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import ExpertEditorPage from "./ExpertEditorPage.vue";
import ExpertTeamEditorPage from "./ExpertTeamEditorPage.vue";

const experts: Expert[] = ["架构师", "开发工程师", "测试工程师"].map((name, index) => ({
  id: `expert-${index + 1}`, name, icon: "sparkles", icon_background: "sage", introduction: `${name}简介`, core_capability: `${name}能力`, operating_procedure: `${name}工作流程`, output_standard: `${name}输出规范`, cautions: "",
  complete: true, compatibility: "verified",
  expertise_tags: [], mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [], available: true,
  created_at: "2026-09-03T00:00:00Z", updated_at: "2026-09-03T00:00:00Z", version: 1,
}));

function mountOptions(api: PlatformApi, router: ReturnType<typeof createAppRouter>) {
  return { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } };
}

describe("Expert editors", () => {
  it("saves visible structured guidance without execution settings", async () => {
    const createExpert = vi.fn(async (input: ExpertInput) => ({ ...experts[0], ...input }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), createExpert } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts/new");
    const wrapper = mount(ExpertEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(wrapper.find(".editor-section > div:first-child > span").exists()).toBe(false);
    const inputs = wrapper.findAll("input");
    const textareas = wrapper.findAll("textarea");
    await inputs[0]!.setValue("架构专家");
    await textareas[0]!.setValue("展示用简介");
    await textareas[1]!.setValue("架构设计与边界治理");
    await textareas[2]!.setValue("先分析约束，再提出方案");
    await textareas[3]!.setValue("输出决策、理由和验证计划");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createExpert).toHaveBeenCalledWith(expect.objectContaining({ introduction: "展示用简介", core_capability: "架构设计与边界治理", operating_procedure: "先分析约束，再提出方案", output_standard: "输出决策、理由和验证计划" }));
    expect(wrapper.text()).not.toContain("运行引擎");
    wrapper.unmount();
  });

  it("keeps Expert Team member order when using accessible reorder controls", async () => {
    const createExpertTeam = vi.fn(async (input: ExpertTeamInput) => ({ id: "team-1", ...input, experts, available: true, created_at: "", updated_at: "", version: 1 }));
    const api = { listExperts: vi.fn(async () => experts), listModelProviderConnections: vi.fn(async () => []), createExpertTeam } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/expert-teams/new");
    const wrapper = mount(ExpertTeamEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(wrapper.find(".editor-section > div:first-child > span").exists()).toBe(false);
    const select = wrapper.get(".member-picker select");
    for (const expert of experts) {
      await select.setValue(expert.id);
      await wrapper.get(".member-picker button").trigger("click");
    }
    await wrapper.get('[aria-label="上移 测试工程师"]').trigger("click");
    expect(wrapper.findAll<HTMLInputElement>('.member-fields input[aria-label="成员名称"]').map((item) => item.element.value)).toEqual(["架构师", "测试工程师", "开发工程师"]);
    wrapper.unmount();
  });
});
