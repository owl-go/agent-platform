// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type ExpertTeam, type PlatformApi, type Skill, type MCPServer, type CLIConnectorDefinition } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ExpertBindingsEditor from "./ExpertBindingsEditor.vue";
import ExpertResourceSelector from "./ExpertResourceSelector.vue";

const expert = { id: "expert", version: 7, name: "Reviewer", introduction: "Capability", guidance: "# Independent guidance", icon: "code", icon_background: "sky", starter_prompts: ["Review evidence"], connector_dependencies: [{ source: "tools", kind: "mcp", version: "1.0.0" }], skill_ids: ["original"], mcp_server_ids: [], cli_connector_definition_ids: [], available: true, expertise_tags: [], complete: true, compatibility: "verified", created_at: "", updated_at: "" } satisfies Expert;
const resources = () => ({
  listSkills: vi.fn(async () => [{ id: "original", name: "Original" }, { id: "new", name: "New" }] as Skill[]),
  listMCPServers: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []),
  listCLIConnectorEnablements: vi.fn(async () => []),
});
function options(api: unknown, administrator = false) {
  return { attachTo: document.body, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: {
    [platformApiKey as symbol]: api as PlatformApi,
    [authContextKey as symbol]: { session: { state: { value: { kind: "authenticated", currentUser: { administrator } } } } } as unknown as AuthContext,
  } } };
}
afterEach(() => document.body.replaceChildren());
it("edits bindings in two compact dropdowns and preserves the complete profile", async () => {
  const api = { ...resources(), updateExpert: vi.fn(async () => expert) };
  const wrapper = mount(ExpertBindingsEditor, { props: { expert }, ...options(api) });
  await flushPromises();
  expect(wrapper.find("textarea").exists()).toBe(false);
  expect(wrapper.findAllComponents({ name: "ElSelect" })).toHaveLength(2);
  expect(wrapper.find(".expert-resource-options").exists()).toBe(false);
  wrapper.getComponent(ExpertResourceSelector).vm.$emit("update:skillIds", ["new"]);
  await wrapper.get("form").trigger("submit"); await flushPromises();
  expect(api.updateExpert).toHaveBeenCalledWith("expert", expect.objectContaining({
    name: expert.name, introduction: expert.introduction, guidance: expert.guidance,
    icon: expert.icon, icon_background: expert.icon_background,
    starter_prompts: expert.starter_prompts, connector_dependencies: expert.connector_dependencies,
    skill_ids: ["new"], mcp_server_ids: [], cli_connector_definition_ids: [],
  }), 7);
  expect(expert.skill_ids).toEqual(["original"]);
  expect(wrapper.emitted("saved")).toHaveLength(1); wrapper.unmount();
});
it("blocks duplicate saves, preserves a failed draft and ignores completion after closing", async () => {
  let reject!: (error: Error) => void;
  let resolve!: (value: Expert) => void;
  const api = { ...resources(), updateExpert: vi.fn().mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; })).mockImplementationOnce(() => new Promise(done => { resolve = done; })) };
  const wrapper = mount(ExpertBindingsEditor, { props: { expert }, ...options(api) }); await flushPromises();
  wrapper.getComponent(ExpertResourceSelector).vm.$emit("update:skillIds", ["new"]);
  await wrapper.get("form").trigger("submit"); await wrapper.get("form").trigger("submit");
  expect(api.updateExpert).toHaveBeenCalledTimes(1);
  expect(wrapper.getComponent(ExpertResourceSelector).props("disabled")).toBe(true);
  reject(new Error("conflict")); await flushPromises();
  expect(wrapper.text()).toContain("保存失败");
  expect(wrapper.getComponent(ExpertResourceSelector).props("skillIds")).toEqual(["new"]);
  await wrapper.get("form").trigger("submit"); wrapper.unmount();
  resolve(expert); await flushPromises(); expect(wrapper.emitted("saved")).toBeUndefined();
});
it("does not save after a resource catalog failed to load", async () => {
  const api = { ...resources(), listSkills: vi.fn(async () => { throw new Error("offline"); }), updateExpert: vi.fn() };
  const wrapper = mount(ExpertBindingsEditor, { props: { expert }, ...options(api) }); await flushPromises();
  await wrapper.get("form").trigger("submit"); expect(api.updateExpert).not.toHaveBeenCalled(); wrapper.unmount();
});
it("preserves the Team Lead, member roles and guidance when changing member bindings", async () => {
  const team = { id: "team", version: 4, mutable: true, name: "Team", icon: "users", icon_background: "sage", introduction: "Team capability", core_capability: "Combined ability", starter_prompts: ["Review together"], lead_member_id: "lead", members: [
    { id: "lead", name: "Lead", labels: ["Coordination"], expert, expert_id: "", position: 1 },
    { id: "reviewer", name: "Reviewer", labels: ["Evidence"], expert: { ...expert, id: "reviewer" }, expert_id: "", position: 2 },
  ], experts: [], expertise_tags: [], available: true, created_at: "", updated_at: "" } satisfies ExpertTeam;
  const api = { ...resources(), updateExpertTeam: vi.fn(async () => team) };
  const wrapper = mount(ExpertBindingsEditor, { props: { team }, ...options(api, true) }); await flushPromises();
  wrapper.findAllComponents(ExpertResourceSelector)[1]!.vm.$emit("update:skillIds", ["new"]);
  await wrapper.get("form").trigger("submit"); await flushPromises();
  expect(api.updateExpertTeam).toHaveBeenCalledWith("team", expect.objectContaining({
    lead_member_id: "lead", starter_prompts: ["Review together"],
    members: [expect.objectContaining({ id: "lead", labels: ["Coordination"], definition: expect.objectContaining({ guidance: expert.guidance, skill_ids: ["original"] }) }), expect.objectContaining({ id: "reviewer", labels: ["Evidence"], definition: expect.objectContaining({ guidance: expert.guidance, skill_ids: ["new"] }) })],
  }), 4); wrapper.unmount();
});
for (const props of ([{ expert: { ...expert, platform: true } }, { expert: { ...expert, immutable: true } }, { team: { id: "team", mutable: true, members: [] } as unknown as ExpertTeam }] as { expert?: Expert; team?: ExpertTeam }[])) {
  it("keeps resources read-only for a non-maintainer", async () => {
    const api = { ...resources(), updateExpert: vi.fn(), updateExpertTeam: vi.fn() };
    const wrapper = mount(ExpertBindingsEditor, { props, ...options(api) }); await flushPromises();
    expect(wrapper.find("form").exists()).toBe(false); expect(api.listSkills).not.toHaveBeenCalled(); wrapper.unmount();
  });
}
it("keeps MCP and CLI identifiers separate and disables unavailable additions", async () => {
  const wrapper = mount(ExpertResourceSelector, { props: {
    mcpServers: [{ id: "same", name: "MCP", tested: true }, { id: "untested", name: "Untested", tested: false }] as MCPServer[],
    cliConnectors: [{ id: "same", name: "CLI" }, { id: "disabled", name: "Disabled" }] as CLIConnectorDefinition[],
    cliEnablements: [{ definition_id: "same", state: "enabled" } as never],
  }, ...options({}) });
  const selects = wrapper.findAllComponents({ name: "ElSelect" });
  selects[1]!.vm.$emit("update:modelValue", ["mcp:same", "cli:same"]); await flushPromises();
  expect(wrapper.emitted("update:mcpServerIds")).toEqual([[["same"]]]);
  expect(wrapper.emitted("update:cliConnectorDefinitionIds")).toEqual([[["same"]]]);
  const unavailable = wrapper.findAllComponents({ name: "ElOption" }).filter(option => ["mcp:untested", "cli:disabled"].includes(option.props("value")));
  expect(unavailable).toHaveLength(2); expect(unavailable.every(option => option.props("disabled"))).toBe(true); wrapper.unmount();
});
