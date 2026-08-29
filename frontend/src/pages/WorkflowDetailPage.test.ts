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

function apiStub(): PlatformApi {
  return {
    getWorkflow: vi.fn(async () => workflow),
    listExperts: vi.fn(async () => []),
    listModelProviderConnections: vi.fn(async () => []),
    listRuntimeEngines: vi.fn(async () => []),
    listRuns: vi.fn(async () => [run]),
    listArtifacts: vi.fn(async () => []),
    listWorkspace: vi.fn(async () => ({ items: [], used_bytes: 0, limit_bytes: 1024 })),
    streamRunEvents: vi.fn(async (_workflowID, _runID, onEvent) => {
      onEvent({ sequence: 1, type: "message.delta", payload: { delta: "分析结果" }, raw: "{}" });
    }),
  } as unknown as PlatformApi;
}

async function mountPage() {
  const router = createAppRouter(createMemoryHistory());
  await router.push(`/workflows/${workflow.id}?tab=history`);
  await router.isReady();
  const wrapper = mount(WorkflowDetailPage, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: apiStub() },
    },
  });
  await flushPromises();
  return wrapper;
}

describe("WorkflowDetailPage", () => {
  it("opens a Run as a conversation instead of raw Runtime events", async () => {
    const wrapper = await mountPage();
    await wrapper.get(".run-row:not(.run-head)").trigger("click");
    await flushPromises();

    expect(wrapper.get(".run-conversation .message.user").text()).toContain("重点关注发布风险");
    expect(wrapper.get(".run-conversation .message.assistant .markdown-body h2").text()).toBe("分析结果");
    expect(wrapper.text()).not.toContain("message.delta");
    expect(wrapper.text()).not.toContain("工作流快照");
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
});
