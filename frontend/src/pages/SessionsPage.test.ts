// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type Artifact, type Expert, type ModelProviderConnection, type PlatformApi, type Session, type SessionMessage, type SessionMessageSnapshot } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
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
    listSessions: vi.fn(async (archived = false) => archived ? [] : [session]),
    listSessionMessages: vi.fn(async () => sessionMessages),
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

async function mountPageWithAPI(api: PlatformApi) {
  const router = createAppRouter(createMemoryHistory());
  await router.push("/sessions");
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
    Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: vi.fn(function (this: HTMLElement, options?: ScrollToOptions | number) {
      if (typeof options === "object") this.scrollTop = options.top ?? this.scrollTop;
    }) });
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:attachment-preview") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  });
  afterEach(() => {
    delete (HTMLElement.prototype as { scrollTo?: unknown }).scrollTo;
    delete (URL as { createObjectURL?: unknown }).createObjectURL;
    delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
    vi.restoreAllMocks();
  });

  it("keeps user and Agent messages in distinct role rows", async () => {
    const wrapper = await mountPage();
    expect(wrapper.get(".message.user .message-content").text()).toContain("我的消息");
    expect(wrapper.get(".message.assistant .message-content").text()).toContain("Agent 的消息");
    expect(wrapper.find(".message-avatar").exists()).toBe(false);
    expect(wrapper.find(".composer-layer").exists()).toBe(true);
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

    expect(wrapper.get(".specialist-selector").text()).toContain(expert.name);
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
    await wrapper.get<HTMLTextAreaElement>(".composer textarea").setValue("使用选中模型");
    await wrapper.get(".composer > button:last-child").trigger("click");
    await flushPromises();

    expect(api.sendSessionMessage).toHaveBeenCalledWith(session.id, "使用选中模型", []);
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

  it("shows safe progress instead of a queued label while the Agent is working", async () => {
    const wrapper = await mountPage([
      messages[0]!,
      { id: 2, role: "assistant", state: "queued", content: "", progress_stage: "preparing", elapsed_ms: 0, created_at: "2026-08-25T12:00:01Z" },
    ]);

    expect(wrapper.text()).toContain("思考中");
    expect(wrapper.text()).toContain("正在准备运行环境");
    expect(wrapper.text()).not.toContain("排队中");
    wrapper.unmount();
  });

  it("shows an expandable activity timeline with the concrete Codex command", async () => {
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

    expect(wrapper.get(".runtime-activity summary").text()).toContain("查看执行过程");
    expect(wrapper.get(".runtime-activity").text()).toContain("先检查仓库状态");
    expect(wrapper.get(".runtime-activity").text()).toContain("git status --short");
    wrapper.unmount();
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
