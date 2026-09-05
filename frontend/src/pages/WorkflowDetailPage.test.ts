// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type Artifact, type Expert, type PlatformApi, type Run, type Workflow } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import WorkflowDetailPage from "./WorkflowDetailPage.vue";

const workflow: Workflow = {
  id: "workflow-1",
  name: "每日分析",
  goal: "汇总今天的变化",
  environment: [],
  api_credential_configured: false,
  deleted: false,
  created_at: "2026-08-29T00:00:00Z",
  updated_at: "2026-08-29T00:00:00Z",
  version: 1,
};

const run: Run = {
  id: "run-1",
  conversation_id: "run-1",
  turn_number: 1,
  workflow_id: workflow.id,
  workflow_name: workflow.name,
  trigger: "manual",
  state: "succeeded",
  text_input: "重点关注发布风险",
  final_text: "## 分析结果\n\n- 没有阻塞项",
  queued_at: "2026-08-29T02:18:08Z",
  started_at: "2026-08-29T02:18:09Z",
  ended_at: "2026-08-29T02:18:33Z",
  elapsed_ms: 24_000,
};

function apiStub(overrides: Partial<PlatformApi> = {}): PlatformApi {
  return {
    getWorkflow: vi.fn(async () => workflow),
    listExperts: vi.fn(async () => []),
    listExpertTeams: vi.fn(async () => []),
    listModelProviderConnections: vi.fn(async () => []),
    listRuntimeEngines: vi.fn(async () => []),
    listRuns: vi.fn(async () => [run]),
    listRunTurns: vi.fn(async () => [run]),
    listArtifacts: vi.fn(async () => []),
    listWorkspace: vi.fn(async () => ({ items: [], used_bytes: 0, limit_bytes: 1024 })),
    streamRunEvents: vi.fn(async (_workflowID, _runID, onEvent) => {
      onEvent({ sequence: 1, type: "message.delta", payload: { delta: "分析结果" }, raw: "{}" });
    }),
    ...overrides,
  } as unknown as PlatformApi;
}

async function mountPage(api = apiStub()) {
  const router = createAppRouter(createMemoryHistory());
  await router.push(`/workflows/${workflow.id}?tab=history`);
  await router.isReady();
  const wrapper = mount(WorkflowDetailPage, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api },
    },
  });
  await flushPromises();
  return wrapper;
}

describe("WorkflowDetailPage", () => {
  beforeEach(() => {
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:workflow-attachment") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  });
  afterEach(() => {
    delete (URL as { createObjectURL?: unknown }).createObjectURL;
    delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
    vi.restoreAllMocks();
  });

  it("does not repeat the active tab label as a section title", async () => {
    const wrapper = await mountPage();

    expect(wrapper.find(".detail-hero .eyebrow").exists()).toBe(false);

    for (const tabButton of wrapper.findAll(".tabs button")) {
      await tabButton.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.find(".tab-content .section-heading h2").exists()).toBe(false);
    }
    expect(wrapper.find(".settings-section .section-number").exists()).toBe(false);
    wrapper.unmount();
  });

  it("opens a Run as a conversation instead of raw Runtime events", async () => {
    const wrapper = await mountPage();
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".run-conversation .message.user").text()).toContain("重点关注发布风险");
    expect(wrapper.get(".run-conversation .message.assistant .markdown-body h2").text()).toBe("分析结果");
    expect(wrapper.find(".run-page").exists()).toBe(true);
    expect(wrapper.find(".run-dialog").exists()).toBe(false);
    expect(wrapper.find(".detail-hero").exists()).toBe(false);
    expect(wrapper.find(".run-conversation-head .eyebrow").exists()).toBe(false);
    expect(wrapper.text()).not.toContain("message.delta");
    expect(wrapper.text()).not.toContain("工作流快照");
    wrapper.unmount();
  });

  it("renders a Run Conversation image from authenticated attachment content", async () => {
    const turn: Run = { ...run, attachments: [{ id: "attachment-1", name: "photo.png", content_type: "image/png", size: 5, sha256: "digest", image: true }] };
    const getAttachmentDownload = vi.fn(async () => new Blob(["image"], { type: "image/png" }));
    const wrapper = await mountPage(apiStub({ listRunTurns: vi.fn(async () => [turn]), getAttachmentDownload }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(getAttachmentDownload).toHaveBeenCalledWith("attachment-1");
    expect(wrapper.get<HTMLImageElement>('.turn-attachment img[alt="photo.png"]').attributes("src")).toBe("blob:workflow-attachment");
    wrapper.unmount();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:workflow-attachment");
  });

  it("shows the latest turn state and time in the Run Conversation header", async () => {
    const latestTurn: Run = {
      ...run,
      id: "run-2",
      turn_number: 2,
      state: "failed",
      text_input: "检查最新状态",
      final_text: undefined,
      error: "执行失败",
      queued_at: "2026-09-02T07:13:02Z",
      started_at: "2026-09-02T07:13:03Z",
      ended_at: "2026-09-02T07:14:08Z",
      elapsed_ms: 65_000,
    };
    const wrapper = await mountPage(apiStub({ listRunTurns: vi.fn(async () => [run, latestTurn]) }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    const header = wrapper.get(".run-conversation-head").text();
    expect(header).toContain("失败");
    expect(header).toContain(new Date(latestTurn.started_at!).toLocaleString());
    expect(header).not.toContain(new Date(run.started_at!).toLocaleString());
    wrapper.unmount();
  });

  it("shows a finite live accumulated duration while the latest turn is running", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-02T07:15:03Z"));
    const completedTurn: Run = { ...run, elapsed_ms: Number.NaN };
    const activeTurn: Run = {
      ...run,
      id: "run-2",
      turn_number: 2,
      state: "running",
      queued_at: "2026-09-02T07:13:02Z",
      started_at: "2026-09-02T07:13:03Z",
      ended_at: undefined,
      elapsed_ms: 0,
    };
    const wrapper = await mountPage(apiStub({ listRunTurns: vi.fn(async () => [completedTurn, activeTurn]) }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    const header = wrapper.get(".run-conversation-head").text();
    expect(header).toContain("2分钟");
    expect(header).not.toContain("NaN");
    wrapper.unmount();
    vi.useRealTimers();
  });

  it("shows only files as artifacts and ignores legacy final-result records", async () => {
    const legacyResult: Artifact = { id: "result-1", run_id: run.id, kind: "result", name: "Final result", path: "", size: 0, text_preview: "done", expired: false, created_at: run.ended_at! };
    const generatedFile: Artifact = { id: "file-1", run_id: run.id, kind: "file", name: "report.md", path: "report.md", size: 12, sha256: "abc", expired: false, created_at: run.ended_at! };
    const wrapper = await mountPage(apiStub({ listArtifacts: vi.fn(async () => [legacyResult, generatedFile]) }));

    await wrapper.findAll(".tabs button").at(0)!.trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.get(".artifact-list").text()).toContain("report.md");
    expect(wrapper.get(".artifact-list").text()).not.toContain("Final result");

    await wrapper.findAll(".tabs button").at(2)!.trigger("click");
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    expect(wrapper.get(".generated-artifacts").text()).toContain("report.md");
    expect(wrapper.get(".generated-artifacts").text()).not.toContain("Final result");
    wrapper.unmount();
  });

  it("downloads a Run Artifact from its conversation card", async () => {
    const generatedFile: Artifact = { id: "file-1", run_id: run.id, kind: "file", name: "report.md", path: "report.md", size: 1536, sha256: "abc", expired: false, created_at: run.ended_at! };
    const artifactRun = { ...run, final_text: "已生成 `/workspace/report.md`" };
    const getArtifactDownload = vi.fn(async () => new Blob(["workflow report"], { type: "application/octet-stream" }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [artifactRun]), listRunTurns: vi.fn(async () => [artifactRun]), listArtifacts: vi.fn(async () => [generatedFile]), getArtifactDownload }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    expect(wrapper.get(".generated-artifact").text()).toContain("1.5 KB");
    expect(wrapper.get(".message.assistant .markdown-body").text()).toContain("report.md");
    expect(wrapper.get(".message.assistant .markdown-body").text()).not.toContain("/workspace/");
    await wrapper.get(".generated-artifact").trigger("click");
    await flushPromises();

    expect(getArtifactDownload).toHaveBeenCalledWith(workflow.id, generatedFile.id);
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(click).toHaveBeenCalledOnce();
    wrapper.unmount();
  });

  it("runs the Workflow goal and opens its conversation immediately", async () => {
    const runWorkflow = vi.fn(async () => run);
    const wrapper = await mountPage(apiStub({ runWorkflow }));

    expect(wrapper.find('[placeholder="本次补充输入（可选）"]').exists()).toBe(false);
    await wrapper.get(".detail-hero .button.primary").trigger("click");
    await flushPromises();

    expect(runWorkflow).toHaveBeenCalledWith(workflow.id);
    expect(wrapper.get(".run-page").text()).toContain("运行对话");
    expect(wrapper.get(".run-conversation .message.user").text()).toContain(workflow.goal);
    wrapper.unmount();
  });

  it("keeps every Workflow setting section expanded by default", async () => {
    const wrapper = await mountPage();
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    const sections = wrapper.findAll(".settings-section");
    expect(sections.length).toBeGreaterThan(1);
    expect(sections.every((section) => section.attributes("open") !== undefined)).toBe(true);
    wrapper.unmount();
  });

  it("keeps Workflow settings usable when a migrated Expert has no Runtime Engine", async () => {
    const incompleteExpert = {
      id: "expert-incomplete",
      name: "待完善专家",
      provider_model_name: "",
      available: false,
      compatibility: "unavailable",
      runtime_engine: undefined,
    } as unknown as Expert;
    const wrapper = await mountPage(apiStub({ listExperts: vi.fn(async () => [incompleteExpert]) }));

    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    expect(wrapper.get(".settings-section").text()).toContain("待完善专家");
    expect(wrapper.get(".settings-section").text()).not.toContain("Codex");
    wrapper.unmount();
  });

  it("submits a Workflow-scoped SSH config for private Git clone", async () => {
    const configureWorkflowGitSource = vi.fn(async () => workflow);
    const wrapper = await mountPage(apiStub({ configureWorkflowGitSource }));
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    await wrapper.get<HTMLSelectElement>('.git-settings select').setValue("ssh");
    await wrapper.get<HTMLInputElement>('.git-settings input[placeholder*="git@github.com"]').setValue("agent-platform:/srv/git/project.git");
    await wrapper.get<HTMLTextAreaElement>('textarea[name="ssh-config"]').setValue("Host agent-platform\n  HostName 47.237.108.63\n  User root\n  IdentityFile ~/.ssh/xinjiapo.pem\n");
    await wrapper.get<HTMLTextAreaElement>('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').setValue("private-key");
    await wrapper.get(".git-settings .button.primary").trigger("click");
    await flushPromises();

    expect(configureWorkflowGitSource).toHaveBeenCalledWith(workflow.id, expect.objectContaining({
      ssh_config: expect.stringContaining("Host agent-platform"),
    }));
    wrapper.unmount();
  });

  it("accepts SCP-style SSH repository addresses in native form validation", async () => {
    const wrapper = await mountPage();
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    const input = wrapper.get<HTMLInputElement>('.git-settings input[placeholder*="git@github.com"]');
    await input.setValue("git@github.com:owl-go/agent-platform.git");

    expect(input.element.type).toBe("text");
    expect(input.element.validity.typeMismatch).toBe(false);
    wrapper.unmount();
  });

  it("continues a Run Conversation from the composer", async () => {
    const followUp: Run = {
      ...run,
      id: "run-2",
      turn_number: 2,
      state: "queued",
      text_input: "继续给出修复建议",
      final_text: undefined,
      started_at: undefined,
      ended_at: undefined,
      elapsed_ms: 0,
    };
    const continueRunConversation = vi.fn(async () => followUp);
    const api = apiStub({
      continueRunConversation,
      getRun: vi.fn(async (): Promise<Run> => ({ ...followUp, state: "succeeded", final_text: "已补充修复建议" })),
    });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    await wrapper.get(".run-composer textarea").setValue("继续给出修复建议");
    await wrapper.get(".run-composer").trigger("submit");
    await flushPromises();

    expect(continueRunConversation).toHaveBeenCalledWith(workflow.id, run.id, "继续给出修复建议", []);
    expect(wrapper.findAll(".run-conversation .message.user")).toHaveLength(2);
    wrapper.unmount();
  });

  it("uses the same centered composer layout as a Session", async () => {
    const wrapper = await mountPage();
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.find(".run-page > .composer-layer > .run-composer.composer").exists()).toBe(true);
    wrapper.unmount();
  });

  it("reserves the measured composer height so the latest Run message stays visible", async () => {
    const wrapper = await mountPage();
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    const composer = wrapper.get<HTMLElement>(".run-composer-layer");
    vi.spyOn(composer.element, "getBoundingClientRect").mockReturnValue({
      width: 780, height: 214, top: 486, right: 780, bottom: 700, left: 0, x: 0, y: 486, toJSON: () => ({}),
    });

    window.dispatchEvent(new Event("resize"));
    await wrapper.vm.$nextTick();
    expect(wrapper.get<HTMLElement>(".run-conversation").element.style.paddingBottom).toBe("230px");
    wrapper.unmount();
  });

  it("shows live Runtime activity and progressively reveals a whole final chunk", async () => {
    vi.useFakeTimers();
    const response = "这是 Runtime 最终一次性交付的完整回答，界面仍然需要逐步展示。";
    const activeRun: Run = {
      ...run,
      state: "running",
      final_text: undefined,
      ended_at: undefined,
      elapsed_ms: 0,
    };
    const api = apiStub({
      listRuns: vi.fn(async () => [activeRun]),
      listRunTurns: vi.fn(async () => [activeRun]),
      streamRunEvents: vi.fn(async (_workflowID, _runID, onEvent, signal) => {
        onEvent({ sequence: 1, type: "runtime.started", payload: { runtime: "codex" }, raw: "{}" });
        onEvent({ sequence: 2, type: "command.requested", payload: { command: "git status" }, raw: "{}" });
        onEvent({ sequence: 3, type: "message.delta", payload: { delta: response }, raw: "{}" });
        await new Promise<void>((_resolve, reject) => signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true }));
      }),
    });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".runtime-activity").text()).toContain("正在调用工具");
    const initiallyVisible = wrapper.get(".run-conversation .message.assistant .markdown-body").text();
    expect(initiallyVisible.length).toBeGreaterThan(0);
    expect(initiallyVisible.length).toBeLessThan(response.length);

    await vi.advanceTimersByTimeAsync(2_000);
    await flushPromises();
    expect(wrapper.get(".run-conversation .message.assistant .markdown-body").text()).toBe(response);
    wrapper.unmount();
    vi.useRealTimers();
  });
});
