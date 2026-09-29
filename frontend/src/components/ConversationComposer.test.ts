// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { ApiError, platformApiKey, type CLIConnectorDefinition, type ConversationScope, type PlatformApi, type Skill } from "../api/client";
import { conversationDraftKey, saveConversationDraft } from "../conversationDraft";
import type { CLIAuthorizationRequest } from "../cliAuthorization";
import { authContextKey } from "../auth/session";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import { conversationApiStub, emptySelection } from "../test/conversation";
import ConversationComposer from "./ConversationComposer.vue";

const skill = { id: "pdf", name: "PDF 文档处理" } as Skill;
async function setup(options: { fail?: boolean; initial?: boolean; session?: string; scope?: ConversationScope; owner?: string; authorization?: boolean; activation?: boolean; managed?: boolean; managedAuthorized?: boolean; managedRefreshable?: boolean; managedSetupComplete?: boolean; installationVersion?: number; dingtalk?: boolean; noScopes?: boolean; taskCapability?: boolean; documentCapability?: boolean; authorizationRequest?: CLIAuthorizationRequest } = {}) {
 const connectorName = options.dingtalk ? "钉钉" : "飞书 CLI";
 const initial = { ...emptySelection(), name: "Reviewer", expert_id: "expert-1", mcp_servers: [{ id: "mcp-1", name: "Search", revision: "1" }], cli_connectors: options.authorization ? [{ id: options.managed ? "installation-1" : "feishu", name: connectorName, revision: "3" }] : [] };
 const definition = { id: options.managed ? "installation-1" : "feishu", name: options.managed ? connectorName : "飞书 CLI", state: "available", authentication_driver: options.dingtalk ? "dingtalk" : "feishu", managed_installation: options.managed, managed_authorized: options.managedAuthorized, capabilities: options.noScopes ? [] : [{ id: "send", argv_prefix: ["im", "+messages-send"], risk: "high", identities: ["user"], scopes: options.dingtalk ? [] : ["im:message", "im:message.send_as_user"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 60 }, ...(options.taskCapability ? [{ id: "task_create", argv_prefix: ["task", "+create"], risk: "high", identities: ["user"], scopes: ["task:task:write"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 60 }] : []), ...(options.documentCapability ? [{ id: "docs_create", argv_prefix: ["docs", "+create"], risk: "high", identities: ["user"], scopes: ["docx:document:create", "docx:document:write_only"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 120 }, { id: "mail_send", argv_prefix: ["mail", "+send"], risk: "high", identities: ["user"], scopes: ["mail:mail:write"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 120 }] : [])] } as CLIConnectorDefinition;
 const enabled = { id: "enable-1", definition_id: definition.id, state: "enabled" as const, version: 1 };
 let refreshed = false;
 const api = { ...conversationApiStub(initial), listExperts: vi.fn(async () => []), listExpertTeams: vi.fn(async () => []), listSkills: vi.fn(async () => [skill]), ...((options.authorization || options.activation || options.managed) ? {
  listCLIConnectorDefinitions: vi.fn(async () => [definition]),
  listCLIConnectorEnablements: vi.fn(async () => options.managed ? [] : options.activation ? [{ ...enabled, state: "disabled" as const }] : [enabled]),
  enableCLIConnector: vi.fn(async () => { if (options.managed) throw new ApiError("not_found", 404, "not_found"); return enabled; }),
  listConnectorInstallations: vi.fn(async () => options.managed ? [{ id: "installation-1", source: options.dingtalk ? "dingtalk" : "feishu", active_revision_id: "revision-1", state: "active" as const, authorized: Boolean(options.managedAuthorized || refreshed), version: options.installationVersion ?? 1, package_version: "1.0.93", name: options.dingtalk ? "钉钉" : "飞书", description: "", authentication_driver: options.dingtalk ? "dingtalk" : "feishu", upgrade_available: false }] : []),
  listConnectorPublications: vi.fn(async () => options.managed ? [{ source: options.dingtalk ? "dingtalk" : "feishu", active_revision_id: "revision-1", state: "available" as const, version: 1, revision: { id: "revision-1", source: options.dingtalk ? "dingtalk" : "feishu", package_version: "1.0.93", mode: "cli" as const, sha256: "a".repeat(64), name: options.dingtalk ? "钉钉" : "飞书", description: "", icon: "plug", runtime_digests: [], conformance_available: true, authentication_driver: options.dingtalk ? "dingtalk" : "feishu", required_scopes: options.dingtalk ? [] : ["im:message", "im:message.send_as_user"] } }] : []),
  listConnectorAuthorizations: vi.fn(async () => options.managedAuthorized || options.managedRefreshable ? [{ id: "authorization-1", installation_id: "installation-1", identity_ref: "user", external_identity_id: "ou_test", external_display_name: "Tester", scopes: ["im:message", "im:message.send_as_user"], state: options.managedRefreshable ? "expired" as const : "active" as const, version: 1, selected: true }] : []),
  refreshConnectorAuthorization: vi.fn(async () => { refreshed = true; return { id: "authorization-1", installation_id: "installation-1", identity_ref: "user", external_identity_id: "ou_test", external_display_name: "Tester", scopes: ["im:message", "im:message.send_as_user"], state: "active" as const, version: 2, selected: true }; }),
  disableConnectorInstallation: vi.fn(async () => ({ id: "installation-1", source: "feishu", active_revision_id: "revision-1", state: "disabled" as const, authorized: Boolean(options.managedAuthorized), version: 2, package_version: "1.0.93", name: "飞书", description: "", authentication_driver: "feishu", upgrade_available: false })),
  beginConnectorSetup: vi.fn(async () => ({ id: "setup-1", installation_id: "installation-1", state: options.managedSetupComplete ? "completed" as const : "waiting_for_user" as const, action_url: "https://open.feishu.cn/app" })),
  completeConnectorSetup: vi.fn(async () => ({ id: "setup-1", installation_id: "installation-1", state: "waiting_for_user" as const, action_url: "https://open.feishu.cn/app" })),
  beginConnectorAuthorizationFlow: vi.fn(async () => ({ id: "authorization-1", installation_id: "installation-1", identity: "user" as const, scopes: options.dingtalk ? [] : ["im:message", "im:message.send_as_user"], state: "waiting_for_user" as const, action_url: options.dingtalk ? "https://login.dingtalk.com/oauth2/auth" : "https://accounts.feishu.cn/authorize" })),
  completeConnectorAuthorizationFlow: vi.fn(async () => ({ id: "authorization-1", installation_id: "installation-1", identity: "user" as const, scopes: options.dingtalk ? [] : ["im:message", "im:message.send_as_user"], state: "waiting_for_user" as const, action_url: options.dingtalk ? "https://login.dingtalk.com/oauth2/auth" : "https://accounts.feishu.cn/authorize" })),
  disableCLIConnector: vi.fn(async () => ({ ...enabled, state: "disabled" as const, version: 2 })),
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
 it("sends an explicit Plan preference only when Plan first is selected", async () => {
  const { wrapper, submit } = await setup();
  await wrapper.get(".composer-plan-toggle").trigger("click");
  expect(wrapper.get(".composer-plan-toggle").attributes("aria-pressed")).toBe("true");
  const editor = wrapper.get<HTMLElement>(".composer-editor"); editor.element.textContent = "先列计划"; await editor.trigger("input");
  await wrapper.get('[aria-label="发送"]').trigger("click"); await flushPromises();
  expect(submit).toHaveBeenCalledWith(expect.objectContaining({ input: expect.objectContaining({ plan_preference: "always" }) }));
  wrapper.unmount();
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
 it("reconciles an expired saved Connector selection with the server while keeping draft text", async () => {
  const scope = { session_id: "session-1" };
  saveConversationDraft(conversationDraftKey("owner-1", scope), {
   parts: [{ kind: "text", text: "继续" }],
   selection: { ...emptySelection(), id: "saved-selection", cli_connectors: [{ id: "installation-1", name: "飞书", icon: "feishu", revision: "1" }] },
   attachments: [], pendingFileNames: [],
  });
  const { wrapper, api } = await setup({ managed: true, managedAuthorized: false });
  expect(api.getConversationSelection).toHaveBeenCalledWith(scope);
  expect(wrapper.find('[aria-label="飞书"]').exists()).toBe(false);
  expect(wrapper.get(".composer-editor").text()).toBe("继续");
  expect(wrapper.get('[role="alert"]').text()).toContain("连接器选择已失效");
  wrapper.unmount();
 });
 it("refreshes an upgraded managed Connector before restoring a saved draft selection", async () => {
  const scope = { session_id: "session-1" };
  saveConversationDraft(conversationDraftKey("owner-1", scope), {
   parts: [{ kind: "text", text: "继续" }],
   selection: { ...emptySelection(), id: "saved-selection", cli_connectors: [{ id: "installation-1", name: "钉钉", icon: "dingtalk", revision: "2" }] },
   attachments: [], pendingFileNames: [],
  });
  const { wrapper, api } = await setup({ authorization: true, managed: true, managedAuthorized: true, installationVersion: 4, dingtalk: true });
  expect(api.resolveConversationSelection).toHaveBeenCalledWith(scope, expect.objectContaining({
   previous_id: "saved-selection",
   cli_connector_ids: ["installation-1"],
   refresh_ids: ["cli:installation-1"],
  }));
  expect(wrapper.get(".composer-editor").text()).toBe("继续");
  wrapper.unmount();
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
  expect(api.beginCLIConnectorAuthorization).not.toHaveBeenCalled();
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
 it("opens the managed Feishu authorization link from a failed conversation command", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true, managed: true, managedSetupComplete: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "send" } }); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("飞书 CLI 需要飞书账号授权");
  await wrapper.get(".composer-authorization button").trigger("click"); await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", ["im:message", "im:message.send_as_user"]);
  expect(wrapper.get(".composer-authorization a").attributes("href")).toBe("https://accounts.feishu.cn/authorize");
  vi.mocked(api.completeConnectorAuthorizationFlow).mockResolvedValue({ id: "authorization-1", installation_id: "installation-1", identity: "user", scopes: ["im:message", "im:message.send_as_user"], state: "completed" });
  document.dispatchEvent(new Event("visibilitychange")); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("飞书授权已完成");
  wrapper.unmount();
 });
 it("does not ask to reauthorize a managed Feishu Connector with the required scopes", async () => {
  const { wrapper, api } = await setup({ authorization: true, managed: true, managedAuthorized: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "send" } }); await flushPromises();
  expect(wrapper.find(".composer-authorization").exists()).toBe(false);
  expect(api.beginConnectorAuthorizationFlow).not.toHaveBeenCalled();
  wrapper.unmount();
 });
 it("preserves existing message scopes when requesting task creation authorization", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true, managed: true, managedAuthorized: true, managedSetupComplete: true, taskCapability: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "task_create" } }); await flushPromises();
  await wrapper.get(".composer-authorization button").trigger("click"); await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", ["im:message", "im:message.send_as_user", "task:task:write"]);
  wrapper.unmount();
 });
 it("requests cloud-document scopes from the conversation without requesting unrelated domains", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true, managed: true, managedAuthorized: true, managedSetupComplete: true, documentCapability: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "docs_create" } }); await flushPromises();
  await wrapper.get(".composer-authorization button").trigger("click"); await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", ["im:message", "im:message.send_as_user", "docx:document:create", "docx:document:write_only"]);
  wrapper.unmount();
 });
 it("recovers a managed Feishu grant that expired before the command started", async () => {
  const { wrapper, api } = await setup({ authorization: true, managed: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "" } }); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("需要飞书账号授权");
  expect(api.listConnectorAuthorizations).toHaveBeenCalledWith("installation-1");
  wrapper.unmount();
 });
 it("opens DingTalk authorization from the conversation when its grant is unavailable", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true, managed: true, dingtalk: true });
  await wrapper.setProps({ authorizationRequest: { connectorID: "installation-1", capabilityID: "" } }); await flushPromises();
  expect(wrapper.get(".composer-authorization").text()).toContain("钉钉 需要钉钉账号授权");
  await wrapper.get(".composer-authorization button").trigger("click"); await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", []);
  expect(wrapper.get(".composer-authorization a").attributes("href")).toBe("https://login.dingtalk.com/oauth2/auth");
  wrapper.unmount();
 });
 it("activates, authorizes, selects, and deactivates a CLI Connector from the conversation picker", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ activation: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await flushPromises();
  const option = wrapper.get(".composer-connector-option");
  expect(option.text()).toContain("打开右侧开关即可激活");
  await option.get(".el-switch").trigger("click");
  await flushPromises();
  expect(api.enableCLIConnector).toHaveBeenCalledWith("feishu");
  expect(api.beginCLIConnectorAuthorization).toHaveBeenCalledWith("enable-1", "user", ["im:message", "im:message.send_as_user"]);
  expect(api.resolveConversationSelection).toHaveBeenLastCalledWith({ session_id: "session-1" }, expect.objectContaining({ cli_connector_ids: ["feishu"] }));
  expect(wrapper.find('.composer-authorization a[href="https://accounts.feishu.cn/authorize"]').exists()).toBe(true);
  await option.get(".el-switch").trigger("click");
  await flushPromises();
  expect(api.disableCLIConnector).toHaveBeenCalledWith("feishu", 1);
  expect(api.resolveConversationSelection).toHaveBeenLastCalledWith({ session_id: "session-1" }, expect.objectContaining({ cli_connector_ids: [] }));
  wrapper.unmount();
 });
 it("offers activation in a conversation when the saved Feishu application has no authorization", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ authorization: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await flushPromises();
  const option = wrapper.get(".composer-connector-option");
  expect(option.text()).toContain("打开右侧开关即可激活");
  expect(option.get(".el-switch").classes()).not.toContain("is-checked");
  await option.get(".el-switch").trigger("click");
  await flushPromises();
  expect(api.enableCLIConnector).toHaveBeenCalledWith("feishu");
  expect(api.beginCLIConnectorAuthorization).toHaveBeenCalledWith("enable-1", "user", ["im:message", "im:message.send_as_user"]);
  expect(option.get(".el-switch").classes()).toContain("is-checked");
  wrapper.unmount();
 });
 it("starts Feishu account authorization even when the connector has no requested scopes", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ activation: true, noScopes: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.beginCLIConnectorAuthorization).toHaveBeenCalledWith("enable-1", "user", []);
  expect(wrapper.find('.composer-authorization a[href="https://accounts.feishu.cn/authorize"]').exists()).toBe(true);
  wrapper.unmount();
 });
 it("opens setup for an installed Feishu package from the conversation switch", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.beginConnectorSetup).toHaveBeenCalledWith("installation-1");
  expect(api.enableCLIConnector).not.toHaveBeenCalled();
  expect(wrapper.find('[role="alert"]').exists()).toBe(false);
  wrapper.unmount();
 });
 it("starts DingTalk account authorization directly from the conversation switch", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true, dingtalk: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.beginConnectorSetup).not.toHaveBeenCalled();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", []);
  wrapper.unmount();
 });
 it("selects an authorized Feishu package and deactivates it from the conversation switch", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true, managedAuthorized: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  const option = wrapper.get(".composer-connector-option");
  expect(option.get(".el-switch").classes()).toContain("is-checked");
  await option.get("button").trigger("click");
  await flushPromises();
  expect(api.resolveConversationSelection).toHaveBeenLastCalledWith({ session_id: "session-1" }, expect.objectContaining({ cli_connector_ids: ["installation-1"] }));
  await option.get(".el-switch").trigger("click");
  await flushPromises();
  expect(api.disableConnectorInstallation).toHaveBeenCalledWith("installation-1", 1);
  expect(api.disableCLIConnector).not.toHaveBeenCalled();
  wrapper.unmount();
 });
 it("refreshes an expired Feishu grant when the user activates the Connector", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true, managedRefreshable: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.refreshConnectorAuthorization).toHaveBeenCalledWith("installation-1", "authorization-1", 1);
  expect(api.beginConnectorSetup).not.toHaveBeenCalled();
  expect(api.resolveConversationSelection).toHaveBeenLastCalledWith({ session_id: "session-1" }, expect.objectContaining({ cli_connector_ids: ["installation-1"] }));
  wrapper.unmount();
 });
 it("goes straight to account authorization when a managed Feishu application is already set up", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true, managedSetupComplete: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", ["im:message", "im:message.send_as_user"]);
  expect(wrapper.find('.composer-authorization a[href="https://accounts.feishu.cn/authorize"]').exists()).toBe(true);
  wrapper.unmount();
 });
 it("activates managed Feishu with document scopes while leaving unrelated domains on demand", async () => {
  vi.spyOn(window, "open").mockReturnValue(null);
  const { wrapper, api } = await setup({ managed: true, managedSetupComplete: true, documentCapability: true });
  await wrapper.get(".composer-plus").trigger("click");
  await wrapper.findAll(".composer-menu button").find((button) => button.text() === "连接器")!.trigger("click");
  await wrapper.get(".composer-connector-option .el-switch").trigger("click");
  await flushPromises();
  expect(api.beginConnectorAuthorizationFlow).toHaveBeenCalledWith("installation-1", "user", ["im:message", "im:message.send_as_user", "docx:document:create", "docx:document:write_only"]);
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
