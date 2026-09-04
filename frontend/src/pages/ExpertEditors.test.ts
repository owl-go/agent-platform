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
  id: `expert-${index + 1}`, name, capability_introduction: `${name}能力`, execution_instruction: `${name}执行指令`,
  provider_model_id: "model-1", provider_model_name: "GPT 5", runtime_engine: "codex",
  complete: true, compatibility: "verified",
  expertise_tags: [], mcp_server_ids: [], skill_ids: [], available: true,
  created_at: "2026-09-03T00:00:00Z", updated_at: "2026-09-03T00:00:00Z", version: 1,
}));

function mountOptions(api: PlatformApi, router: ReturnType<typeof createAppRouter>) {
  return { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } };
}

describe("Expert editors", () => {
  it("saves capability introduction and the separate execution instruction", async () => {
    const createExpert = vi.fn(async (input: ExpertInput) => ({ ...experts[0], ...input }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listModelProviderConnections: vi.fn(async () => []), listRuntimeEngines: vi.fn(async () => []), getSettings: vi.fn(async () => ({ default_runtime_engine: "codex", runtime_model_defaults: [], personality: "direct", personality_instructions: "", language: "zh-CN", timezone: "Asia/Shanghai", version: 1 })), createExpert } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/experts/new");
    const wrapper = mount(ExpertEditorPage, mountOptions(api, router));
    await flushPromises();
    expect(wrapper.find(".editor-section > div:first-child > span").exists()).toBe(false);
    const inputs = wrapper.findAll("input");
    const textareas = wrapper.findAll("textarea");
    await inputs[0]!.setValue("架构专家");
    await textareas[0]!.setValue("展示用能力介绍");
    await textareas[1]!.setValue("只注入这段执行指令");
    await wrapper.get(".profile-confirm input").setValue(true);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(createExpert).toHaveBeenCalledWith(expect.objectContaining({ capability_introduction: "展示用能力介绍", execution_instruction: "只注入这段执行指令" }));
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
    expect(wrapper.findAll(".ordered-members strong").map((item) => item.text())).toEqual(["架构师", "测试工程师", "开发工程师"]);
    wrapper.unmount();
  });
});
