// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { ApiError, platformApiKey, type Artifact, type Expert, type ModelProviderConnection, type PersonalSettings, type PlatformApi, type Session, type SessionMessage, type SessionMessageSnapshot, type SessionWorkflowCreation } from "../api/client";
import { createAppI18n } from "../i18n";
import { conversationApiStub } from "../test/conversation";
import { createAppRouter } from "../router";
import ConversationComposer from "../components/ConversationComposer.vue";
import { embeddedCommandApproval } from "../commandApprovalPlacement";
import SessionsPage from "./SessionsPage.vue";

const session: Session = {
  id: "session-1",
  title: "布局验收",
  archived: false,
  created_at: "2026-08-25T12:00:00Z",
  updated_at: "2026-08-25T12:01:00Z",
  version: 1,
};
const messages: SessionMessage[] = [
  { id: 1, role: "user", state: "succeeded", content: "我的消息", elapsed_ms: 0, created_at: "2026-08-25T12:00:00Z" },
  { id: 2, role: "assistant", state: "succeeded", content: "Agent 的消息", elapsed_ms: 1200, created_at: "2026-08-25T12:00:01Z" },
];

function apiStub(sessionMessages: SessionMessage[] = messages, stream?: (snapshot: (value: SessionMessageSnapshot) => void) => Promise<void>): PlatformApi {
  return {
    ...conversationApiStub(),
    listSessions: vi.fn(async (archived = false) => archived ? [] : [session]),
    listSessionMessages: vi.fn(async () => sessionMessages),
    listSessionWorkflowLinks: vi.fn(async () => []),
    streamSessionMessage: vi.fn(async (_sessionID, _messageID, onSnapshot) => stream?.(onSnapshot)),
    listExperts: vi.fn(async () => []),
    listExpertTeams: vi.fn(async () => []),
    listModelProviderConnections: vi.fn(async () => [{ id: "connection-1", name: "Provider", provider_type: "openai", endpoint: "https://model.invalid", protocols: ["openai_responses"], api_key_configured: true, verification_status: "verified", custom_endpoint: true, models: [{ id: "model-1", connection_id: "connection-1", model_id: "model", display_name: "Model", available: true, manually_added: false, compatibility: [{ runtime_engine: "codex", status: "verified" }] }], created_at: session.created_at, updated_at: session.updated_at, version: 1 }]),
    listRuntimeEngines: vi.fn(async () => [{ name: "codex", available: true, native_resume: true, cli_version: "1.0.0" }]),
    getSettings: vi.fn(async () => ({ personality: "direct_efficient", personality_instructions: "", runtime_model_defaults: [{ runtime_engine: "codex", provider_model_id: "model-1" }], default_runtime_engine: "codex", language: "zh-CN", timezone: "Asia/Shanghai", version: 1 })),
  } as unknown as PlatformApi;
}

async function mountPage(sessionMessages: SessionMessage[] = messages, stream?: (snapshot: (value: SessionMessageSnapshot) => void) => Promise<void>) {
  return mountPageWithAPI(apiStub(sessionMessages, stream));
}

async function mountPageWithAPI(api: PlatformApi, path = "/sessions") {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path);
  await router.isReady();
  const wrapper = mount(SessionsPage, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api },
    },
  });
  await flushPromises();
  return wrapper;
}


describe("SessionsPage conversation layout", () => {
  beforeEach(() => {
    window.localStorage.clear();
    Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: vi.fn(function (this: HTMLElement, options?: ScrollToOptions | number) {
      if (typeof options === "object") this.scrollTop = options.top ?? this.scrollTop;
    }) });
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:attachment-preview") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  });
  afterEach(() => {
    vi.useRealTimers();
    embeddedCommandApproval.value = undefined;
    delete (HTMLElement.prototype as { scrollTo?: unknown }).scrollTo;
    delete (URL as { createObjectURL?: unknown }).createObjectURL;
    delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
    vi.restoreAllMocks();
  });

it("passes connector guidance into a new Session composer without posting a message", async () => {
  const api = apiStub([]);
  const created = { ...session, id: "new-session" };
  api.createSession = vi.fn(async () => created);
  api.getAttachmentDownload = vi.fn(async () => new Blob());
  api.listMCPServers = vi.fn(async () => [{ id: "mcp-1", name: "Example", tested: true, test_pending: false, arguments: [], environment: [], transport: "streamable_http" as const, ...{ created_at: session.created_at, updated_at: session.updated_at, version: 1 } }]);
  api.sendSessionMessage = vi.fn();
  const wrapper = await mountPageWithAPI(api, "/sessions?new=launch&connector_kind=mcp&connector_id=mcp-1&draft=查询示例数据");
  expect(api.createSession).toHaveBeenCalled();
  expect(wrapper.get(".composer-editor").text()).toBe("查询示例数据");
  expect(api.resolveConversationSelection).toHaveBeenCalledWith({ session_id: created.id }, expect.objectContaining({ mcp_server_ids: ["mcp-1"] }));
  expect(api.sendSessionMessage).not.toHaveBeenCalled();
  wrapper.unmount();
});

  it("blocks a new task when an inherited execution pair is no longer verified", async () => {
    const api = apiStub([]);
    api.getSettings = vi.fn(async (): Promise<PersonalSettings> => ({ personality: "direct_efficient", personality_instructions: "", runtime_model_defaults: [{ runtime_engine: "codex", provider_model_id: "model-1" }], default_runtime_engine: "codex", language: "zh-CN", timezone: "Asia/Shanghai", version: 1, execution_inherited: true, platform_execution_available: true }));
    api.listModelProviderConnections = vi.fn(async (): Promise<ModelProviderConnection[]> => [{ id: "connection-1", name: "Provider", provider_type: "openai", endpoint: "https://model.invalid", protocols: ["openai_responses"], api_key_configured: true, verification_status: "verified", custom_endpoint: true, models: [{ id: "model-1", connection_id: "connection-1", model_id: "model", display_name: "Model", available: true, manually_added: false, compatibility: [{ runtime_engine: "codex", status: "unverified" }] }], created_at: session.created_at, updated_at: session.updated_at, version: 1 }]);
    api.getAttachmentDownload = vi.fn(async () => new Blob());
    const wrapper = await mountPageWithAPI(api);
    expect(wrapper.text()).toContain("开始前完成 3 个步骤");
    expect(wrapper.get(".setup-guide").text()).toContain("模型供应商");
    wrapper.unmount();
  });

  it("keeps user and Agent messages in distinct role rows", async () => {
    const wrapper = await mountPage();
    expect(wrapper.get(".message.user .message-content").text()).toContain("我的消息");
    expect(wrapper.get(".message.assistant .message-content").text()).toContain("Agent 的消息");
    expect(wrapper.find(".message-avatar").exists()).toBe(false);
    expect(wrapper.find(".composer-layer").exists()).toBe(true);
    wrapper.unmount();
  });

  it("prefills a Workflow from a successful response and requires an explicit file destination", async () => {
    const successful = [
      { ...messages[0]!, attachments: [{ id: "attachment-1", name: "brief.pdf", content_type: "application/pdf", size: 2048, sha256: "digest", image: false }] },
      { ...messages[1]!, state: "completed", response_snapshot: { schema_version: 2, stages: [{ position: 1, runtime_engine: "codex", provider_model: { id: "model-1", connection_id: "connection-1", connection_version: 1, connection_name: "Provider", provider_type: "openai", model_id: "model", name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], compatibility: "verified" }, skills: [{ id: "skill-1", name: "Report", object_key: "skills/report.zip", sha256: "digest" }] }] } },
    ] as SessionMessage[];
    const api = apiStub(successful);
    api.previewSessionWorkflowDraft = vi.fn<PlatformApi["previewSessionWorkflowDraft"]>(async () => ({ suggested_name: "周报", suggested_goal: "生成本周周报", specialist_name: "默认执行配置", resources: [{ kind: "skill", id: "skill-1", name: "Report" }], files: [{ source_key: "attachment:attachment-1", kind: "attachment", name: "brief.pdf", size: 2048, available: true }] }));
    api.createWorkflowFromSession = vi.fn<PlatformApi["createWorkflowFromSession"]>(async () => ({
      workflow: { id: "workflow-1", name: "周报", goal: "生成本周周报", environment: [], api_credential_configured: false, deleted: false, created_at: session.created_at, updated_at: session.updated_at, version: 1 },
      validation_run: { id: "run-1", conversation_id: "run-1", turn_number: 1, workflow_id: "workflow-1", workflow_name: "周报", trigger: "session_conversion", state: "waiting_for_user", queued_at: session.updated_at, elapsed_ms: 0 },
      link: { session_id: session.id, message_id: 2, workflow_id: "workflow-1", workflow_name: "周报", validation_run_id: "run-1", created_at: session.updated_at },
      replayed: false,
    } satisfies SessionWorkflowCreation));
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get(".message.assistant .message-actions button").trigger("click");
    await flushPromises();

    expect(api.previewSessionWorkflowDraft).toHaveBeenCalledWith(session.id, 2);
    expect(wrapper.get<HTMLInputElement>(".workflow-save-dialog input").element.value).toBe("周报");
    expect(wrapper.get(".workflow-save-dialog").text()).toContain("Report");
    await wrapper.get(".workflow-save-dialog .modal-actions .el-button--primary").trigger("click");
    await flushPromises();
    expect(api.createWorkflowFromSession).toHaveBeenCalledWith(session.id, 2, {
      name: "周报",
      goal: "生成本周周报",
      files: [{ source_key: "attachment:attachment-1", destination: "workspace" }],
    });
    await vi.waitFor(() => expect(wrapper.vm.$route.path).toBe("/workflows/workflow-1"));
    expect(wrapper.vm.$route.query).toMatchObject({ open_run: "run-1", from_session: session.id });
    wrapper.unmount();
  });

  it("opens Workflow save when the preview omits empty resources and files", async () => {
    const api = apiStub([{ ...messages[0]! }, { ...messages[1]!, state: "completed" }]);
    api.previewSessionWorkflowDraft = vi.fn<PlatformApi["previewSessionWorkflowDraft"]>(async () => ({ suggested_name: "布局验收", suggested_goal: "我的消息", specialist_name: "默认执行配置" } as Awaited<ReturnType<PlatformApi["previewSessionWorkflowDraft"]>>));
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get(".message.assistant .message-actions button").trigger("click");
    await flushPromises();

    expect(wrapper.get<HTMLInputElement>(".workflow-save-dialog input").element.value).toBe("布局验收");
    expect(wrapper.get(".workflow-save-dialog").text()).toContain("本次未使用额外技能或连接器");
    wrapper.unmount();
  });

  it("shows task details only for a task-bearing response and reopens them from history", async () => {
    const taskMessages = [messages[0]!, {
      ...messages[1]!,
      execution_plan: {
        id: "plan-1", state: "completed", objective: "生成发布报告", created_at: "2026-09-28T08:00:00Z", version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", label: "检查变更", position: 1, state: "completed" }], resources: [], side_effects: [], reasons: [], estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      },
    }] as SessionMessage[];
    const wrapper = await mountPage(taskMessages);

    expect(wrapper.get(".task-workspace-panel").text()).toContain("生成发布报告");
    await wrapper.get(".task-workspace-panel > header button").trigger("click");
    expect(wrapper.find(".task-workspace-panel").exists()).toBe(false);
    expect(localStorage.getItem(`agent-workspace:task-panel:session:${session.id}`)).toBe("closed");
    wrapper.unmount();

    const restored = await mountPage(taskMessages);
    expect(restored.find(".task-workspace-panel").exists()).toBe(false);
    const reopen = restored.get('.conversation-head button[aria-label="打开任务面板"]');
    await reopen.trigger("click");
    expect(restored.get(".task-workspace-panel").text()).toContain("检查变更");
    expect(restored.find('.conversation-head button[aria-label="打开任务面板"]').exists()).toBe(false);
    restored.unmount();
  });

  it("shows the Skills frozen for a historical user message", async () => {
    const snapshotMessages = [
      { ...messages[0]!, content: "[PDF 文档处理] 生成 PDF" },
      {
        ...messages[1]!,
        response_snapshot: {
          stages: [{
            position: 1,
            runtime_engine: "codex",
            provider_model: { id: "model-1", connection_id: "connection-1", connection_version: 1, connection_name: "Provider", provider_type: "openai", model_id: "model", name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], compatibility: "verified" },
            skills: [{ id: "skill-pdf", name: "PDF 文档处理", object_key: "skills/pdf.zip", sha256: "digest" }],
          }],
        },
      },
    ] as unknown as SessionMessage[];

    const wrapper = await mountPage(snapshotMessages);

    expect(wrapper.get(".message.user .message-skill-badge").text()).toContain("PDF 文档处理");
    expect(wrapper.get(".message.user .message-content > p").text()).toBe("生成 PDF");
    wrapper.unmount();
  });

  it("omits decorative English labels from the Chinese Session view", async () => {
    const wrapper = await mountPage();

    expect(wrapper.text()).not.toContain("CONVERSATIONS");
    expect(wrapper.text()).not.toContain("AUTO RUNTIME");
    expect(wrapper.find(".collection-head .eyebrow").exists()).toBe(false);
    expect(wrapper.find(".conversation-head .el-tag").exists()).toBe(false);
    wrapper.unmount();
  });

  it("renders an uploaded image from authenticated attachment content", async () => {
    const api = apiStub([{ ...messages[0]!, attachments: [{ id: "attachment-1", name: "photo.jpeg", content_type: "image/jpeg", size: 5, sha256: "digest", image: true }] }]);
    api.getAttachmentDownload = vi.fn(async () => new Blob(["image"], { type: "image/jpeg" }));

    const wrapper = await mountPageWithAPI(api);
    await flushPromises();

    expect(api.getAttachmentDownload).toHaveBeenCalledWith("attachment-1");
    expect(wrapper.get<HTMLImageElement>('.turn-attachment img[alt="photo.jpeg"]').attributes("src")).toBe("blob:attachment-preview");
    wrapper.unmount();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:attachment-preview");
  });

  it("downloads an uploaded file through the authenticated attachment API", async () => {
    const api = apiStub([{ ...messages[0]!, attachments: [{ id: "attachment-2", name: "notes.txt", content_type: "text/plain", size: 5, sha256: "digest", image: false }] }]);
    api.getAttachmentDownload = vi.fn(async () => new Blob(["notes"], { type: "text/plain" }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get(".turn-attachment").trigger("click");
    await flushPromises();

    expect(api.getAttachmentDownload).toHaveBeenCalledWith("attachment-2");
    expect(click).toHaveBeenCalledOnce();
    wrapper.unmount();
  });

  it("shows a generated file under the Agent response and downloads it", async () => {
    const artifact: Artifact = { id: "artifact-1", message_id: 2, run_id: "", kind: "file", name: "report.md", path: "report.md", size: 1536, text_preview: "generated report", expired: false, created_at: messages[1]!.created_at };
    const api = apiStub([{ ...messages[1]!, content: "已生成 `/workspace/report.md`", artifacts: [artifact] }]);
    api.getSessionArtifactDownload = vi.fn(async () => new Blob(["generated report"], { type: "application/octet-stream" }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const wrapper = await mountPageWithAPI(api);

    const disclosureLinks = wrapper.findAll(".message.assistant .artifact-disclosure-links button");
    expect(disclosureLinks.map((item) => item.text())).toEqual(["查看所有产物 (1)", "查看所有变更 (1)"]);
    expect(wrapper.find(".message.assistant .generated-artifact").exists()).toBe(false);
    await disclosureLinks[0]!.trigger("click");
    expect(wrapper.get(".message.assistant .generated-artifact").text()).toContain("report.md");
    expect(wrapper.get(".message.assistant .generated-artifact").text()).toContain("1.5 KB");
    expect(wrapper.get(".message.assistant .markdown-body").text()).toContain("report.md");
    expect(wrapper.get(".message.assistant .markdown-body").text()).not.toContain("/workspace/");
    await disclosureLinks[1]!.trigger("click");
    expect(wrapper.find(".message.assistant .generated-artifact").exists()).toBe(false);
    expect(wrapper.get(".message.assistant .artifact-changes").text()).toContain("report.md");
    expect(wrapper.get(".message.assistant .artifact-changes").text()).not.toContain("generated report");
    await disclosureLinks[0]!.trigger("click");
    await wrapper.get(".message.assistant .generated-artifact").trigger("click");
    await flushPromises();

    expect(api.getSessionArtifactDownload).toHaveBeenCalledWith(session.id, artifact.id);
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(click).toHaveBeenCalledOnce();
    wrapper.unmount();
  });

  it("creates a Session immediately without selecting an Expert or opening a modal", async () => {
    const createdSession: Session = { ...session, id: "session-new", title: "New session" };
    const api = apiStub();
    api.createSession = vi.fn(async () => createdSession);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get(".collection-head .icon-button").trigger("click");
    await flushPromises();

    expect(api.createSession).toHaveBeenCalledWith();
    expect(wrapper.find(".modal-layer").exists()).toBe(false);
    expect(wrapper.get(".conversation-head h2").text()).toBe("New session");
    wrapper.unmount();
  });

  it("omits empty Expert selection UI when no specialist is available", async () => {
    const wrapper = await mountPage([]);

    expect(wrapper.find(".specialist-selector").exists()).toBe(false);
    expect(wrapper.find(".model-dot").exists()).toBe(false);
    expect(wrapper.get(".conversation-head p").text()).toBe("当前会话");
    expect(wrapper.text()).not.toContain("不选择专家");
    wrapper.unmount();
  });

  it("keeps Expert selection available before the first message when an Expert exists", async () => {
    const expert: Expert = {
      id: "expert-1", name: "架构专家", icon: "sparkles", icon_background: "sage", introduction: "负责架构设计", core_capability: "架构设计", operating_procedure: "分析约束", output_standard: "给出架构建议", cautions: "",
      complete: true, compatibility: "verified",
      expertise_tags: [], mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [], available: true,
      created_at: session.created_at, updated_at: session.updated_at, version: 1,
    };
    const api = apiStub([]);
    api.listExperts = vi.fn(async () => [expert]);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get('.composer-plus').trigger('click');
    await wrapper.findAll('.composer-menu button').find((button) => button.text() === '专家')!.trigger('click');
    expect(wrapper.get(".composer-options").text()).toContain(expert.name);
    wrapper.unmount();
  });

  it("renames a Session inline without opening a browser prompt", async () => {
    const localSession = { ...session };
    const prompt = vi.spyOn(window, "prompt");
    const api = apiStub();
    api.listSessions = vi.fn(async (archived = false) => archived ? [] : [localSession]);
    api.renameSession = vi.fn(async (_id, title) => ({ ...localSession, title, version: 2 }));
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get('.session-row .row-actions button[aria-label="重命名"]').trigger("click");
    const input = wrapper.get<HTMLInputElement>(".session-title-input");
    expect(input.element.value).toBe(localSession.title);
    await input.setValue("原地编辑后的标题");
    await input.trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(prompt).not.toHaveBeenCalled();
    expect(api.renameSession).toHaveBeenCalledWith(localSession.id, "原地编辑后的标题", 1);
    expect(wrapper.get(".session-row strong").text()).toBe("原地编辑后的标题");
    expect(wrapper.find(".session-title-input").exists()).toBe(false);
    wrapper.unmount();
  });

  it("uses consistent icons and localized tooltips for Session actions", async () => {
    const wrapper = await mountPage();
    const actions = wrapper.findAll(".session-row .row-actions .action-icon-button");

    expect(actions).toHaveLength(3);
    expect(actions.every((action) => action.find("svg").exists())).toBe(true);
    expect(actions.map((action) => action.get('[role="tooltip"]').text())).toEqual(["重命名", "归档", "删除"]);
    expect(actions[2]!.classes()).toContain("action-icon-button--danger");
    wrapper.unmount();
  });

  it("uses an in-product dialog instead of the native confirm when deleting a Session", async () => {
    const confirm = vi.spyOn(window, "confirm");
    const api = apiStub();
    api.deleteSession = vi.fn(async () => undefined);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get('button[aria-label="删除会话 布局验收"]').trigger("click");
    expect(confirm).not.toHaveBeenCalled();
    expect(api.deleteSession).not.toHaveBeenCalled();
    expect(wrapper.get('[role="alertdialog"]').attributes("aria-modal")).toBe("true");
    expect(wrapper.get(".delete-target strong").text()).toBe(session.title);
    expect(wrapper.text()).toContain("此操作无法撤销");

    await wrapper.get(".delete-actions .button.ghost").trigger("click");
    expect(wrapper.find('[role="alertdialog"]').exists()).toBe(false);
    expect(api.deleteSession).not.toHaveBeenCalled();

    await wrapper.get('button[aria-label="删除会话 布局验收"]').trigger("click");
    await wrapper.get(".delete-actions .button.danger").trigger("click");
    await flushPromises();
    expect(api.deleteSession).toHaveBeenCalledWith(session.id);
    expect(wrapper.find('[role="alertdialog"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("treats a not-found delete as success when the Session was removed elsewhere", async () => {
    const api = apiStub();
    let removed = false;
    api.deleteSession = vi.fn(async () => {
      removed = true;
      throw new ApiError("not_found", 404, "resource_not_found");
    });
    api.listSessions = vi.fn(async (archived = false) => archived ? [] : removed ? [] : [session]);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get('button[aria-label="删除会话 布局验收"]').trigger("click");
    await wrapper.get(".delete-actions .button.danger").trigger("click");
    await flushPromises();

    expect(api.deleteSession).toHaveBeenCalledWith(session.id);
    expect(wrapper.find('[role="alertdialog"]').exists()).toBe(false);
    expect(wrapper.find(".session-row").exists()).toBe(false);
    wrapper.unmount();
  });

  it("does not flash the selected Session title while its messages are loading", async () => {
    const secondSession: Session = { ...session, id: "session-2", title: "不会闪现的标题" };
    let releaseMessages!: (value: SessionMessage[]) => void;
    const pendingMessages = new Promise<SessionMessage[]>((resolve) => { releaseMessages = resolve; });
    const api = apiStub();
    api.listSessions = vi.fn(async (archived = false) => archived ? [] : [session, secondSession]);
    api.listSessionMessages = vi.fn(async (sessionID) => sessionID === secondSession.id ? pendingMessages : messages);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.findAll(".session-row")[1]!.trigger("click");
    await wrapper.vm.$nextTick();
    const transientTitle = wrapper.find(".chat-welcome h2");
    const flashedTitle = transientTitle.exists() && transientTitle.text() === secondSession.title;

    releaseMessages(messages);
    await flushPromises();
    wrapper.unmount();
    expect(flashedTitle).toBe(false);
  });

  it("ignores a stale message response after switching back to another Session", async () => {
    const secondSession: Session = { ...session, id: "session-2", title: "慢响应会话" };
    const staleMessages: SessionMessage[] = [{ ...messages[0]!, content: "不应出现的旧响应" }];
    let releaseMessages!: (value: SessionMessage[]) => void;
    const pendingMessages = new Promise<SessionMessage[]>((resolve) => { releaseMessages = resolve; });
    const api = apiStub();
    api.listSessions = vi.fn(async (archived = false) => archived ? [] : [session, secondSession]);
    api.listSessionMessages = vi.fn(async (sessionID) => sessionID === secondSession.id ? pendingMessages : messages);
    const wrapper = await mountPageWithAPI(api);

    await wrapper.findAll(".session-row")[1]!.trigger("click");
    await wrapper.findAll(".session-row")[0]!.trigger("click");
    releaseMessages(staleMessages);
    await flushPromises();

    expect(wrapper.get(".conversation-head h2").text()).toBe(session.title);
    expect(wrapper.text()).not.toContain("不应出现的旧响应");
    wrapper.unmount();
  });

  it("renders Agent Markdown while keeping user messages as plain text", async () => {
    const wrapper = await mountPage([
      { ...messages[0]!, content: "**用户原文**" },
      { ...messages[1]!, content: "## 结果\n\n- **温度**：28°C\n- `湿度`：82%" },
    ]);

    expect(wrapper.get(".message.user p").text()).toBe("**用户原文**");
    expect(wrapper.find(".message.user strong").exists()).toBe(false);
    expect(wrapper.get(".message.assistant .markdown-body h2").text()).toBe("结果");
    expect(wrapper.get(".message.assistant .markdown-body strong").text()).toBe("温度");
    expect(wrapper.get(".message.assistant .markdown-body code").text()).toBe("湿度");
    wrapper.unmount();
  });

  it("copies each question and answer independently", async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const wrapper = await mountPage([
      { ...messages[0]!, content: "问题原文" },
      { ...messages[1]!, content: "**回答原文**" },
    ]);

    const copyButtons = wrapper.findAll(".message-copy");
    expect(copyButtons[0]!.attributes("aria-label")).toBe("复制当前问题");
    expect(copyButtons[1]!.attributes("aria-label")).toBe("复制当前回答");
    await copyButtons[0]!.trigger("click");
    await copyButtons[1]!.trigger("click");

    expect(writeText).toHaveBeenNthCalledWith(1, "问题原文");
    expect(writeText).toHaveBeenNthCalledWith(2, "**回答原文**");
    expect(copyButtons[1]!.text()).toBe("已复制");
    wrapper.unmount();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
  });

  it("sends without a per-message model override", async () => {
    let releaseStream!: () => void;
    const streamPending = new Promise<void>((resolve) => { releaseStream = resolve; });
    const api = apiStub();
    const provider: ModelProviderConnection = {
      id: "connection-1", name: "Provider", provider_type: "openai", endpoint: "https://model.invalid", protocols: ["openai_responses"], api_key_configured: true, verification_status: "verified", custom_endpoint: true,
      models: [
        { id: "model-1", connection_id: "connection-1", model_id: "default", display_name: "Default", available: true, manually_added: false, compatibility: [{ runtime_engine: "codex", status: "verified" }] },
        { id: "model-2", connection_id: "connection-1", model_id: "selected", display_name: "Selected", available: true, manually_added: false, compatibility: [{ runtime_engine: "codex", status: "verified" }] },
      ],
      created_at: session.created_at, updated_at: session.updated_at, version: 1,
    };
    api.listModelProviderConnections = vi.fn(async () => [provider]);
    const pair: { user_message: SessionMessage; assistant_message: SessionMessage } = {
      user_message: { id: 3, role: "user", state: "completed", content: "使用选中模型", elapsed_ms: 0, created_at: session.updated_at },
      assistant_message: { id: 4, role: "assistant", state: "queued", content: "", elapsed_ms: 0, created_at: session.updated_at },
    };
    api.sendSessionMessage = vi.fn(async () => pair);
    api.streamSessionMessage = vi.fn(async () => streamPending);
    const wrapper = await mountPageWithAPI(api);

    expect(wrapper.find('.composer-model-control').exists()).toBe(false);
    wrapper.get('.composer-editor').element.textContent = '使用选中模型';
    await wrapper.get('.composer-editor').trigger('input');
    await wrapper.get('.composer-toolbar button[aria-label="发送"]').trigger('click');
    await flushPromises();

    expect(api.sendSessionMessage).toHaveBeenCalledWith(session.id, "使用选中模型", [], undefined, { selection_id: "selection-1", file_references: [] });
    releaseStream();
    await flushPromises();
    wrapper.unmount();
  });

  it("shows a jump control away from the bottom and returns to the latest message", async () => {
    const wrapper = await mountPage();
    const stream = wrapper.get<HTMLElement>(".message-stream");
    Object.defineProperties(stream.element, {
      scrollHeight: { configurable: true, value: 1000 },
      clientHeight: { configurable: true, value: 300 },
    });
    stream.element.scrollTop = 120;
    await stream.trigger("scroll");

    const jump = wrapper.get("button.jump-to-latest");
    expect(jump.attributes("aria-label")).toBe("回到最新消息");
    await jump.trigger("click");
    expect(stream.element.scrollTop).toBe(1000);
    expect(wrapper.find("button.jump-to-latest").exists()).toBe(false);
    wrapper.unmount();
  });

  it("reserves the measured composer height so the latest message stays above it", async () => {
    const wrapper = await mountPage();
    const composer = wrapper.get<HTMLElement>(".composer-layer");
    vi.spyOn(composer.element, "getBoundingClientRect").mockReturnValue({
      width: 780, height: 180, top: 520, right: 780, bottom: 700, left: 0, x: 0, y: 520, toJSON: () => ({}),
    });

    window.dispatchEvent(new Event("resize"));
    await wrapper.vm.$nextTick();
    expect(wrapper.get<HTMLElement>(".message-stream").element.style.paddingBottom).toBe("196px");
    wrapper.unmount();
  });

  it("distinguishes queued state from the safe activity description", async () => {
    const wrapper = await mountPage([
      messages[0]!,
      { id: 2, role: "assistant", state: "queued", content: "", progress_stage: "preparing", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" },
    ]);

    expect(wrapper.text()).toContain("思考中");
    expect(wrapper.text()).toContain("正在准备运行环境");
    expect(wrapper.get(".execution-status-bar").text()).toContain("排队中");
    expect(wrapper.get(".execution-status-stop").text()).toContain("取消排队");
    wrapper.unmount();
  });

  it("shows execution summaries before revealing concrete commands", async () => {
    const pending: SessionMessage = { id: 2, role: "assistant", state: "generating", content: "", progress_stage: "using_tool", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" };
    const api = apiStub([messages[0]!, pending]);
    api.streamSessionMessage = vi.fn(async (_sessionID, _messageID, onSnapshot, signal) => {
      onSnapshot({
        state: "generating",
        content: "",
        progress_stage: "using_tool",
        elapsed_ms: 500,
        activities: [
          { type: "reasoning.summary", detail: "先检查仓库状态" },
          { type: "command.requested", detail: "git status --short" },
        ],
      });
      await new Promise<void>((resolve) => signal?.addEventListener("abort", () => resolve(), { once: true }));
    });

    const wrapper = await mountPageWithAPI(api);
    await flushPromises();

    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("本次执行");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项工具调用");
    expect(wrapper.get(".activity-summary-group > summary").text()).toContain("先检查仓库状态");
    expect(wrapper.get(".activity-state").text()).toBe("进行中");
    expect(wrapper.get(".activity-summary-group > summary").text()).not.toContain("git status --short");
    expect(wrapper.get(".activity-detail-list").text()).toContain("git status --short");
    wrapper.unmount();
  });

  it("groups Feishu command pairs into human-readable summaries", async () => {
    const command = (capability: string, args: string) => `/bin/sh -lc 'agent-cli --connector connector-1 --capability ${capability} --identity user -- ${args}'`;
    const chatHelp = command("im_chat_search", "im chat-search --help");
    const chatSearch = command("im_chat_search", "im chat-search --query 云隙科技");
    const send = command("im_messages_send", "im messages-send --chat-id oc_1 --text 大家好");
    const completed: SessionMessage = {
      ...messages[1]!,
      activities: [
        { type: "runtime.started", detail: "codex" },
        { type: "command.requested", detail: chatHelp },
        { type: "command.completed", detail: chatHelp },
        { type: "command.requested", detail: chatSearch },
        { type: "command.completed", detail: chatSearch },
        { type: "command.requested", detail: send },
        { type: "command.completed", detail: send },
      ],
    };

    const wrapper = await mountPage([messages[0]!, completed]);
    const summaries = wrapper.findAll(".activity-summary-group > summary");

    expect(summaries.map((summary) => summary.get("strong").text())).toEqual([
      "运行环境已准备",
      "已调用飞书连接器读取群聊搜索说明",
      "已调用飞书连接器搜索群聊",
      "已调用飞书连接器发送消息",
    ]);
    expect(summaries.every((summary) => !summary.text().includes("/bin/sh"))).toBe(true);
    expect(wrapper.findAll(".activity-summary-group").every((summary) => summary.attributes("open") === undefined)).toBe(true);
    expect(wrapper.get(".activity-detail-list").text()).toContain("codex");
    expect(wrapper.findAll(".activity-detail-list").at(-1)?.text()).toContain("/bin/sh -lc");
    wrapper.unmount();
  });

  it("passes an attempted user CLI operation to the conversation composer", async () => {
    const attempted: SessionMessage = {
      ...messages[1]!,
      state: "failed",
      content: "",
      error: "authorization unavailable",
      activities: [{ type: "command.requested", detail: "/bin/sh -lc 'agent-cli --connector feishu --capability im_messages_send --identity user -- im +messages-send'" }],
    };
    const wrapper = await mountPage([messages[0]!, attempted]);

    expect(wrapper.getComponent(ConversationComposer).props("authorizationRequest")).toEqual({ connectorID: "feishu", capabilityID: "im_messages_send" });
    wrapper.unmount();
  });

  it("offers managed Feishu recovery when authorization fails before any CLI command runs", async () => {
    const authorizationError = "resource is invalid: queued CLI Connector for Stage 1 is unavailable: resource conflicts with current state: Connector authorization is unavailable";
    const failed: SessionMessage = {
      ...messages[1]!, state: "failed", content: "", activities: [],
      error: authorizationError,
      expert_stages: [{ expert_id: "expert-1", expert_name: "飞书专家", position: 1, total: 1, state: "failed", elapsed_ms: 1200, error: authorizationError }],
      response_snapshot: { provider_model_id: "model-1", connection_id: "connection-1", connection_name: "Provider", provider_type: "openai", model_id: "model", model_name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], runtime_engine: "codex", compatibility: "verified", connection_version: 1, stages: [{ position: 1, runtime_engine: "codex", provider_model: { id: "model-1", connection_id: "connection-1", connection_version: 1, connection_name: "Provider", provider_type: "openai", model_id: "model", name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], compatibility: "verified" }, cli_connectors: [{ id: "installation-1", name: "飞书", executable: "lark-cli", authentication_driver: "feishu", bundle_sha256: "a".repeat(64), runtime_digests: [], version: 1 }] }] },
    };
    const wrapper = await mountPage([messages[0]!, failed]);
    expect(wrapper.getComponent(ConversationComposer).props("authorizationRequest")).toEqual({ connectorID: "installation-1", capabilityID: "" });
    const reply = wrapper.get(".message.assistant .message-content").text();
    expect(reply).toContain("等待飞书授权");
    expect(reply).toContain("完成授权后");
    expect(reply).not.toContain("resource is invalid");
    expect(reply).not.toContain("Connector authorization is unavailable");
    expect(wrapper.text()).not.toContain("resource is invalid");
    wrapper.unmount();
  });

  it("offers managed DingTalk recovery when authorization fails before any CLI command runs", async () => {
    const authorizationError = "resource is invalid: queued CLI Connector is unavailable: Connector authorization is unavailable";
    const failed: SessionMessage = {
      ...messages[1]!, state: "failed", content: "", activities: [], error: authorizationError,
      response_snapshot: { provider_model_id: "model-1", connection_id: "connection-1", connection_name: "Provider", provider_type: "openai", model_id: "model", model_name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], runtime_engine: "codex", compatibility: "verified", connection_version: 1, stages: [{ position: 1, runtime_engine: "codex", provider_model: { id: "model-1", connection_id: "connection-1", connection_version: 1, connection_name: "Provider", provider_type: "openai", model_id: "model", name: "Model", endpoint: "https://model.invalid", protocols: ["openai_responses"], compatibility: "verified" }, cli_connectors: [{ id: "installation-1", name: "钉钉", executable: "dws", authentication_driver: "dingtalk", bundle_sha256: "a".repeat(64), runtime_digests: [], version: 1 }] }] },
    };
    const wrapper = await mountPage([messages[0]!, failed]);
    expect(wrapper.getComponent(ConversationComposer).props("authorizationRequest")).toEqual({ connectorID: "installation-1", capabilityID: "" });
    const reply = wrapper.get(".message.assistant .message-content").text();
    expect(reply).toContain("等待钉钉授权");
    expect(reply).not.toContain("Connector authorization is unavailable");
    wrapper.unmount();
  });

  it("does not disguise an unrelated Feishu command failure as an authorization wait", async () => {
    const failed: SessionMessage = {
      ...messages[1]!, state: "failed", content: "", error: "Connector command failed unexpectedly",
      activities: [{ type: "command.requested", detail: "agent-cli --connector feishu --capability im_messages_send --identity user -- im +messages-send" }],
    };
    const wrapper = await mountPage([messages[0]!, failed]);

    expect(wrapper.get(".message.assistant .message-content").text()).toContain("Connector command failed unexpectedly");
    expect(wrapper.text()).not.toContain("等待飞书授权");
    wrapper.unmount();
  });

  it.each(["generating", "waiting_for_user"] as const)("keeps a %s Session approval in the active conversation composer", async (state) => {
    const active: SessionMessage = { ...messages[1]!, state, content: "", progress_stage: "using_tool" };
    const wrapper = await mountPage([messages[0]!, active]);

    expect(wrapper.getComponent(ConversationComposer).props("approvalExecutionId")).toBe(active.id);
    expect(embeddedCommandApproval.value).toEqual({ executionKind: "session", executionID: String(active.id) });
    expect(wrapper.find("#command-approval-slot").exists()).toBe(true);
    wrapper.unmount();
    expect(embeddedCommandApproval.value).toBeUndefined();
  });

  it("does not present an old runtime activity as current after failure", async () => {
    const failed: SessionMessage = {
      id: 2,
      role: "assistant",
      state: "failed",
      content: "",
      error: "command failed",
      elapsed_ms: 189_000,
      created_at: "2026-08-25T12:00:01Z",
      activities: [{ type: "runtime.started", detail: "claude" }],
    };
    const wrapper = await mountPage([messages[0]!, failed]);

    expect(wrapper.find(".runtime-activity-current").exists()).toBe(false);
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("本次执行");
    expect(wrapper.get(".runtime-activity").text()).toContain("运行环境已准备");
    expect(wrapper.get(".runtime-activity").text()).not.toContain("正在准备运行环境");
    wrapper.unmount();
  });

  it("omits a single execution stage that repeats the final answer", async () => {
    const stage = { expert_id: "expert-1", expert_name: "飞书助手", provider_model_name: "GPT 5.6 Sol", runtime_engine: "codex" as const, position: 1, total: 1, state: "succeeded" as const, elapsed_ms: 47_000, final_text: "未发送任何消息。" };
    const duplicate = { ...messages[1]!, content: "未发送任何消息。", expert_stages: [stage] };
    const wrapper = await mountPage([messages[0]!, duplicate]);

    expect(wrapper.find(".expert-stage-list").exists()).toBe(false);
    expect(wrapper.get(".message.assistant .message-content").text().match(/未发送任何消息。/g)).toHaveLength(1);
    wrapper.unmount();
  });

  it("keeps stage results that add information to the final answer", async () => {
    const stage = { expert_id: "expert-1", expert_name: "检索专家", provider_model_name: "GPT 5.6 Sol", runtime_engine: "codex" as const, position: 1, total: 1, state: "succeeded" as const, elapsed_ms: 12_000, final_text: "阶段检索结果" };
    const response = { ...messages[1]!, content: "综合回答", expert_stages: [stage] };
    const wrapper = await mountPage([messages[0]!, response]);

    expect(wrapper.get(".expert-stage-list").text()).toContain("阶段检索结果");
    wrapper.unmount();
  });

  it("reconnects through a command approval wait and displays completion without a page refresh", async () => {
    vi.useFakeTimers();
    const pending: SessionMessage = { ...messages[1]!, state: "generating", content: "", progress_stage: "using_tool" };
    const waiting: SessionMessage = { ...pending, state: "waiting_for_user" };
    const completed: SessionMessage = { ...pending, state: "completed", content: "表格已填写", progress_stage: undefined };
    const api = apiStub();
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce([messages[0]!, pending])
      .mockResolvedValueOnce([messages[0]!, waiting])
      .mockResolvedValue([messages[0]!, completed]);
    api.streamSessionMessage = vi.fn()
      .mockImplementationOnce(async (_sessionID, _messageID, onSnapshot) => {
        onSnapshot({ ...waiting });
        throw new Error("connection lost during approval");
      })
      .mockImplementationOnce(async (_sessionID, _messageID, onSnapshot) => {
        onSnapshot({ ...completed });
      });

    const wrapper = await mountPageWithAPI(api);
    try {
      expect(wrapper.get(".execution-status-bar").text()).toContain("等待用户操作");
      await vi.advanceTimersByTimeAsync(750);
      await flushPromises();

      expect(api.streamSessionMessage).toHaveBeenCalledTimes(2);
      expect(vi.mocked(api.streamSessionMessage).mock.calls[1]?.[4]).toEqual({ reconnect: true });
      await vi.advanceTimersByTimeAsync(200);
      await flushPromises();
      expect(wrapper.get(".message.assistant .message-content").text()).toContain("表格已填写");
      expect(wrapper.find(".execution-status-bar").exists()).toBe(false);
      expect(wrapper.find('button[aria-label="中止生成"]').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("keeps polling after a closed stream while a command approval is pending", async () => {
    vi.useFakeTimers();
    const pending: SessionMessage = { ...messages[1]!, state: "generating", content: "" };
    const waiting: SessionMessage = { ...pending, state: "waiting_for_user", progress_stage: "using_tool" };
    const completed: SessionMessage = { ...pending, state: "completed", content: "表格已填写" };
    const api = apiStub();
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce([messages[0]!, pending])
      .mockResolvedValueOnce([messages[0]!, waiting])
      .mockResolvedValue([messages[0]!, completed]);
    const wrapper = await mountPageWithAPI(api);
    try {
      expect(wrapper.get(".execution-status-bar").text()).toContain("等待用户操作");
      await vi.advanceTimersByTimeAsync(900);
      await flushPromises();

      expect(api.listSessionMessages).toHaveBeenCalledTimes(3);
      expect(wrapper.get(".message.assistant .message-content").text()).toContain("表格已填写");
      expect(wrapper.find('button[aria-label="中止生成"]').exists()).toBe(false);
    } finally { wrapper.unmount(); }
  });

  it("subscribes when opening a Session that is waiting for command approval", async () => {
    const waiting: SessionMessage = { ...messages[1]!, state: "waiting_for_user", content: "", progress_stage: "using_tool" };
    const api = apiStub([messages[0]!, waiting]);
    api.streamSessionMessage = vi.fn((_sessionID, _messageID, _onSnapshot, signal) => new Promise<void>((resolve) => {
      signal?.addEventListener("abort", () => resolve(), { once: true });
    }));
    const wrapper = await mountPageWithAPI(api);
    try {
      expect(api.streamSessionMessage).toHaveBeenCalledTimes(1);
      expect(vi.mocked(api.streamSessionMessage).mock.calls[0]?.slice(0, 2)).toEqual([session.id, waiting.id]);
      expect(wrapper.getComponent(ConversationComposer).props("approvalExecutionId")).toBe(waiting.id);
    } finally { wrapper.unmount(); }
  });

  it.each(["online", "visibilitychange"])("reconciles a waiting Session on browser %s", async (event) => {
    const waiting: SessionMessage = { ...messages[1]!, state: "waiting_for_user", content: "", progress_stage: "using_tool" };
    const completed: SessionMessage = { ...waiting, state: "completed", content: "表格已填写", progress_stage: undefined };
    const api = apiStub();
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce([messages[0]!, waiting])
      .mockResolvedValue([messages[0]!, completed]);
    api.streamSessionMessage = vi.fn((_sessionID, _messageID, _onSnapshot, signal) => new Promise<void>((resolve) => {
      signal?.addEventListener("abort", () => resolve(), { once: true });
    }));
    const wrapper = await mountPageWithAPI(api);
    try {
      (event === "online" ? window : document).dispatchEvent(new Event(event));
      await flushPromises();

      expect(api.listSessionMessages).toHaveBeenCalledTimes(2);
      expect(wrapper.get(".message.assistant .message-content").text()).toContain("表格已填写");
      expect(wrapper.getComponent(ConversationComposer).props("approvalExecutionId")).toBeUndefined();
    } finally { wrapper.unmount(); }
  });

  it("reconciles a completed message after the event stream closes without a terminal snapshot", async () => {
    const pending: SessionMessage = { id: 2, role: "assistant", state: "generating", content: "", progress_stage: "using_tool", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" };
    const completed: SessionMessage = { ...pending, state: "completed", content: "图片内容已识别", progress_stage: undefined, elapsed_ms: 1200 };
    const api = apiStub([messages[0]!, pending]);
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce([messages[0]!, pending])
      .mockResolvedValueOnce([messages[0]!, completed]);
    api.streamSessionMessage = vi.fn(async (_sessionID, _messageID, onSnapshot) => {
      onSnapshot({ state: "generating", content: "", progress_stage: "using_tool", elapsed_ms: 500 });
    });

    const wrapper = await mountPageWithAPI(api);
    await flushPromises();

    expect(api.listSessionMessages).toHaveBeenCalledTimes(2);
    expect(wrapper.text()).toContain("图片内容已识别");
    expect(wrapper.text()).not.toContain("正在调用工具");
    wrapper.unmount();
  });

  it("reads authoritative state before reconnecting a broken message stream", async () => {
    vi.useFakeTimers();
    const pending: SessionMessage = { id: 2, role: "assistant", state: "generating", content: "", progress_stage: "thinking", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" };
    const api = apiStub([messages[0]!, pending]);
    let streamCalls = 0;
    api.streamSessionMessage = vi.fn(async (_sessionID, _messageID, _onSnapshot, signal, options) => {
      streamCalls += 1;
      if (streamCalls === 1) throw new Error("connection lost");
      expect(options).toEqual({ reconnect: true });
      await new Promise<void>((resolve) => signal?.addEventListener("abort", () => resolve(), { once: true }));
    });

    const wrapper = await mountPageWithAPI(api);
    await flushPromises();
    expect(api.listSessionMessages).toHaveBeenCalledTimes(2);

    await vi.advanceTimersByTimeAsync(750);
    await flushPromises();

    expect(api.streamSessionMessage).toHaveBeenCalledTimes(2);
    expect(vi.mocked(api.streamSessionMessage).mock.calls[1]?.[4]).toEqual({ reconnect: true });
    wrapper.unmount();
  });

  it("replaces send with stop and cancels the active backend generation", async () => {
    const pending: SessionMessage = { id: 2, role: "assistant", state: "generating", content: "", progress_stage: "thinking", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" };
    const api = apiStub([messages[0]!, pending]);
    api.cancelSessionMessage = vi.fn(async () => ({ ...pending, state: "cancelled" }));
    const wrapper = await mountPageWithAPI(api);

    expect(wrapper.find(".composer > button:not(.stop-generation)").exists()).toBe(false);
    const stop = wrapper.get('button[aria-label="中止生成"]');
    expect(stop.find("svg").exists()).toBe(true);
    await stop.trigger("click");
    await flushPromises();

    expect(api.cancelSessionMessage).toHaveBeenCalledWith(session.id, pending.id);
    expect(wrapper.text()).toContain("已中止生成");
    expect(wrapper.find(".composer .stop-generation").exists()).toBe(false);
    wrapper.unmount();
  });

  it("reconciles an asynchronously accepted cancellation until it becomes terminal", async () => {
    const pending: SessionMessage = { id: 2, role: "assistant", state: "generating", content: "", progress_stage: "thinking", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" };
    const cancelled: SessionMessage = { ...pending, state: "cancelled", progress_stage: undefined, elapsed_ms: 500 };
    const api = apiStub([messages[0]!, pending]);
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce([messages[0]!, pending])
      .mockResolvedValueOnce([messages[0]!, cancelled]);
    api.streamSessionMessage = vi.fn((_sessionID, _messageID, _onSnapshot, signal) => new Promise<void>((resolve) => {
      signal?.addEventListener("abort", () => resolve(), { once: true });
    }));
    api.cancelSessionMessage = vi.fn(async () => ({ ...pending }));
    const wrapper = await mountPageWithAPI(api);

    await wrapper.get('button[aria-label="中止生成"]').trigger("click");
    await flushPromises();

    expect(api.listSessionMessages).toHaveBeenCalledTimes(2);
    expect(wrapper.text()).toContain("已中止生成");
    expect(wrapper.find(".composer .stop-generation").exists()).toBe(false);
    wrapper.unmount();
  });

  it("reveals a completed response progressively instead of replacing the whole message", async () => {
    vi.useFakeTimers();
    const response = "这是一个会被逐步展示的完整回答，不会在一个渲染帧里全部出现。";
    const pendingMessages = [
      messages[0]!,
      { id: 2, role: "assistant", state: "queued", content: "", progress_stage: "preparing", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" },
    ] satisfies SessionMessage[];
    const api = apiStub(pendingMessages, async (onSnapshot) => {
      onSnapshot({ state: "generating", content: "", progress_stage: "thinking", elapsed_ms: 0 });
      onSnapshot({ state: "completed", content: response, elapsed_ms: 900 });
    });
    api.listSessionMessages = vi.fn()
      .mockResolvedValueOnce(pendingMessages)
      .mockResolvedValueOnce([messages[0]!, { ...pendingMessages[1]!, state: "completed", content: response, progress_stage: undefined, elapsed_ms: 900 }]);
    const wrapper = await mountPageWithAPI(api);
    await flushPromises();

    const initiallyVisible = wrapper.get(".message.assistant p").text();
    expect(initiallyVisible.length).toBeGreaterThan(0);
    expect(initiallyVisible.length).toBeLessThan(response.length);

    await vi.runAllTimersAsync();
    await flushPromises();
    expect(wrapper.get(".message.assistant p").text()).toBe(response);
    expect(wrapper.find(".thinking-state").exists()).toBe(false);
    wrapper.unmount();
    vi.useRealTimers();
  });
});
