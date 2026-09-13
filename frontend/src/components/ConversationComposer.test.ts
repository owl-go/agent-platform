// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type CLIConnectorDefinition, type ConversationScope, type PlatformApi, type Skill } from "../api/client";
import type { CLIAuthorizationRequest } from "../cliAuthorization";
import { authContextKey } from "../auth/session";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import { conversationApiStub, emptySelection } from "../test/conversation";
import ConversationComposer from "./ConversationComposer.vue";

const skill = { id: "pdf", name: "PDF 文档处理" } as Skill;
async function setup(options: { fail?: boolean; initial?: boolean; session?: string; scope?: ConversationScope; owner?: string; authorization?: boolean; authorizationRequest?: CLIAuthorizationRequest } = {}) {
 const initial = { ...emptySelection(), name: "Reviewer", expert_id: "expert-1", mcp_servers: [{ id: "mcp-1", name: "Search", revision: "1" }], cli_connectors: options.authorization ? [{ id: "feishu", name: "飞书 CLI", revision: "3" }] : [] };
 const definition = { id: "feishu", name: "飞书 CLI", state: "available", authentication_driver: "feishu", capabilities: [{ id: "send", argv_prefix: ["im", "+messages-send"], risk: "high", identities: ["user"], scopes: ["im:message", "im:message.send_as_user"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 60 }] } as CLIConnectorDefinition;
 const api = { ...conversationApiStub(initial), listExperts: vi.fn(async () => []), listExpertTeams: vi.fn(async () => []), listSkills: vi.fn(async () => [skill]), ...(options.authorization ? {
  listCLIConnectorDefinitions: vi.fn(async () => [definition]),
  listCLIConnectorEnablements: vi.fn(async () => [{ id: "enable-1", definition_id: definition.id, state: "enabled", version: 1 }]),
  listCLIConnectorAuthorizations: vi.fn(async () => []),
  beginCLIConnectorAuthorization: vi.fn(async () => ({ id: "flow-1", enablement_id: "enable-1", identity: "user", scopes: ["im:message", "im:message.send_as_user"], state: "waiting_for_user", action_url: "https://accounts.feishu.cn/authorize" })),
  completeCLIConnectorAuthorization: vi.fn(async () => ({ id: "flow-1", enablement_id: "enable-1", identity: "user", scopes: ["im:message", "im:message.send_as_user"], state: "waiting_for_user", action_url: "https://accounts.feishu.cn/authorize" })),
 } : {}), uploadAttachment: vi.fn(async (file: File) => ({ id: `attachment-${file.name}`, name: file.name, content_type: file.type, size: file.size, sha256: "sha256", image: file.type.startsWith("image/") })) } as unknown as PlatformApi;
 const submit = options.fail ? vi.fn(async () => { throw new Error("offline"); }) : vi.fn(async () => {});
 const router = createAppRouter(createMemoryHistory()); await router.push("/sessions"); await router.isReady();
 const wrapper = mount(ConversationComposer, { attachTo: document.body, props: { scope: options.scope ?? { session_id: options.session ?? "session-1" }, submit, initialSkillId: options.initial ? "pdf" : undefined, authorizationRequest: options.authorizationRequest }, global: {
  plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
  provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: { session: { state: ref({ kind: "authenticated", currentUser: { id: options.owner ?? "owner-1" } }) } } } },
 }); await flushPromises(); return { wrapper, api, submit };
}
afterEach(() => { localStorage.clear(); document.body.replaceChildren(); vi.restoreAllMocks(); });

describe("ConversationComposer", () => {
 it("replaces action icons with a single loading icon while sending or stopping", async () => {
  const { wrapper, submit } = await setup({ initial: true });
  const editor = wrapper.get<HTMLElement>(".composer-editor"); editor.element.append(document.createTextNode("创建 PDF")); await editor.trigger("input");
  let finish: (() => void) | undefined;
  submit.mockImplementation(() => new Promise<void>(resolve => { finish = resolve; }));
  const send = wrapper.get('[aria-label="发送"]');
  expect(send.findAll("svg")).toHaveLength(1);
  await send.trigger("click"); await flushPromises();
  expect(send.find(".is-loading").exists()).toBe(true); expect(send.findAll("svg")).toHaveLength(1);
  finish?.(); await flushPromises();
  await wrapper.setProps({ active: true });
  const stop = wrapper.get('[aria-label="中止生成"]');
  expect(stop.findAll("svg")).toHaveLength(1);
  await stop.trigger("click"); expect(wrapper.emitted("stop")).toHaveLength(1);
  await wrapper.setProps({ stopping: true });
  expect(stop.find(".is-loading").exists()).toBe(true); expect(stop.findAll("svg")).toHaveLength(1);
  wrapper.unmount();
 });
 it("preselects a launched Skill without sending and clears only one-turn input on accepted send", async () => {
  const { wrapper, submit } = await setup({ initial: true });
  expect(wrapper.get(".composer-token").text()).toContain(skill.name); expect(submit).not.toHaveBeenCalled();
  expect(wrapper.emitted("launchConsumed")).toHaveLength(1);
  const editor = wrapper.get<HTMLElement>(".composer-editor"); editor.element.append(document.createTextNode("创建 PDF")); await editor.trigger("input");
  await wrapper.get('[aria-label="发送"]').trigger("click"); await flushPromises();
  expect(submit).toHaveBeenCalledWith(expect.objectContaining({ content: "创建 PDF", input: { selection_id: "selection-2", file_references: [] } }));
  expect(wrapper.find(".composer-token").exists()).toBe(false);
  expect(wrapper.get(".composer-specialist").text()).toContain("Reviewer"); expect(wrapper.find('[aria-label="Search"]').exists()).toBe(true);
  wrapper.unmount();
  const restored = await setup(); expect(restored.wrapper.find(".composer-token").exists()).toBe(false); restored.wrapper.unmount();
 });
 it("keeps the exact Skill and draft after submit failure and restores it after reload", async () => {
  const { wrapper, submit } = await setup({ initial: true, fail: true });
  const editor = wrapper.get<HTMLElement>(".composer-editor"); editor.element.append(document.createTextNode("创建 PDF")); await editor.trigger("input");
  await wrapper.get('[aria-label="发送"]').trigger("click"); await flushPromises();
  expect(submit).toHaveBeenCalledOnce(); expect(wrapper.get(".composer-token").text()).toContain(skill.name); expect(wrapper.find('[role="alert"]').exists()).toBe(true);
  wrapper.unmount(); const restored = await setup(); expect(restored.wrapper.get(".composer-token").text()).toContain(skill.name); restored.wrapper.unmount();
 });
 it("removing a token removes its selection before sending and does not remove the Expert", async () => {
  const { wrapper, api } = await setup({ initial: true });
  await wrapper.get(".composer-token button").trigger("click"); await flushPromises();
  expect(api.resolveConversationSelection).toHaveBeenLastCalledWith({ session_id: "session-1" }, expect.objectContaining({ skill_ids: [], mcp_server_ids: ["mcp-1"] }));
  expect(wrapper.get(".composer-specialist").text()).toContain("Reviewer"); wrapper.unmount();
 });
 it("uses slash keyboard selection without sending, including during IME composition", async () => {
  const { wrapper, submit } = await setup(); const editor = wrapper.get<HTMLElement>(".composer-editor");
  editor.element.textContent = "/PDF"; const range = document.createRange(); range.selectNodeContents(editor.element); range.collapse(false); window.getSelection()?.removeAllRanges();
  range.setStart(editor.element.firstChild!, 4); range.collapse(true); window.getSelection()?.addRange(range);
  await editor.trigger("input"); await flushPromises(); expect(wrapper.get(".composer-options").text()).toContain(skill.name);
  await editor.trigger("keydown", { key: "Enter", isComposing: true }); expect(wrapper.find(".composer-token").exists()).toBe(false);
  await editor.trigger("keydown", { key: "Enter" }); await flushPromises(); expect(wrapper.get(".composer-token").text()).toContain(skill.name); expect(editor.text()).not.toContain("/PDF"); expect(submit).not.toHaveBeenCalled(); wrapper.unmount();
 });
 it("keeps drafts isolated by user and conversation", async () => {
  const first = await setup({ initial: true }); first.wrapper.unmount();
  const second = await setup({ session: "session-2" }); expect(second.wrapper.find(".composer-token").exists()).toBe(false); second.wrapper.unmount();
  const otherUser = await setup({ owner: "owner-2" }); expect(otherUser.wrapper.find(".composer-token").exists()).toBe(false); otherUser.wrapper.unmount();
 });
 it("does not submit when Enter is pressed in the Expert picker", async () => {
  const { wrapper, submit } = await setup({ initial: true }); await wrapper.get(".composer-specialist").trigger("click");
  await wrapper.get(".composer-menu-search input").trigger("keydown", { key: "Enter" }); await flushPromises(); expect(submit).not.toHaveBeenCalled(); wrapper.unmount();
 });
 it.each([
  ["Session", { session_id: "session-1" }],
  ["Workflow Run Conversation", { workflow_id: "workflow-1", run_id: "run-1" }],
 ])("pastes clipboard images and files into the %s composer", async (_label, scope) => {
  const { wrapper, api, submit } = await setup({ scope });
  const image = new File(["image"], "diagram.png", { type: "image/png" });
  const documentFile = new File(["notes"], "notes.txt", { type: "text/plain" });
  const paste = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(paste, "clipboardData", { value: { files: [image, documentFile], items: [], getData: () => "" } });

  wrapper.get(".composer-editor").element.dispatchEvent(paste);
  await flushPromises();

  expect(paste.defaultPrevented).toBe(true);
  expect(wrapper.findAll(".pending-attachments > span").map((item) => item.text())).toEqual(expect.arrayContaining([expect.stringContaining("diagram.png"), expect.stringContaining("notes.txt")]));

  await wrapper.get('[aria-label="发送"]').trigger("click");
  await flushPromises();

  expect(api.uploadAttachment).toHaveBeenNthCalledWith(1, image);
  expect(api.uploadAttachment).toHaveBeenNthCalledWith(2, documentFile);
  expect(submit).toHaveBeenCalledWith({ content: "", attachmentIDs: ["attachment-diagram.png", "attachment-notes.txt"], input: { selection_id: "selection-1", file_references: [] } });
  wrapper.unmount();
 });
 it("waits for an attempted operation before showing Feishu authorization and keeps a direct recovery link", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true });
  expect(wrapper.find(".composer-authorization").exists()).toBe(false);
  expect(api.listCLIConnectorAuthorizations).not.toHaveBeenCalled();
  await wrapper.setProps({ authorizationRequest: { connectorID: "feishu", capabilityID: "send" } }); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("飞书 CLI 需要飞书账号授权");
  expect(wrapper.get(".composer-authorization").text()).toContain("回复“已授权”");
  await wrapper.get(".composer-authorization button").trigger("click"); await flushPromises();
  expect(api.beginCLIConnectorAuthorization).toHaveBeenCalledWith("enable-1", "user", ["im:message", "im:message.send_as_user"]);
  expect(wrapper.get(".composer-authorization a").attributes("href")).toBe("https://accounts.feishu.cn/authorize");
  vi.mocked(api.completeCLIConnectorAuthorization).mockResolvedValue({ id: "flow-1", enablement_id: "enable-1", identity: "user", scopes: ["im:message", "im:message.send_as_user"], state: "completed" });
  document.dispatchEvent(new Event("visibilitychange")); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("飞书授权已完成");
  expect(wrapper.get(".composer-authorization").text()).toContain("回复“已授权”");
  wrapper.unmount();
 });
});

it("keeps the editor editable while removed Skill selections synchronize", async () => {
 const { wrapper, api } = await setup({ initial: true });
 let finish: ((value: ReturnType<typeof emptySelection>) => void) | undefined;
 api.resolveConversationSelection = vi.fn(() => new Promise<ReturnType<typeof emptySelection>>(resolve => { finish = resolve; }));
 const editor = wrapper.get<HTMLElement>(".composer-editor"); editor.element.focus(); editor.element.textContent = "@report";
 await editor.trigger("input");
 expect(editor.attributes("contenteditable")).toBe("true"); expect(wrapper.get('[aria-label="发送"]').attributes("disabled")).toBeDefined();
 finish?.(emptySelection()); await flushPromises(); expect(editor.element.textContent).toBe("@report"); wrapper.unmount();
});
