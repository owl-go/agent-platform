// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { ApiError, platformApiKey, type Artifact, type CLIConnectorDefinition, type ConnectorInstallation, type ConversationScope, type Expert, type KnowledgeBase, type PlatformApi, type Run, type RunEvent, type Workflow } from "../api/client";
import { createAppI18n } from "../i18n";
import { conversationApiStub, emptySelection } from "../test/conversation";
import { createAppRouter } from "../router";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import ConversationComposer from "../components/ConversationComposer.vue";
import { embeddedCommandApproval } from "../commandApprovalPlacement";
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
    ...conversationApiStub(),
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

async function mountPage(api = apiStub(), path = `/workflows/${workflow.id}?tab=history`) {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path);
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
  it("identifies the configured channel in Run History", async () => {
    const channelRun: Run = { ...run, trigger: "message_channel", message_channel_id: "channel-1", message_channel_name: "Telegram 问答" };
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [channelRun]) }));
    expect(wrapper.get('.run-row[role="button"]').text()).toContain("消息渠道 · Telegram 问答");
    wrapper.unmount();
  });
  beforeEach(() => {
    window.localStorage.clear();
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:workflow-attachment") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  });
  afterEach(() => {
    vi.useRealTimers();
    delete (URL as { createObjectURL?: unknown }).createObjectURL;
    delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
    vi.restoreAllMocks();
  });

  it("places the current Run approval beside its conversation composer", async () => {
    const waitingRun: Run = { ...run, id: "run-2", conversation_id: run.id, turn_number: 2, state: "waiting_for_user", final_text: undefined };
    const wrapper = await mountPage(apiStub({ listRunTurns: vi.fn(async () => [run, waitingRun]) }), `/workflows/${workflow.id}?open_run=${run.id}`);

    expect(wrapper.getComponent(ConversationComposer).props("approvalExecutionId")).toBe(waitingRun.id);
    expect(embeddedCommandApproval.value).toEqual({ executionKind: "run", executionID: waitingRun.id });
    expect(wrapper.find("#command-approval-slot").exists()).toBe(true);
    wrapper.unmount();
    expect(embeddedCommandApproval.value).toBeUndefined();
  });

  it("shows independent Connector selection for each Workflow run", async () => {
    const secondRun: Run = { ...run, id: "run-2", conversation_id: "run-2", queued_at: "2026-08-29T03:18:08Z" };
    const selected = { id: "installation-1", name: "飞书", icon: "feishu", revision: "1" };
    const definition: CLIConnectorDefinition = { id: selected.id, name: selected.name, icon: selected.icon, description: "", installation_type: "npm", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "", executable: "lark-cli", authentication_driver: "feishu", capabilities: [], supported_architectures: ["linux-amd64"], recommended_skills: [], recommended_skill_ids: [], state: "available", mutable: false, version: 1, conformance_runtime_digests: [], managed_installation: true };
    const installation: ConnectorInstallation = { id: selected.id, source: "feishu", active_revision_id: "revision-1", state: "active", authorized: true, version: 1, package_version: "1.0.93", name: selected.name, description: "", authentication_driver: "feishu", upgrade_available: false };
    const api = apiStub({
      listRuns: vi.fn(async () => [secondRun, run]),
      listRunTurns: vi.fn(async (_workflowID: string, runID: string) => [runID === secondRun.id ? secondRun : run]),
      getConversationSelection: vi.fn(async (scope: ConversationScope) => ({ ...emptySelection(), id: `selection-${scope.run_id}`, inherited_cli_connectors: [selected], disabled_connectors: scope.run_id === secondRun.id ? [`cli:${selected.id}`] : [] })),
      listCLIConnectorDefinitions: vi.fn(async () => [definition]),
      listConnectorInstallations: vi.fn(async () => [installation]),
    });
    const wrapper = await mountPage(api);
    await wrapper.findAll('.run-row[role="button"]')[1]!.trigger("click");
    await flushPromises();
    expect(wrapper.get('.run-composer .composer-connector[aria-label="飞书"]').attributes("title")).toContain("已用于当前对话");
    await wrapper.get(".run-conversation-head .back-link").trigger("click");
    await wrapper.findAll('.run-row[role="button"]')[0]!.trigger("click");
    await flushPromises();
    const connector = wrapper.get('.run-composer .composer-connector[aria-label="飞书"]');
    expect(connector.classes()).toContain("is-off");
    expect(connector.attributes("title")).toContain("未用于当前对话");
    wrapper.unmount();
  });

  it("offers advanced setup only after a successful validation run", async () => {
    const pending = await mountPage(apiStub({ listRuns: vi.fn(async () => [{ ...run, state: "waiting_for_user" as const }]) }));
    expect(pending.find(".workflow-next-steps").exists()).toBe(false);
    pending.unmount();

    const validated = await mountPage();
    expect(validated.get(".workflow-next-steps").text()).toContain("这个工作流已经跑通");
    expect(validated.get(".workflow-next-steps").text()).toContain("定时触发");
    expect(validated.get(".workflow-next-steps").text()).toContain("接入业务系统");
    expect(validated.get(".workflow-next-steps").text()).toContain("连接代码仓库");
    expect(validated.get(".workflow-next-steps").text()).toContain("配置消息渠道");
    await validated.get(".workflow-next-steps .el-button").trigger("click");
    await flushPromises();
    expect(validated.findAll(".tabs button")[3]!.classes()).toContain("active");
    expect((validated.get("#workflow-settings-schedule").element as HTMLDetailsElement).open).toBe(true);
    validated.unmount();
  });

  it("opens Message Channel setup from the successful Run quick configuration", async () => {
    const listMessageChannels = vi.fn(async () => ({ items: [], available: true }));
    const wrapper = await mountPage(apiStub({ listMessageChannels }));
    expect(listMessageChannels).not.toHaveBeenCalled();
    const action = wrapper.findAll(".workflow-next-step-actions .el-button").find(button => button.text() === "配置消息渠道")!;
    await action.trigger("click");
    await flushPromises();

    expect(wrapper.findAll(".tabs button")[3]!.classes()).toContain("active");
    const section = wrapper.get("#workflow-settings-channels");
    expect((section.element as HTMLDetailsElement).open).toBe(true);
    expect(listMessageChannels).toHaveBeenCalledWith(workflow.id, expect.any(AbortSignal));
    expect(section.findAll(".channel-provider-card")).toHaveLength(13);
    await section.get('[data-provider="slack"]').trigger("click");
    await flushPromises();
    const dialog = Array.from(document.body.querySelectorAll('[role="dialog"]')).find((item) => item.textContent?.includes("配置 Slack"))!;
    expect(dialog).toBeDefined();
    expect(dialog.textContent).toContain("配置 Slack");
    expect(dialog.textContent).toContain("Signing Secret");
    expect(dialog.querySelectorAll('input[type="password"]')).toHaveLength(2);
    wrapper.unmount();
  });

  it("opens the operational Overview by default", async () => {
    const wrapper = await mountPage(apiStub(), `/workflows/${workflow.id}`);
    expect(wrapper.findAll(".tabs button")[0]!.classes()).toContain("active");
    expect(wrapper.get(".workflow-overview-goal").text()).toContain(workflow.goal);
    expect(wrapper.get(".workflow-overview-metrics").text()).toContain("30 天成功率");
    expect(wrapper.get(".workflow-recent-runs").text()).toContain("最近运行");
    wrapper.unmount();
  });

  it("shows a recoverable error when initial validation could not start", async () => {
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => []) }), `/workflows/${workflow.id}?tab=history&validation_error=1`);
    expect(wrapper.text()).toContain("工作流已创建，但验证运行未能启动");
    wrapper.unmount();
  });

  it("pauses polling in Settings and refreshes when returning to Run History", async () => {
    vi.useFakeTimers();
    const api = apiStub();
    const wrapper = await mountPage(api);
    await wrapper.findAll(".tabs button")[3]!.trigger("click");
    vi.mocked(api.listRuns).mockClear(); vi.mocked(api.listArtifacts).mockClear();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listRuns).not.toHaveBeenCalled();
    expect(api.listArtifacts).not.toHaveBeenCalled();
    await wrapper.findAll(".tabs button")[1]!.trigger("click");
    await flushPromises();
    expect(api.listRuns).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(15_000);
    expect(api.listRuns).toHaveBeenCalledOnce();
    expect(api.listArtifacts).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("serializes active Run polling and refreshes artifacts once when a Run completes", async () => {
    vi.useFakeTimers();
    let finish: ((items: Run[]) => void) | undefined;
    const api = apiStub({ listRuns: vi.fn<PlatformApi["listRuns"]>()
      .mockResolvedValueOnce([{ ...run, state: "running", ended_at: undefined }])
      .mockImplementationOnce(() => new Promise<Run[]>((resolve) => { finish = resolve; }))
      .mockResolvedValue([run]) });
    const wrapper = await mountPage(api);
    await vi.advanceTimersByTimeAsync(6_000);
    expect(api.listRuns).toHaveBeenCalledTimes(2);
    expect(api.listArtifacts).toHaveBeenCalledOnce();
    finish?.([run]); await flushPromises();
    expect(api.listArtifacts).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(api.listRuns).toHaveBeenCalledTimes(3);
    expect(api.listArtifacts).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });

  it("pauses hidden-tab polling and refreshes immediately on return", async () => {
    vi.useFakeTimers();
    const api = apiStub({ listRuns: vi.fn(async () => [{ ...run, state: "running" as const }]) });
    const wrapper = await mountPage(api);
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listRuns).toHaveBeenCalledOnce();
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    document.dispatchEvent(new Event("visibilitychange")); await flushPromises();
    expect(api.listRuns).toHaveBeenCalledTimes(2);
    wrapper.unmount();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listRuns).toHaveBeenCalledTimes(2);
  });

  it("does not repeat the active tab label as a section title", async () => {
    const wrapper = await mountPage();

    expect(wrapper.find(".detail-hero .eyebrow").exists()).toBe(false);
    expect(wrapper.get(".detail-hero h2").text()).toBe(workflow.name);
    expect(wrapper.find(".detail-hero h1").exists()).toBe(false);
    expect(wrapper.find(".detail-hero").text()).not.toContain(workflow.goal);

    for (const tabButton of wrapper.findAll(".tabs button")) {
      await tabButton.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.find(".tab-content .section-heading h2").exists()).toBe(false);
    }
    expect(wrapper.find(".settings-section .section-number").exists()).toBe(false);
    wrapper.unmount();
  });

  it("shows the source Session and opens the first validation Run from the conversion link", async () => {
    const validationRun: Run = { ...run, trigger: "session_conversion", state: "queued", execution_plan: { id: "plan-1", state: "approved", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules", steps: [{ id: "step-1", kind: "execute_stage", label: "执行任务", position: 1, state: "pending" }], resources: [], side_effects: [], reasons: ["workflow_execution"], estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0 } };
    const linkedWorkflow: Workflow = { ...workflow, origin: { session_id: "session-1", message_id: 2, workflow_id: workflow.id, workflow_name: workflow.name, validation_run_id: validationRun.id, created_at: validationRun.queued_at } };
    const api = apiStub({ getWorkflow: vi.fn(async () => linkedWorkflow), listRuns: vi.fn(async () => [validationRun]), listRunTurns: vi.fn(async () => [validationRun]) });
    const wrapper = await mountPage(api, `/workflows/${workflow.id}?open_run=${validationRun.id}`);

    expect(api.listRunTurns).toHaveBeenCalledWith(workflow.id, validationRun.id);
    expect(wrapper.get(".run-page .execution-plan-card").text()).toContain("执行任务");
    expect(wrapper.get(".run-page .execution-plan-card").text()).toContain("自动开始");
    expect(wrapper.find(".run-page .execution-plan-card footer").exists()).toBe(false);
    wrapper.unmount();

    const summary = await mountPage(api);
    expect(summary.get(".workflow-origin-link").text()).toContain("来自会话");
    summary.unmount();
  });

  it("shows the input and plan actions when the API omits empty plan resources", async () => {
    const pendingRun: Run = {
      ...run,
      state: "waiting_for_user",
      final_text: undefined,
      execution_plan: {
        id: "plan-1", state: "pending", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", label: "执行任务", position: 1, state: "pending" }],
        side_effects: ["workspace_files_may_change"], reasons: ["workflow_execution"],
        estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      } as NonNullable<Run["execution_plan"]>,
    };
    const decideRunExecutionPlan = vi.fn(async () => pendingRun);
    const api = apiStub({ listRuns: vi.fn(async () => [pendingRun]), listRunTurns: vi.fn(async () => [pendingRun]), decideRunExecutionPlan, getAttachmentDownload: vi.fn(async () => new Blob()) });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".run-conversation .message.user").text()).toContain(workflow.goal);
    expect(wrapper.find(".run-conversation .execution-plan-card footer").exists()).toBe(false);
    expect(wrapper.get(".task-workspace-plan-actions").text()).toContain("按计划开始");
    await wrapper.get(".task-workspace-plan-actions .el-button--primary").trigger("click");
    await flushPromises();
    expect(decideRunExecutionPlan).toHaveBeenCalledWith(workflow.id, pendingRun.id, "start", 1);
    await wrapper.get(".task-workspace-panel > header button").trigger("click");
    expect(wrapper.get(".run-conversation .execution-plan-card footer").text()).toContain("按计划开始");
    wrapper.unmount();
  });

  it("keeps a pending plan until the edited request is submitted", async () => {
    const pendingRun: Run = {
      ...run, state: "waiting_for_user", final_text: undefined,
      execution_plan: {
        id: "plan-1", state: "pending", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", label: "执行任务", position: 1, state: "pending" }],
        resources: [], side_effects: ["workspace_files_may_change"], reasons: ["workflow_execution"],
        estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      },
    };
    const cancelled: Run = { ...pendingRun, state: "cancelled", execution_plan: { ...pendingRun.execution_plan!, state: "cancelled" } };
    const decideRunExecutionPlan = vi.fn(async () => cancelled);
    const replacement: Run = { ...run, id: "run-2", turn_number: 2, state: "queued", final_text: undefined };
    const continueRunConversation = vi.fn(async () => replacement);
    const api = apiStub({ listRuns: vi.fn(async () => [pendingRun]), listRunTurns: vi.fn(async () => [pendingRun]), decideRunExecutionPlan, continueRunConversation, getAttachmentDownload: vi.fn(async () => new Blob()) });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    const editButton = wrapper.findAll(".task-workspace-plan-actions .el-button").find((button) => button.text() === "修改要求");
    expect(editButton).toBeDefined();
    await editButton!.trigger("click");
    await flushPromises();

    expect(decideRunExecutionPlan).not.toHaveBeenCalled();
    expect(wrapper.get(".run-plan-edit-overlay").text()).toContain("提交后");
    expect(wrapper.get(".run-composer .composer-editor").text()).toContain(workflow.goal);
    await wrapper.get(".run-plan-edit-heading .el-button").trigger("click");
    expect(wrapper.find(".run-plan-edit-overlay").exists()).toBe(false);
    expect(decideRunExecutionPlan).not.toHaveBeenCalled();
    await editButton!.trigger("click");
    await flushPromises();
    await wrapper.get(".run-composer [aria-label='发送']").trigger("click");
    await flushPromises();
    expect(decideRunExecutionPlan).toHaveBeenCalledWith(workflow.id, pendingRun.id, "cancel", 1);
    expect(continueRunConversation).toHaveBeenCalledWith(workflow.id, pendingRun.id, `${workflow.goal}\n\n${pendingRun.text_input}`, [], undefined, expect.any(Object));
    expect(wrapper.find(".run-plan-edit-overlay").exists()).toBe(false);
    wrapper.unmount();
  });

  it("keeps the edited request available when creating its replacement fails", async () => {
    const pendingRun: Run = {
      ...run, state: "waiting_for_user", final_text: undefined,
      execution_plan: {
        id: "plan-1", state: "pending", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", label: "执行任务", position: 1, state: "pending" }],
        resources: [], side_effects: [], reasons: ["workflow_execution"],
        estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      },
    };
    const cancelled: Run = { ...pendingRun, state: "cancelled", execution_plan: { ...pendingRun.execution_plan!, state: "cancelled" } };
    const replacement: Run = { ...run, id: "run-2", turn_number: 2, state: "queued", final_text: undefined };
    const decideRunExecutionPlan = vi.fn(async () => cancelled);
    const continueRunConversation = vi.fn().mockRejectedValueOnce(new Error("temporary failure")).mockResolvedValue(replacement);
    const api = apiStub({ listRuns: vi.fn(async () => [pendingRun]), listRunTurns: vi.fn(async () => [pendingRun]), decideRunExecutionPlan, continueRunConversation });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    const editButton = wrapper.findAll(".task-workspace-plan-actions .el-button").find((button) => button.text() === "修改要求");
    await editButton!.trigger("click");
    await flushPromises();
    await wrapper.get(".run-composer [aria-label='发送']").trigger("click");
    await flushPromises();

    expect(wrapper.get(".run-plan-edit-overlay").text()).toContain("新任务未创建");
    expect(wrapper.get(".run-composer .composer-editor").text()).toContain(workflow.goal);
    expect(continueRunConversation).toHaveBeenCalledTimes(1);
    await wrapper.get(".run-composer [aria-label='发送']").trigger("click");
    await flushPromises();
    expect(decideRunExecutionPlan).toHaveBeenCalledTimes(1);
    expect(continueRunConversation).toHaveBeenCalledTimes(2);
    expect(wrapper.find(".run-plan-edit-overlay").exists()).toBe(false);
    wrapper.unmount();
  });

  it("does not create a replacement when cancelling the pending plan fails", async () => {
    const pendingRun: Run = {
      ...run, state: "waiting_for_user", final_text: undefined,
      execution_plan: {
        id: "plan-1", state: "pending", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules",
        steps: [], resources: [], side_effects: [], reasons: ["workflow_execution"],
        estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      },
    };
    const decideRunExecutionPlan = vi.fn(async () => { throw new Error("conflict"); });
    const continueRunConversation = vi.fn();
    const api = apiStub({ listRuns: vi.fn(async () => [pendingRun]), listRunTurns: vi.fn(async () => [pendingRun]), decideRunExecutionPlan, continueRunConversation });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    const editButton = wrapper.findAll(".task-workspace-plan-actions .el-button").find((button) => button.text() === "修改要求");
    await editButton!.trigger("click");
    await flushPromises();
    await wrapper.get(".run-composer [aria-label='发送']").trigger("click");
    await flushPromises();

    expect(decideRunExecutionPlan).toHaveBeenCalledTimes(1);
    expect(continueRunConversation).not.toHaveBeenCalled();
    expect(wrapper.get(".run-plan-edit-overlay").text()).toContain("提交后");
    expect(wrapper.get(".run-composer .composer-editor").text()).toContain(workflow.goal);
    wrapper.unmount();
  });

  it("opens a pending plan at its start instead of scrolling past its heading", async () => {
    const scrollTo = vi.fn();
    const scrollToDescriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "scrollTo");
    const scrollHeightDescriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "scrollHeight");
    Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: scrollTo });
    Object.defineProperty(HTMLElement.prototype, "scrollHeight", { configurable: true, get: () => 1000 });
    try {
      const pendingRun: Run = {
        ...run, state: "waiting_for_user", final_text: undefined,
        execution_plan: {
          id: "plan-1", state: "pending", objective: workflow.goal, created_at: run.queued_at, version: 1, generator: "platform_rules",
          steps: [{ id: "step-1", kind: "execute_stage", label: "执行任务", position: 1, state: "pending" }],
          resources: [], side_effects: ["workspace_files_may_change"], reasons: ["workflow_execution"],
          estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
        },
      };
      const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [pendingRun]), listRunTurns: vi.fn(async () => [pendingRun]), getAttachmentDownload: vi.fn(async () => new Blob()) }));
      await wrapper.get(".run-row:not(.run-head)").trigger("click");
      await flushPromises();

      expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
      wrapper.unmount();
    } finally {
      if (scrollToDescriptor) Object.defineProperty(HTMLElement.prototype, "scrollTo", scrollToDescriptor);
      else Reflect.deleteProperty(HTMLElement.prototype, "scrollTo");
      if (scrollHeightDescriptor) Object.defineProperty(HTMLElement.prototype, "scrollHeight", scrollHeightDescriptor);
      else Reflect.deleteProperty(HTMLElement.prototype, "scrollHeight");
    }
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

  it("links a Run conversation to its adaptive task panel", async () => {
    const plannedRun: Run = { ...run, execution_plan: {
      id: "plan-1", state: "completed", objective: "审查发布风险", created_at: "2026-09-28T08:00:00Z", version: 1, generator: "platform_rules",
      steps: [{ id: "step-1", kind: "execute_stage", label: "检查变更", position: 1, state: "completed" }], resources: [], side_effects: [], reasons: [], estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
    } };
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [plannedRun]), listRunTurns: vi.fn(async () => [plannedRun]) }));
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".run-page").classes()).toContain("has-task-panel");
    expect(wrapper.get(".task-workspace-panel").text()).toContain("审查发布风险");
    await wrapper.get(".task-workspace-panel > header button").trigger("click");
    expect(wrapper.find(".task-workspace-panel").exists()).toBe(false);
    await wrapper.get('.run-conversation-head button[aria-label="打开任务面板"]').trigger("click");
    expect(wrapper.get(".task-workspace-panel").text()).toContain("检查变更");
    expect(wrapper.find('.run-conversation-head button[aria-label="打开任务面板"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("replays persisted Run activity into the shared conversation thread after completion", async () => {
    const streamRunEvents = vi.fn(async (_workflowID: string, _runID: string, onEvent: (event: RunEvent) => void) => {
      onEvent({ sequence: 1, type: "runtime.started", payload: { runtime: "codex" }, raw: "{}" });
      onEvent({ sequence: 2, type: "command.requested", payload: { command: "git status" }, raw: "{}" });
      onEvent({ sequence: 3, type: "command.completed", payload: { command: "git status", exit_code: 0 }, raw: "{}" });
      onEvent({ sequence: 4, type: "file.changed", payload: { path: "report.md" }, raw: "{}" });
    });
    const wrapper = await mountPage(apiStub({ streamRunEvents }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(streamRunEvents).toHaveBeenCalledWith("workflow-1", "run-1", expect.any(Function));
    expect(wrapper.get(".runtime-activity").text()).toContain("正在更新文件");
    expect(wrapper.get(".runtime-activity details").text()).toContain("运行环境已准备");
    expect(wrapper.get(".runtime-activity details").text()).not.toContain("Codex");
    expect(wrapper.get(".runtime-activity details").text()).toContain("git status");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项工具调用");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项文件变化");
    expect(wrapper.findAll(".activity-kind").map((item) => item.text())).toEqual(["环境", "工具", "文件"]);
    expect(wrapper.findAll(".activity-state").map((item) => item.text())).toEqual(["已完成", "已完成", "已完成"]);
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
    expect(wrapper.find(".execution-status-bar").exists()).toBe(false);
    expect(wrapper.get(".message-terminal-state.is-failed").text()).toBe("失败");
    expect(header).toContain(new Date(latestTurn.started_at!).toLocaleString());
    expect(header).not.toContain(new Date(run.started_at!).toLocaleString());
    wrapper.unmount();
  });

  it("renders a failed Run error once when its expert stage repeats it", async () => {
    const error = "command_failed: runtime command failed: PI Agent stopped with error: OpenAI API error (502)";
    const failedRun: Run = {
      ...run,
      state: "failed",
      final_text: undefined,
      error,
      expert_stages: [{
        expert_id: "expert-1",
        expert_name: "PI Agent",
        position: 1,
        total: 1,
        state: "failed",
        elapsed_ms: 27_000,
        error,
      }],
    };
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [failedRun]), listRunTurns: vi.fn(async () => [failedRun]) }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".failure-card").text()).toContain(error);
    expect(wrapper.findAll(".failure-card dd").filter((item) => item.text() === error)).toHaveLength(1);
    expect(wrapper.find(".expert-stage-list").exists()).toBe(false);
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

    const status = wrapper.get(".execution-status-bar").text();
    expect(status).toContain("2:00");
    expect(status).not.toContain("NaN");
    wrapper.unmount();
    vi.useRealTimers();
  });

  it("keeps files under their producing Run and ignores legacy final-result records", async () => {
    const legacyResult: Artifact = { id: "result-1", run_id: run.id, kind: "result", name: "Final result", path: "", size: 0, text_preview: "done", expired: false, created_at: run.ended_at! };
    const generatedFile: Artifact = { id: "file-1", run_id: run.id, kind: "file", name: "report.md", path: "report.md", size: 12, sha256: "abc", expired: false, created_at: run.ended_at! };
    const wrapper = await mountPage(apiStub({ listArtifacts: vi.fn(async () => [legacyResult, generatedFile]) }));

    expect(wrapper.findAll(".tabs button").map((item) => item.text())).toEqual(["概览", "运行记录", "工作空间", "设置"]);
    expect(wrapper.find(".artifact-list").exists()).toBe(false);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".artifact-disclosure-links button").map((item) => item.text())).toEqual(["查看所有产物 (1)", "查看所有变更 (1)"]);
    await wrapper.findAll(".artifact-disclosure-links button")[0]!.trigger("click");
    expect(wrapper.get(".generated-artifacts").text()).toContain("report.md");
    expect(wrapper.get(".generated-artifacts").text()).not.toContain("Final result");
    wrapper.unmount();
  });

  it("routes an old Artifacts tab link to Run History", async () => {
    const wrapper = await mountPage(apiStub(), `/workflows/${workflow.id}?tab=artifacts`);
    expect(wrapper.findAll(".tabs button").map((item) => item.text())).toEqual(["概览", "运行记录", "工作空间", "设置"]);
    expect(wrapper.findAll(".tabs button")[1]!.classes()).toContain("active");
    expect(wrapper.vm.$route.query.tab).toBe("history");
    wrapper.unmount();
  });

  it("downloads a Run Artifact from its conversation card", async () => {
    const generatedFile: Artifact = { id: "file-1", run_id: run.id, kind: "file", name: "Agent_Workspace_项目介绍.pptx", path: "Agent_Workspace_项目介绍.pptx", size: 1536, sha256: "abc", text_preview: "workflow report", expired: false, created_at: run.ended_at! };
    const artifactText = "PPT 已生成：\n\n[下载 Agent Workspace 项目介绍 PPT](Agent_Workspace_项目介绍.pptx)";
    const artifactRun = { ...run, final_text: artifactText, expert_stages: [{ expert_id: "expert-1", expert_name: "演示文稿专家", position: 1, total: 1, state: "succeeded" as const, elapsed_ms: 1000, final_text: artifactText }] };
    const getArtifactDownload = vi.fn(async () => new Blob(["workflow report"], { type: "application/octet-stream" }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const wrapper = await mountPage(apiStub({ listRuns: vi.fn(async () => [artifactRun]), listRunTurns: vi.fn(async () => [artifactRun]), listArtifacts: vi.fn(async () => [generatedFile]), getArtifactDownload }));

    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();
    const disclosureLinks = wrapper.findAll(".artifact-disclosure-links button");
    await disclosureLinks[0]!.trigger("click");
    expect(wrapper.get(".generated-artifact").text()).toContain("1.5 KB");
    expect(wrapper.get(".message.assistant .markdown-body").text()).toContain("Agent_Workspace_项目介绍.pptx");
    expect(wrapper.findAll(".message.assistant .markdown-body a")).toHaveLength(0);
    expect(wrapper.get(".message.assistant .markdown-body").text()).not.toContain("/workspace/");
    await disclosureLinks[1]!.trigger("click");
    expect(wrapper.find(".generated-artifact").exists()).toBe(false);
    expect(wrapper.get(".artifact-changes").text()).toContain("Agent_Workspace_项目介绍.pptx");
    expect(wrapper.get(".artifact-changes").text()).not.toContain("workflow report");
    await disclosureLinks[0]!.trigger("click");
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

  it("keeps Workflow setting sections summarized and collapsed by default", async () => {
    const wrapper = await mountPage();
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    const sections = wrapper.findAll(".settings-section");
    expect(sections).toHaveLength(6);
    expect(wrapper.find("#workflow-settings-channels").text()).toContain("消息渠道");
    expect(sections.every((section) => section.attributes("open") === undefined)).toBe(true);
    expect(wrapper.get("#workflow-settings-environment > summary").text()).toContain("0 个环境变量");
    expect(wrapper.find("#workflow-settings-resources").exists()).toBe(false);
    expect(wrapper.find("#workflow-settings-basic .knowledge-base-picker").exists()).toBe(true);
    expect(wrapper.get("#workflow-settings-environment").find(".knowledge-base-picker").exists()).toBe(false);
    expect(wrapper.find(".section-heading-actions").exists()).toBe(false);
    expect(wrapper.find(".danger-zone").exists()).toBe(false);
    const bottomActions = wrapper.get(".settings-actions-bottom");
    expect(bottomActions.find("button[type='submit']").exists()).toBe(true);
    expect(bottomActions.findAll("button")).toHaveLength(2);
    expect(bottomActions.findAll("button").map((button) => button.text())).toEqual(["删除", "保存"]);
    wrapper.unmount();
  });

  it("previews the next three Schedule times without saving", async () => {
    const previewWorkflowSchedule = vi.fn(async () => ["2026-09-30T01:00:00Z", "2026-10-01T01:00:00Z", "2026-10-02T01:00:00Z"]);
    const wrapper = await mountPage(apiStub({ previewWorkflowSchedule }));
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.get("#workflow-settings-schedule > .button").trigger("click");
    await wrapper.get(".schedule-preview .el-button").trigger("click");
    await flushPromises();

    expect(previewWorkflowSchedule).toHaveBeenCalledWith(expect.objectContaining({ frequency: "daily", timezone: "Asia/Shanghai" }));
    expect(wrapper.findAll(".schedule-preview li")).toHaveLength(3);
    wrapper.unmount();
  });

  it("lets a Workflow explicitly choose zero or more Knowledge Bases without a default", async () => {
    const knowledgeBases: KnowledgeBase[] = [
      { id: "kb-1", owner_id: "user-1", name: "产品资料", description: "", visibility: "private", platform: false, deleted: false, created_at: "2026-09-18T00:00:00Z", updated_at: "2026-09-18T00:00:00Z", version: 1, document_count: 2, ready_document_count: 2 },
      { id: "kb-2", owner_id: "user-1", name: "公开规范", description: "", visibility: "public", platform: true, deleted: false, created_at: "2026-09-18T00:00:00Z", updated_at: "2026-09-18T00:00:00Z", version: 1, document_count: 1, ready_document_count: 0 },
    ];
    const updateWorkflow = vi.fn(async (_id: string, input: Parameters<PlatformApi["updateWorkflow"]>[1], _version: number) => ({ ...workflow, ...input }));
    const api = apiStub({ listKnowledgeBases: vi.fn(async () => knowledgeBases), updateWorkflow });
    const wrapper = await mountPage(api);

    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    const choices = wrapper.get("#workflow-settings-basic").findAll<HTMLInputElement>(".knowledge-base-option input[type='checkbox']");
    expect(choices).toHaveLength(2);
    expect(choices.every((choice) => !choice.element.checked)).toBe(true);
    expect(choices[0]!.element.disabled).toBe(false);
    expect(choices[1]!.element.disabled).toBe(true);
    expect(wrapper.get(".knowledge-base-options").text()).toContain("2 份文档可检索");
    expect(wrapper.get(".knowledge-base-options").text()).toContain("暂无可检索文档");

    await choices[0]!.setValue(true);
    expect(choices[0]!.element.checked).toBe(true);
    await wrapper.get(".settings-form").trigger("submit");
    await flushPromises();
    expect(updateWorkflow).toHaveBeenCalledWith(workflow.id, expect.objectContaining({ knowledge_base_ids: ["kb-1"] }), workflow.version);
    await choices[0]!.setValue(false);
    await wrapper.get(".settings-form").trigger("submit");
    await flushPromises();
    expect(updateWorkflow).toHaveBeenLastCalledWith(workflow.id, expect.objectContaining({ knowledge_base_ids: [] }), workflow.version);
    wrapper.unmount();
  });

  it("shows copyable generated credentials and opens the integration guide", async () => {
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const api = apiStub({
      generateWorkflowCredential: vi.fn(async () => ({ api_key: "awk_test", api_secret: "aws_test", created_at: "2026-09-12T00:00:00Z" })),
    });
    const wrapper = await mountPage(api);
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.get(".api-credential-actions .button").trigger("click");
    await flushPromises();

    const rows = wrapper.findAll(".credential-row");
    expect(rows).toHaveLength(2);
    expect(wrapper.find(".secret-reveal > p").exists()).toBe(false);
    expect(wrapper.find(".credential-copy-all").exists()).toBe(false);
    expect(rows[0]!.text()).toContain("awk_test");
    expect(rows[1]!.text()).not.toContain("aws_test");
    expect(rows[1]!.text()).toContain("****");
    const secretActions = rows[1]!.findAll(".credential-icon-button");
    expect(secretActions).toHaveLength(2);
    await secretActions[0]!.trigger("click");
    expect(rows[1]!.text()).toContain("aws_test");
    await secretActions[1]!.trigger("click");
    expect(writeText).toHaveBeenCalledWith("aws_test");
    await rows[0]!.get(".credential-icon-button").trigger("click");
    expect(writeText).toHaveBeenCalledWith("awk_test");

    await wrapper.get(".api-credential-actions .button:nth-child(2)").trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.get("#workflow-settings-api").text()).toContain("API Key 和 API Secret 没有固定到期时间");
    const guide = wrapper.get(".integration-guide");
    expect(guide.text()).toContain("每次调用前自动换取 JWT");
    const commands = guide.findAll(".integration-code code").map((code) => code.text());
    expect(commands).toHaveLength(4);
    expect(commands[0]).toContain("workflow_request() {");
    expect(commands[0]).toContain("/api/v1/workflows/workflow-1/api-token");
    expect(commands[0]).toContain("jq -er '.jwt_token'");
    expect(commands[1]).toContain("RUN_ID=$(workflow_request");
    expect(commands[2]).toContain("workflow_request -N");
    expect(commands[2]).toContain("/api/v1/workflows/workflow-1/runs/$RUN_ID/events");
    expect(commands[3]).toContain("workflow_request");
    expect(commands[3]).toContain("/api/v1/workflows/workflow-1/runs/$RUN_ID");
    expect(commands.slice(1).join("\n")).not.toContain("$JWT_TOKEN");
    wrapper.unmount();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
  });

  it("loads and displays persisted credentials when reopening settings", async () => {
    const savedWorkflow = { ...workflow, api_credential_configured: true };
    const getWorkflowCredential = vi.fn(async () => ({ api_key: "awk_saved", api_secret: "aws_saved", created_at: "2026-09-12T00:00:00Z" }));
    const api = apiStub({ getWorkflow: vi.fn(async () => savedWorkflow), getWorkflowCredential });
    const wrapper = await mountPage(api);

    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await flushPromises();

    expect(getWorkflowCredential).toHaveBeenCalledWith(workflow.id);
    const rows = wrapper.findAll(".credential-row");
    expect(rows).toHaveLength(2);
    expect(rows[0]!.text()).toContain("awk_saved");
    expect(rows[1]!.text()).not.toContain("aws_saved");
    wrapper.unmount();
  });

  it("confirms credential rotation and can revoke the credential", async () => {
    const savedWorkflow = { ...workflow, api_credential_configured: true };
    const generateWorkflowCredential = vi.fn(async () => ({ api_key: "awk_new", api_secret: "aws_new", created_at: "2026-09-12T00:00:00Z" }));
    const revokeWorkflowCredential = vi.fn(async () => undefined);
    const wrapper = await mountPage(apiStub({
      getWorkflow: vi.fn(async () => savedWorkflow),
      getWorkflowCredential: vi.fn(async () => ({ api_key: "awk_saved", api_secret: "aws_saved" })),
      generateWorkflowCredential,
      revokeWorkflowCredential,
    }));
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await flushPromises();

    await wrapper.get(".api-credential-actions .button").trigger("click");
    expect(generateWorkflowCredential).not.toHaveBeenCalled();
    const dialogs = wrapper.findAllComponents(ConfirmDialog);
    const rotation = dialogs.find((dialog) => dialog.props("title") === "重新生成 API 凭证？");
    expect(rotation?.props("open")).toBe(true);
    rotation?.vm.$emit("confirm");
    await flushPromises();
    expect(generateWorkflowCredential).toHaveBeenCalledWith(workflow.id);

    await wrapper.get(".credential-revoke").trigger("click");
    const revocation = dialogs.find((dialog) => dialog.props("title") === "停用 API 凭证？");
    expect(revocation?.props("open")).toBe(true);
    revocation?.vm.$emit("confirm");
    await flushPromises();
    expect(revokeWorkflowCredential).toHaveBeenCalledWith(workflow.id);
    expect(wrapper.text()).toContain("API 凭证已停用");
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

  it("shows clone failures beside the Git action and preserves inputs for retry", async () => {
    const configureWorkflowGitSource = vi.fn<PlatformApi["configureWorkflowGitSource"]>()
      .mockRejectedValueOnce(new ApiError("validation", 422, "git_workspace_not_empty"))
      .mockResolvedValue(workflow);
    const wrapper = await mountPage(apiStub({ configureWorkflowGitSource }));
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    const url = wrapper.get<HTMLInputElement>('.git-settings input[placeholder*="git@github.com"]');
    await url.setValue("git@git.example.com:team/project.git");
    await wrapper.get(".git-settings .button.primary").trigger("click");
    await flushPromises();

    expect(wrapper.get('.git-settings [role="alert"]').text()).toContain("工作空间已有内容");
    expect(url.element.value).toBe("git@git.example.com:team/project.git");
    expect(wrapper.get(".git-settings .button.primary").attributes("disabled")).toBeUndefined();
    await wrapper.get(".git-settings .button.primary").trigger("click");
    await flushPromises();
    expect(wrapper.get(".git-feedback").text()).toContain("Git 仓库已克隆并保存");
    expect(wrapper.get(".git-feedback").text()).not.toContain("工作空间已有内容");
    wrapper.unmount();
  });

  it("submits a Workflow-scoped SSH config for private Git clone", async () => {
    const configureWorkflowGitSource = vi.fn(async () => workflow);
    const wrapper = await mountPage(apiStub({ configureWorkflowGitSource }));
    await wrapper.findAll(".tabs button").at(3)!.trigger("click");
    await wrapper.vm.$nextTick();

    await wrapper.get<HTMLSelectElement>('.git-settings select').setValue("ssh");
    await wrapper.get<HTMLInputElement>('.git-settings input[placeholder*="git@github.com"]').setValue("git-server:/srv/git/project.git");
    await wrapper.get<HTMLTextAreaElement>('textarea[name="ssh-config"]').setValue("Host git-server\n  HostName git.example.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n");
    await wrapper.get<HTMLTextAreaElement>('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').setValue("private-key");
    await wrapper.get(".git-settings .button.primary").trigger("click");
    await flushPromises();

    expect(configureWorkflowGitSource).toHaveBeenCalledWith(workflow.id, expect.objectContaining({
      ssh_config: expect.stringContaining("Host git-server"),
    }));
    wrapper.unmount();
  });

  it("replaces a saved SSH key input with a write-only status and only opens an empty replacement input", async () => {
    const saved: Workflow = { ...workflow, git_source: { url: "git@git.example.com:team/project.git", branch: "main", authentication: "ssh", config: [], credential_configured: true } };
    const configureWorkflowGitSource = vi.fn(async () => saved);
    const wrapper = await mountPage(apiStub({ configureWorkflowGitSource }));
    await wrapper.findAll(".tabs button")[3]!.trigger("click");
    await wrapper.get<HTMLSelectElement>('.git-settings select').setValue("ssh");
    await wrapper.get<HTMLTextAreaElement>('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').setValue("synthetic-private-key");
    await wrapper.get(".git-settings .button.primary").trigger("click"); await flushPromises();

    expect(wrapper.find('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').exists()).toBe(false);
    expect(wrapper.get(".git-credential-status").text()).toContain("已保存");
    expect(wrapper.html()).not.toContain("synthetic-private-key");
    await wrapper.get(".git-credential-status button").trigger("click");
    expect(wrapper.get<HTMLTextAreaElement>('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').element.value).toBe("");
    wrapper.unmount();

    const reopened = await mountPage(apiStub({ getWorkflow: vi.fn(async () => saved) }));
    await reopened.findAll(".tabs button")[3]!.trigger("click");
    expect(reopened.find('textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]').exists()).toBe(false);
    expect(reopened.get(".git-credential-status").text()).toContain("已保存");
    reopened.unmount();
  });

  it.each(["ssh", "basic"] as const)("keeps only an explicitly entered %s replacement after failure and clears it when authentication changes", async (authentication) => {
    const saved: Workflow = { ...workflow, git_source: { url: "https://git.example.com/team/project.git", branch: "main", authentication, config: [], credential_configured: true } };
    const wrapper = await mountPage(apiStub({ getWorkflow: vi.fn(async () => saved), configureWorkflowGitSource: vi.fn(async () => { throw new ApiError("validation", 422, "git_authentication_failed"); }) }));
    await wrapper.findAll(".tabs button")[3]!.trigger("click");
    const selector = authentication === "ssh" ? 'textarea[placeholder*="BEGIN OPENSSH PRIVATE KEY"]' : 'input[type="password"]';
    expect(wrapper.find(selector).exists()).toBe(false);
    await wrapper.get(".git-credential-status button").trigger("click");
    await wrapper.get(selector).setValue("new-credential-not-saved");
    await wrapper.get(".git-settings .button.primary").trigger("click"); await flushPromises();
    expect(wrapper.get<HTMLTextAreaElement | HTMLInputElement>(selector).element.value).toBe("new-credential-not-saved");
    expect(wrapper.get(".git-settings").text()).toContain("尚未保存");
    await wrapper.get<HTMLSelectElement>('.git-settings select').setValue("none");
    await wrapper.get<HTMLSelectElement>('.git-settings select').setValue(authentication);
    expect(wrapper.find(selector).exists()).toBe(false);
    await wrapper.get(".git-credential-status button").trigger("click");
    expect(wrapper.get<HTMLTextAreaElement | HTMLInputElement>(selector).element.value).toBe("");
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

    wrapper.get('.composer-editor').element.textContent = '继续给出修复建议';
    await wrapper.get('.composer-editor').trigger('input');
    await wrapper.get('.composer-toolbar button[aria-label="发送"]').trigger('click');
    await flushPromises();

    expect(continueRunConversation).toHaveBeenCalledWith(workflow.id, run.id, "继续给出修复建议", [], undefined, { selection_id: "selection-1", file_references: [] });
    expect(wrapper.findAll(".run-conversation .message.user")).toHaveLength(2);
    wrapper.unmount();
  });

  it("uses the Workflow conversation width for its composer", async () => {
    const wrapper = await mountPage();
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.find(".run-page > .run-composer-layer > .run-composer.resource-composer.composer").exists()).toBe(true);
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
    expect(wrapper.get(".runtime-activity details").text()).toContain("运行环境已准备");
    expect(wrapper.get(".runtime-activity details").text()).not.toContain("正在准备运行环境");
    const initiallyVisible = wrapper.get(".run-conversation .message.assistant .markdown-body").text();
    expect(initiallyVisible.length).toBeGreaterThan(0);
    expect(initiallyVisible.length).toBeLessThan(response.length);

    await vi.advanceTimersByTimeAsync(2_000);
    await flushPromises();
    expect(wrapper.get(".run-conversation .message.assistant .markdown-body").text()).toBe(response);
    wrapper.unmount();
    vi.useRealTimers();
  });

  it("resumes from the last event sequence and ignores replayed duplicates", async () => {
    vi.useFakeTimers();
    const activeRun: Run = { ...run, state: "running", final_text: undefined, ended_at: undefined, elapsed_ms: 0 };
    let streamCalls = 0;
    const streamRunEvents = vi.fn(async (_workflowID: string, _runID: string, onEvent: (event: RunEvent) => void, signal?: AbortSignal, options?: { afterSequence?: number; reconnect?: boolean }) => {
      streamCalls += 1;
      if (streamCalls === 1) {
        onEvent({ sequence: 1, type: "runtime.started", payload: { runtime: "codex" }, raw: "{}" });
        throw new Error("connection lost");
      }
      onEvent({ sequence: 1, type: "runtime.started", payload: { runtime: "codex" }, raw: "{}" });
      onEvent({ sequence: 2, type: "command.requested", payload: { command: "git status" }, raw: "{}" });
      expect(options).toEqual({ afterSequence: 1, reconnect: true });
      await new Promise<void>((resolve) => signal?.addEventListener("abort", () => resolve(), { once: true }));
    });
    const api = apiStub({
      listRuns: vi.fn(async () => [activeRun]),
      listRunTurns: vi.fn(async () => [activeRun]),
      getRun: vi.fn(async () => activeRun),
      streamRunEvents,
    });
    const wrapper = await mountPage(api);
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    await vi.advanceTimersByTimeAsync(750);
    await flushPromises();

    expect(streamRunEvents).toHaveBeenCalledTimes(2);
    expect(streamRunEvents.mock.calls[1]?.[4]).toEqual({ afterSequence: 1, reconnect: true });
    expect(wrapper.findAll(".activity-summary-group")).toHaveLength(2);
    wrapper.unmount();
  });
});
