// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import type { ConversationMessage } from "../conversationThread";
import TaskWorkspacePanel from "./TaskWorkspacePanel.vue";

const taskMessage: ConversationMessage = {
  id: "assistant-1",
  role: "assistant",
  content: "Report ready",
  state: "succeeded",
  timestamp: "2026-09-28T08:00:00Z",
  elapsedMs: 2400,
  executionPlan: {
    id: "plan-1", state: "completed", objective: "Prepare the release report", created_at: "2026-09-28T08:00:00Z", version: 1, generator: "platform_rules",
    steps: [{ id: "step-1", kind: "execute_stage", label: "Inspect changes", position: 1, state: "completed" }], resources: [], side_effects: [], reasons: [], estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
  },
  activities: [{ id: 1, label: "Read repository", state: "completed", items: [] }],
  evidence: [{ id: "evidence-1", kind: "knowledge", source_id: "source-1", source_name: "Release policy", container_id: "base-1", state: "succeeded", action: "retrieved indexed source", stage_position: 1, citation: { revision_id: "revision-1", source_location: "Page 2" } }],
  attachments: [{ id: "attachment-1", name: "input.csv", content_type: "text/csv", size: 12, sha256: "digest", image: false }],
  artifacts: [{ id: "artifact-1", kind: "file", name: "report.md", path: "report.md", size: 42, expired: false, created_at: "2026-09-28T08:01:00Z" }],
};

describe("TaskWorkspacePanel", () => {
  it("shows failed Connector calls even when the response completed", () => {
    const wrapper = mount(TaskWorkspacePanel, {
      props: { message: { ...taskMessage, state: "completed", evidence: [{ id: "notion-call", kind: "connector", source_id: "notion", source_name: "Notion", state: "failed", action: "identity", stage_position: 1 }] }, loadAttachment: vi.fn(async () => new Blob()) },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });
    expect(wrapper.get(".task-workspace-result").text()).toContain("已完成，有调用失败");
    expect(wrapper.get(".task-workspace-result").text()).not.toContain("成功");
    wrapper.unmount();
  });
  it("lets the user confirm a pending plan from the task panel", async () => {
    const wrapper = mount(TaskWorkspacePanel, {
      props: {
        message: { ...taskMessage, state: "waiting_for_user", executionPlan: { ...taskMessage.executionPlan!, state: "pending", side_effects: ["workspace_files_may_change"] } },
        loadAttachment: vi.fn(async () => new Blob()),
      },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    await wrapper.get(".task-workspace-plan-actions .el-button--primary").trigger("click");
    expect(wrapper.emitted("planDecision")?.[0]).toEqual(["assistant-1", "start"]);
    expect(wrapper.get(".task-workspace-plan-actions").text()).not.toContain("直接回答");
    wrapper.unmount();
  });

  it("does not display NaN when consumption has no usable total", () => {
    const wrapper = mount(TaskWorkspacePanel, {
      props: { message: { ...taskMessage, creditConsumption: { total_hundredths: Number.NaN, stages: [] } }, loadAttachment: vi.fn(async () => new Blob()) },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });
    expect(wrapper.text()).not.toContain("NaN");
    expect(wrapper.text()).not.toContain("消耗积分");
    wrapper.unmount();
  });

  it("groups plan, evidence, files, and result without inventing content", async () => {
    const wrapper = mount(TaskWorkspacePanel, {
      props: { message: taskMessage, loadAttachment: vi.fn(async () => new Blob()) },
      global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")] },
    });

    expect(wrapper.text()).toContain("Prepare the release report");
    expect(wrapper.text()).toContain("Release policy");
    expect(wrapper.text()).toContain("input.csv");
    expect(wrapper.text()).toContain("report.md");
    expect(wrapper.text()).toContain("Succeeded");
    await wrapper.get(".task-workspace-sources button").trigger("click");
    expect(wrapper.emitted("openEvidence")?.[0]?.[0]).toMatchObject({ id: "evidence-1" });
    await wrapper.findAll(".task-workspace-files button")[1]!.trigger("click");
    expect(wrapper.emitted("downloadArtifact")?.[0]?.[0]).toMatchObject({ id: "artifact-1" });
    await wrapper.get(".task-workspace-panel > header button").trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
    wrapper.unmount();
  });
});
