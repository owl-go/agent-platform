// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type PlatformApi, type Run, type Workflow } from "../api/client";
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
  it("does not repeat the active tab label as a section title", async () => {
    const wrapper = await mountPage();

    for (const tabButton of wrapper.findAll(".tabs button")) {
      await tabButton.trigger("click");
      await wrapper.vm.$nextTick();
      expect(wrapper.find(".tab-content .section-heading h2").exists()).toBe(false);
    }
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
    expect(wrapper.text()).not.toContain("message.delta");
    expect(wrapper.text()).not.toContain("工作流快照");
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

    expect(continueRunConversation).toHaveBeenCalledWith(workflow.id, run.id, "继续给出修复建议");
    expect(wrapper.findAll(".run-conversation .message.user")).toHaveLength(2);
    wrapper.unmount();
  });
});
