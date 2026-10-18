// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type PlatformApi, type Skill, type Expert, type ExpertTeam } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import CatalogDetails from "./CatalogDetails.vue";

afterEach(() => document.body.replaceChildren());
it("opens an Expert modal with only capability and common tasks, without copy or instruction details", async () => {
 const expert = { id: "expert", name: "Reviewer", introduction: "Review supplied evidence", starter_prompts: ["Review this proposal", "Check the evidence"], guidance: "HIDDEN_GUIDANCE", core_capability: "STALE_FORM", mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [], available: true } as unknown as Expert;
 const api = { listSkills: vi.fn() } as unknown as PlatformApi;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources");
 const wrapper = mount(CatalogDetails, { attachTo: document.body, props: { expert }, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
 await flushPromises();
 expect(document.body.querySelector(".el-dialog")?.textContent).toContain("Reviewer");
 expect(document.body.querySelector(".el-drawer")).toBeNull();
 expect(document.body.textContent).toContain("Review supplied evidence");
 expect(document.body.textContent).toContain("Review this proposal");
 expect(document.body.textContent).not.toContain("HIDDEN_GUIDANCE");
 expect(document.body.textContent).not.toContain("STALE_FORM");
 expect(document.body.textContent).not.toContain("另存");
 expect(api.listSkills).not.toHaveBeenCalled();
 wrapper.unmount();
});
it("renders installed Skill Markdown safely and launches a preselected new Session", async () => {
 const skill = { id: "pdf", name: "PDF 文档处理", source: "upload", version: 2 } as Skill;
 const api = { getSkillDocument: vi.fn(async () => ({ skill, content: '# Document\n\n<script>window.untrusted=true</script>\n\n[unsafe](javascript:alert(1))\n\n![remote](https://example.test/tracker.png)' })) } as unknown as PlatformApi;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources"); await router.isReady();
 const wrapper = mount(CatalogDetails, { attachTo: document.body, props: { skill }, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
 await flushPromises();
 expect(api.getSkillDocument).toHaveBeenCalledWith("pdf"); expect(document.body.querySelector(".skill-document h1")?.textContent).toBe("Document");
 expect(document.body.querySelector(".skill-document script, .skill-document img, a[href^='javascript:']")).toBeNull();
 const launch = [...document.body.querySelectorAll<HTMLButtonElement>("button")].find(button => button.textContent?.includes("去使用")); expect(launch).toBeDefined(); launch!.click(); await flushPromises();
 expect(router.currentRoute.value.path).toBe("/sessions"); expect(router.currentRoute.value.query.skill_id).toBe("pdf"); expect(router.currentRoute.value.query.new).toBeTruthy(); expect(wrapper.emitted("close")).toBeTruthy(); wrapper.unmount();
});

it("does not offer editing for a platform Skill to an ordinary User", async () => {
 const skill = { id: "platform-pdf", name: "平台 PDF", source: "upload", version: 1, platform: true } as Skill;
 const api = { getSkillDocument: vi.fn(async () => ({ skill, content: "# PDF" })) } as unknown as PlatformApi;
 const auth = { session: { state: { value: { kind: "authenticated", currentUser: { administrator: false } } } } } as unknown as AuthContext;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources"); await router.isReady();
 const wrapper = mount(CatalogDetails, { attachTo: document.body, props: { skill }, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth } } });
 await flushPromises();
 expect([...document.body.querySelectorAll("button")].some((button) => button.textContent?.trim() === "编辑")).toBe(false);
 expect(document.body.textContent).toContain("去使用");
 wrapper.unmount();
});

it("edits a Team in its modal using the four Team settings", async () => {
 const {default: ExpertTeamSettings} = await import("./ExpertTeamSettings.vue");
 const team = {id: "team", name: "Team", introduction: "Review", core_capability: "Review", members: [], mutable: true, available: true, version: 2} as unknown as import("../api/client").ExpertTeam;
 const api = {listExperts: vi.fn(async () => [])} as unknown as PlatformApi;
 const auth = {session: {state: {value: {kind: "authenticated", currentUser: {administrator: true}}}}} as unknown as AuthContext;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources");
 const wrapper = mount(CatalogDetails, {attachTo: document.body, props: {team}, global: {plugins: [router, createAppI18n({getItem: () => "zh-CN"}, "zh-CN")], provide: {[platformApiKey as symbol]: api, [authContextKey as symbol]: auth}}}); await flushPromises();
 [...document.body.querySelectorAll<HTMLButtonElement>("button")].find(button => button.textContent?.trim() === "编辑")!.click(); await flushPromises();
 const settings = wrapper.getComponent(ExpertTeamSettings); expect(settings.props("team")).toEqual(team); expect(document.body.querySelector('input[aria-label="专家团名称"]')).not.toBeNull(); expect(document.body.querySelector('textarea[aria-label="团队描述"]')).not.toBeNull(); expect(wrapper.findComponent({name: "ExpertBindingsEditor"}).exists()).toBe(false);
 settings.vm.$emit("saved", team); await flushPromises(); expect(wrapper.emitted("saved")).toHaveLength(1); expect(wrapper.emitted("close")).toHaveLength(1); wrapper.unmount();
});

async function taskDetails(kind: "expert" | "team", locale = "zh-CN") {
 const profile = { id: "specialist", name: "Reviewer", introduction: "Review supplied evidence", starter_prompts: ["Review this proposal", "Check the evidence"], available: true };
 const api = { createSession: vi.fn(), getConversationSelection: vi.fn(), sendSessionMessage: vi.fn() } as unknown as PlatformApi;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources"); await router.isReady();
 const wrapper = mount(CatalogDetails, { attachTo: document.body, props: kind === "expert" ? { expert: profile as Expert } : { team: profile as ExpertTeam }, global: { plugins: [router, createAppI18n({ getItem: () => locale }, locale)], provide: { [platformApiKey as symbol]: api } } });
 await flushPromises();
 const prompts = () => [...document.body.querySelectorAll<HTMLButtonElement>(".expert-task-prompt")];
 return { wrapper, api, router, prompts };
}

it.each(["expert", "team"] as const)("opens a clicked %s task as an editable Session draft without submitting", async (kind) => {
 const { wrapper, api, router, prompts } = await taskDetails(kind);
 expect(document.body.querySelector(".expert-common-tasks ul")).toBeNull();
 expect(prompts()[1]!.textContent).toBe("“Check the evidence”");
 prompts()[1]!.click(); await flushPromises();
 expect(router.currentRoute.value.path).toBe("/sessions");
 expect(router.currentRoute.value.query[kind === "expert" ? "expert_id" : "expert_team_id"]).toBe("specialist");
 expect(router.currentRoute.value.query.new).toBeTruthy(); expect(router.currentRoute.value.query.draft).toBe("Check the evidence");
 expect(api.createSession).not.toHaveBeenCalled(); expect(api.getConversationSelection).not.toHaveBeenCalled(); expect(api.sendSessionMessage).not.toHaveBeenCalled();
 expect(wrapper.emitted("close")).toHaveLength(1); wrapper.unmount();
});

it("blocks duplicate task clicks while opening the conversation", async () => {
 const { wrapper, api, router, prompts } = await taskDetails("expert");
 let finish!: () => void;
 const push = vi.spyOn(router, "push").mockImplementationOnce(() => new Promise(resolve => { finish = () => resolve(); }));
 prompts()[0]!.click(); prompts()[1]!.click(); await flushPromises();
 expect(push).toHaveBeenCalledTimes(1); expect(prompts().every(button => button.disabled)).toBe(true);
 expect(document.body.querySelector('[role="status"]')?.textContent).toContain("正在打开会话");
 finish(); await flushPromises(); expect(api.sendSessionMessage).not.toHaveBeenCalled(); wrapper.unmount();
});

it("retains the task after navigation fails and lets the User retry without submitting", async () => {
 const { wrapper, api, router, prompts } = await taskDetails("expert");
 vi.spyOn(router, "push").mockRejectedValueOnce(new Error("navigation failed"));
 prompts()[0]!.click(); await flushPromises();
 expect(wrapper.emitted("close")).toBeUndefined(); expect(document.body.textContent).toContain("未能打开会话");
 expect(prompts()[0]!.disabled).toBe(false); prompts()[0]!.click(); await flushPromises();
 expect(router.currentRoute.value.query.draft).toBe("Review this proposal"); expect(api.sendSessionMessage).not.toHaveBeenCalled(); wrapper.unmount();
});

it("retains the dialog if a navigation guard cancels the launch", async () => {
 const { wrapper, api, router, prompts } = await taskDetails("team");
 const removeGuard = router.beforeEach(() => false);
 prompts()[0]!.click(); await flushPromises();
 expect(router.currentRoute.value.path).toBe("/resources"); expect(wrapper.emitted("close")).toBeUndefined();
 expect(document.body.textContent).toContain("未能打开会话"); expect(api.sendSessionMessage).not.toHaveBeenCalled(); removeGuard(); wrapper.unmount();
});

it("keeps plain Summon as selection only and disables tasks for an unavailable specialist", async () => {
 const { wrapper, api, router, prompts } = await taskDetails("expert");
 [...document.body.querySelectorAll<HTMLButtonElement>("button")].find(button => button.textContent?.trim() === "召唤")!.click(); await flushPromises();
 expect(router.currentRoute.value.query.expert_id).toBe("specialist"); expect(router.currentRoute.value.query.draft).toBeUndefined(); expect(api.createSession).not.toHaveBeenCalled();
 await wrapper.setProps({ expert: { ...wrapper.props("expert")!, available: false } });
 expect(prompts().every(button => button.disabled)).toBe(true); prompts()[0]!.click(); await flushPromises();
 expect(api.sendSessionMessage).not.toHaveBeenCalled(); wrapper.unmount();
});

it("provides a localized draft accessible name in English", async () => {
 const { wrapper, prompts } = await taskDetails("expert", "en");
 expect(prompts()[0]!.getAttribute("aria-label")).toBe("Draft with Reviewer: Review this proposal"); wrapper.unmount();
});
