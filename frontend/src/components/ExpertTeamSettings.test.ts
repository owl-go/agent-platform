// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type Expert, type ExpertTeam, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ExpertTeamSettings from "./ExpertTeamSettings.vue";

const experts: Expert[] = ["Lead", "Reviewer", "Researcher"].map((name, index) => ({ id: `e${index}`, name, icon: "code", icon_background: "sky", introduction: `${name} capability`, guidance: `# ${name} original guidance`, skill_ids: [`s${index}`], mcp_server_ids: [], cli_connector_definition_ids: [], starter_prompts: ["Review evidence"], connector_dependencies: [{ source: "tools", kind: "mcp", version: "1.0.0" }], complete: true, available: true, compatibility: "verified", expertise_tags: [], created_at: "", updated_at: "", version: 1 }));
const team: ExpertTeam = { id: "team", name: "Existing team", introduction: "Old description", core_capability: "Original core", icon: "users", icon_background: "sand", starter_prompts: ["Review together"], mutable: true, lead_member_id: "lead", version: 8, members: [{ id: "lead", name: "Lead", expert_id: "", labels: ["Coordination"], position: 1, expert: { ...experts[0]!, guidance: "# Team-owned lead", skill_ids: ["frozen-skill"] } }, { id: "reviewer", name: "Reviewer", expert_id: "", labels: ["Evidence"], position: 2, expert: experts[1]! }], experts, available: true, expertise_tags: [], created_at: "", updated_at: "" };
function options(api: unknown, administrator = true) {
 return { attachTo: document.body, global: { plugins: [createAppI18n({getItem: () => "zh-CN"}, "zh-CN")], provide: { [platformApiKey as symbol]: api as PlatformApi, [authContextKey as symbol]: {session: {state: {value: {kind: "authenticated", currentUser: {administrator}}}}} as unknown as AuthContext } } };
}
afterEach(() => document.body.replaceChildren());
async function selectMembers(wrapper: ReturnType<typeof mount>, values: string[]) {
 wrapper.getComponent({name: "ElSelect"}).vm.$emit("update:modelValue", values); await flushPromises();
}
async function selectLead(wrapper: ReturnType<typeof mount>, id: string) {
 wrapper.findAllComponents({name: "ElSelect"})[1]!.vm.$emit("update:modelValue", id); await flushPromises();
}
it("creates a Team from four settings and copies selected Expert definitions", async () => {
 const api = {listExperts: vi.fn(async () => experts), createExpertTeam: vi.fn(async () => team)};
 const wrapper = mount(ExpertTeamSettings, options(api)); await flushPromises();
 await wrapper.get('input[aria-label="专家团名称"]').setValue("New team"); await wrapper.get('textarea[aria-label="团队描述"]').setValue("Review a proposal");
 await selectMembers(wrapper, ["expert:e0", "expert:e1"]);
 const memberIDs = wrapper.getComponent({name: "ElSelect"}).props("modelValue") as string[];
 const leadID = memberIDs[1]!.slice(7); await selectLead(wrapper, leadID);
 await wrapper.get("form").trigger("submit"); await flushPromises();
 expect(api.createExpertTeam).toHaveBeenCalledWith(expect.objectContaining({name: "New team", introduction: "Review a proposal", core_capability: "Review a proposal", lead_member_id: leadID, members: [expect.objectContaining({expert_id: "e0", definition: expect.objectContaining({guidance: experts[0]!.guidance, skill_ids: ["s0"]})}), expect.objectContaining({expert_id: "e1"})]}));
 expect(wrapper.emitted("saved")).toHaveLength(1); expect(wrapper.findAll("textarea")).toHaveLength(1); expect(wrapper.findAllComponents({name: "ElSelect"})).toHaveLength(2); wrapper.unmount();
});
it("edits settings while preserving owned member definitions, metadata and stable identities", async () => {
 const api = {listExperts: vi.fn(async () => experts), updateExpertTeam: vi.fn(async () => team)};
 const wrapper = mount(ExpertTeamSettings, {props: {team}, ...options(api)}); await flushPromises();
 await wrapper.get('input[aria-label="专家团名称"]').setValue("Changed team"); await wrapper.get('textarea[aria-label="团队描述"]').setValue("Changed description");
 await selectMembers(wrapper, ["member:reviewer", "member:lead", "expert:e2"]); await selectLead(wrapper, "reviewer");
 await wrapper.get("form").trigger("submit"); await flushPromises();
 expect(api.updateExpertTeam).toHaveBeenCalledWith("team", expect.objectContaining({name: "Changed team", introduction: "Changed description", core_capability: team.core_capability, icon: team.icon, icon_background: team.icon_background, starter_prompts: team.starter_prompts, lead_member_id: "reviewer", members: [expect.objectContaining({id: "reviewer", labels: ["Evidence"]}), expect.objectContaining({id: "lead", labels: ["Coordination"], definition: expect.objectContaining({guidance: "# Team-owned lead", skill_ids: ["frozen-skill"], connector_dependencies: experts[0]!.connector_dependencies})}), expect.objectContaining({expert_id: "e2", definition: expect.objectContaining({guidance: experts[2]!.guidance})})]}), 8);
 expect(team.name).toBe("Existing team"); expect(team.members).toHaveLength(2); wrapper.unmount();
});
it("requires a new lead after removing the current lead and rejects invalid settings", async () => {
 const api = {listExperts: vi.fn(async () => experts), updateExpertTeam: vi.fn(async () => team)};
 const wrapper = mount(ExpertTeamSettings, {props: {team}, ...options(api)}); await flushPromises();
 await selectMembers(wrapper, ["member:reviewer", "expert:e2"]);
 expect(wrapper.findAllComponents({name: "ElSelect"})[1]!.props("modelValue")).toBe("");
 await wrapper.get("form").trigger("submit"); await flushPromises(); expect(api.updateExpertTeam).not.toHaveBeenCalled(); expect(wrapper.text()).toContain("指定领队");
 await selectLead(wrapper, "reviewer"); await wrapper.get('input[aria-label="专家团名称"]').setValue(" "); await wrapper.get("form").trigger("submit"); await flushPromises(); expect(api.updateExpertTeam).not.toHaveBeenCalled();
 await wrapper.get('input[aria-label="专家团名称"]').setValue("名".repeat(34)); await wrapper.get("form").trigger("submit"); await flushPromises(); expect(api.updateExpertTeam).not.toHaveBeenCalled();
 await wrapper.get('input[aria-label="专家团名称"]').setValue("Valid"); await selectMembers(wrapper, ["member:reviewer"]); await wrapper.get("form").trigger("submit"); await flushPromises(); expect(api.updateExpertTeam).not.toHaveBeenCalled(); wrapper.unmount();
});
it("keeps failed drafts and blocks duplicate saves and late completion after closing", async () => {
 let reject!: (error: Error) => void, resolve!: (value: ExpertTeam) => void;
 const api = {listExperts: vi.fn(async () => experts), updateExpertTeam: vi.fn().mockImplementationOnce(() => new Promise((_, fail) => {reject = fail;})).mockImplementationOnce(() => new Promise(done => {resolve = done;}))};
 const wrapper = mount(ExpertTeamSettings, {props: {team}, ...options(api)}); await flushPromises();
 await wrapper.get('input[aria-label="专家团名称"]').setValue("Draft"); await wrapper.get("form").trigger("submit"); await wrapper.get("form").trigger("submit"); expect(api.updateExpertTeam).toHaveBeenCalledTimes(1); expect(wrapper.getComponent({name: "ElSelect"}).props("disabled")).toBe(true);
 reject(new ApiError("conflict", 412, "version_conflict", "save-conflict-request")); await flushPromises(); expect(wrapper.text()).toContain("数据已被更新"); expect(wrapper.text()).toContain("save-conflict-request"); expect(wrapper.get('input[aria-label="专家团名称"]').element).toHaveProperty("value", "Draft");
 await wrapper.get("form").trigger("submit"); wrapper.unmount(); resolve(team); await flushPromises(); expect(wrapper.emitted("saved")).toBeUndefined();
});
it("supports retry after catalog load failure and cannot save before the catalog loads", async () => {
 const api = {listExperts: vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(experts), createExpertTeam: vi.fn()};
 const wrapper = mount(ExpertTeamSettings, options(api)); await flushPromises(); await wrapper.get("form").trigger("submit"); expect(api.createExpertTeam).not.toHaveBeenCalled(); expect(wrapper.text()).toContain("加载专家列表失败");
 await wrapper.findAll("button").find(button => button.text().includes("重试"))!.trigger("click"); await flushPromises(); expect(wrapper.find('input[aria-label="专家团名称"]').exists()).toBe(true); wrapper.unmount();
});
for (const [administrator, value] of [[false, undefined], [false, team], [true, {...team, immutable: true}], [true, {...team, mutable: false}]] as const) {
 it("keeps ordinary Users and non-maintainers out of Team settings", async () => {
  const api = {listExperts: vi.fn(async () => experts), updateExpertTeam: vi.fn(), createExpertTeam: vi.fn()};
  const wrapper = mount(ExpertTeamSettings, {props: {team: value}, ...options(api, administrator)}); await flushPromises(); expect(wrapper.find("form").exists()).toBe(false); expect(api.listExperts).not.toHaveBeenCalled(); wrapper.unmount();
 });
}
it("keeps retained members even when their source Experts disappeared from the catalog", async () => {
 const api = {listExperts: vi.fn(async () => [{...experts[2]!, available: false}]), updateExpertTeam: vi.fn(async () => team)};
 const wrapper = mount(ExpertTeamSettings, {props: {team}, ...options(api)}); await flushPromises();
 const members = wrapper.getComponent({name: "ElSelect"}); expect(members.props("modelValue")).toEqual(["member:lead", "member:reviewer"]); expect(members.findAllComponents({name: "ElOption"})).toHaveLength(2);
 await wrapper.get("form").trigger("submit"); await flushPromises(); expect(api.updateExpertTeam).toHaveBeenCalledTimes(1); wrapper.unmount();
});
