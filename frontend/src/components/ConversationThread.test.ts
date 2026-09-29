// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import type { ConversationMessage } from "../conversationThread";
import ConversationThread from "./ConversationThread.vue";

const stage = { expert_id: "expert-1", expert_name: "架构专家", provider_model_name: "Model", runtime_engine: "codex" as const, position: 1, total: 1, state: "succeeded" as const, elapsed_ms: 1200, final_text: "答案" };

function mountThread(messages: ConversationMessage[], locale: "zh-CN" | "en-US" = "zh-CN") {
  return mount(ConversationThread, {
    props: { messages, loadAttachment: vi.fn(async () => new Blob()) },
    global: { plugins: [createAppI18n({ getItem: () => locale }, locale)] },
  });
}

afterEach(() => vi.restoreAllMocks());

describe("ConversationThread", () => {
  it("renders Session and Workflow-shaped messages through the same transcript", () => {
    const wrapper = mountThread([
      { id: "user-1", role: "user", content: "用户原文", state: "succeeded", timestamp: "2026-08-25T12:00:00Z" },
      { id: "assistant-1", role: "assistant", content: "答案", state: "succeeded", timestamp: "2026-08-25T12:00:01Z", stages: [stage] },
    ]);

    expect(wrapper.findAll(".message")).toHaveLength(2);
    expect(wrapper.get(".message.user p").text()).toBe("用户原文");
    expect(wrapper.get(".message.assistant .markdown-body").text()).toBe("答案");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 个执行阶段");
    expect(wrapper.find(".expert-stage-list").exists()).toBe(false);
    wrapper.unmount();
  });

  it("keeps a non-duplicate stage expandable and copies its text", async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const wrapper = mountThread([{ id: "assistant-1", role: "assistant", content: "最终答案", state: "succeeded", timestamp: "2026-08-25T12:00:01Z", stages: [stage] }]);

    expect(wrapper.find(".expert-stage-list").exists()).toBe(true);
    await wrapper.get(".stage-copy").trigger("click");
    expect(writeText).toHaveBeenCalledWith("答案");
    expect(wrapper.get(".stage-copy").text()).toBe("已复制");
    wrapper.unmount();
  });

  it("does not render a failed stage twice when its error is the assistant error", () => {
    const error = "PI Agent stopped with error: OpenAI API error (502)";
    const wrapper = mountThread([{
      id: "assistant-1",
      role: "assistant",
      content: "",
      error,
      state: "failed",
      timestamp: "2026-08-25T12:00:01Z",
      stages: [{ ...stage, state: "failed", final_text: undefined, error }],
    }]);

    expect(wrapper.get(".failure-card").text()).toContain(error);
    expect(wrapper.findAll(".failure-card dd").filter((item) => item.text() === error)).toHaveLength(1);
    expect(wrapper.find(".expert-stage-list").exists()).toBe(false);
    wrapper.unmount();
  });

  it("does not expose runtime cancellation details as a failed response", () => {
    const internalCancellation = "context canceled\ninterrupted: runtime interrupted: context canceled\nruntime event stream closed without a terminal event";
    const wrapper = mountThread([{
      id: "assistant-1",
      role: "assistant",
      content: "",
      error: internalCancellation,
      state: "cancelled",
      timestamp: "2026-08-25T12:00:01Z",
    }]);

    expect(wrapper.get(".message.assistant").text()).toContain("已中止生成");
    expect(wrapper.get(".message.assistant").text()).not.toContain("runtime event stream closed");
    wrapper.unmount();
  });

  it("distinguishes Agent responses from user bubbles with an identity row", () => {
    const wrapper = mountThread([
      { id: "user-1", role: "user", content: "问题", state: "succeeded", timestamp: "2026-08-25T12:00:00Z" },
      { id: "assistant-1", role: "assistant", content: "回答", state: "succeeded", timestamp: "2026-08-25T12:00:01Z", meta: { label: "架构专家" } },
    ]);

    expect(wrapper.find(".message.user .agent-avatar").exists()).toBe(false);
    expect(wrapper.get(".message.assistant .agent-avatar").text()).toBe("AI");
    expect(wrapper.get(".message-identity").text()).toContain("架构专家");
    expect(wrapper.get(".message-identity").text()).toContain("Agent");
    wrapper.unmount();
  });

  it("summarizes platform-recorded execution evidence without claiming unavailable sources", () => {
    const wrapper = mountThread([{
      id: "assistant-1",
      role: "assistant",
      content: "已完成",
      state: "succeeded",
      timestamp: "2026-08-25T12:00:01Z",
      activities: [
        { id: "tool", kind: "tool", toolCallCount: 1, label: "已调用连接器", state: "completed", items: [{ id: 1, label: "连接器调用完成" }] },
        { id: "file", kind: "file", fileChangeCount: 1, label: "已更新文件", state: "completed", items: [{ id: 2, label: "文件更新完成" }] },
      ],
      stages: [stage],
      artifacts: [{ id: "artifact-1", kind: "file", name: "report.md", path: "report.md", size: 42, expired: false, created_at: "2026-08-25T12:00:01Z" }],
    }]);

    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("本次执行");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项工具调用");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项文件变化");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 个执行阶段");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 个产物");
    expect(wrapper.findAll(".activity-kind").map((item) => item.text())).toEqual(["工具", "文件"]);
    expect(wrapper.findAll(".activity-state").map((item) => item.text())).toEqual(["已完成", "已完成"]);
    expect(wrapper.text()).not.toContain("打开来源");
    wrapper.unmount();
  });

  it("states only that no external activity was recorded", () => {
    const wrapper = mountThread([{
      id: "assistant-1",
      role: "assistant",
      content: "回答",
      state: "succeeded",
      timestamp: "2026-08-25T12:00:01Z",
      activities: [{ id: "runtime", kind: "runtime", label: "运行环境已准备", state: "completed", items: [{ id: 1, label: "运行环境已准备" }] }],
    }]);

    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("未记录外部工具、文件变化或依据");
    expect(wrapper.get(".runtime-activity-history > summary").text()).not.toContain("未使用知识库");
    wrapper.unmount();
  });

  it("renders execution evidence labels in English", () => {
    const wrapper = mountThread([{
      id: "assistant-1",
      role: "assistant",
      content: "Done",
      state: "succeeded",
      timestamp: "2026-08-25T12:00:01Z",
      activities: [{ id: "tool", kind: "tool", toolCallCount: 1, label: "Tool completed", state: "completed", items: [{ id: 1, label: "Tool completed" }] }],
    }], "en-US");

    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("This execution");
    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 tool call");
    expect(wrapper.get(".activity-kind").text()).toBe("Tool");
    wrapper.unmount();
  });

  it("shows verified source states and only opens a succeeded Knowledge citation", async () => {
    const wrapper = mountThread([{
      id: "assistant-1", role: "assistant", content: "回答", state: "succeeded", timestamp: "2026-08-25T12:00:01Z",
      evidence: [
        { id: "knowledge-1", kind: "knowledge", source_id: "document-1", source_name: "制度.pdf", container_id: "base-1", state: "succeeded", action: "retrieved", stage_position: 1, citation: { revision_id: "revision-1", source_location: "第 2 页", relevance: .91 } },
        { id: "connector-1", kind: "connector", source_id: "crm", source_name: "CRM", state: "failed", action: "contact.search", stage_position: 1 },
        { id: "connector-2", kind: "connector", source_id: "mail", source_name: "邮箱", state: "not_used", action: "selected", stage_position: 1 },
      ],
    }]);

    expect(wrapper.get(".runtime-activity-history > summary").text()).toContain("1 项依据");
    expect(wrapper.findAll(".execution-source")).toHaveLength(3);
    expect(wrapper.text()).toContain("已使用");
    expect(wrapper.text()).toContain("调用失败");
    expect(wrapper.text()).toContain("未采用");
    expect(wrapper.findAll(".execution-source .text-button")).toHaveLength(1);
    await wrapper.get(".execution-source .text-button").trigger("click");
    expect(wrapper.emitted("openEvidence")?.[0]?.[0]).toMatchObject({ id: "knowledge-1", source_id: "document-1" });
    wrapper.unmount();
  });

  it("shows a pending execution plan and emits only allowed decisions", async () => {
    const wrapper = mountThread([{
      id: "assistant-plan", role: "assistant", content: "", state: "waiting_for_user", timestamp: "2026-09-28T08:00:00Z",
      executionPlan: {
        id: "plan-1", state: "pending", objective: "更新 CRM 中的客户记录", created_at: "2026-09-28T08:00:00Z", version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", label: "销售运营", position: 1, state: "pending" }],
        resources: [{ kind: "connector", id: "crm", name: "CRM" }], side_effects: ["external_connector_operation"], reasons: ["external_side_effect"],
        estimated_model_calls: 1, estimated_credit_hundredths: 125, generation_credit_hundredths: 0,
      },
    }]);

    expect(wrapper.get(".execution-plan-card").text()).toContain("更新 CRM 中的客户记录");
    expect(wrapper.get(".execution-plan-card").text()).toContain("最多约 1.25 Credits");
    expect(wrapper.get(".execution-plan-card").text()).toContain("平台规则生成 · 0.00 Credits");
    expect(wrapper.text()).not.toContain("直接回答，不执行外部操作");
    await wrapper.get(".execution-plan-card footer .el-button--primary").trigger("click");
    expect(wrapper.emitted("planDecision")?.[0]).toEqual(["assistant-plan", "start"]);
    wrapper.unmount();
  });

  it("renders a plan when empty repeated fields are omitted by the API", async () => {
    const wrapper = mountThread([{
      id: "assistant-plan", role: "assistant", content: "", state: "waiting_for_user", timestamp: "2026-09-28T08:00:00Z",
      executionPlan: {
        id: "plan-1", state: "pending", objective: "解释设计", created_at: "2026-09-28T08:00:00Z", version: 1, generator: "platform_rules",
        steps: [{ id: "step-1", kind: "execute_stage", position: 1, state: "pending" }],
        estimated_model_calls: 1, estimated_credit_hundredths: 100,
      } as NonNullable<ConversationMessage["executionPlan"]>,
    }]);

    expect(wrapper.get(".execution-plan-card").text()).toContain("解释设计");
    expect(wrapper.get(".execution-plan-steps").text()).toContain("执行任务");
    expect(wrapper.get(".execution-plan-card").text()).not.toContain("NaN");
    expect(wrapper.get(".execution-plan-card").text()).toContain("0.00 Credits");
    await wrapper.get(".execution-plan-card footer .el-button--primary").trigger("click");
    expect(wrapper.emitted("planDecision")?.[0]).toEqual(["assistant-plan", "start"]);
    wrapper.unmount();
  });

  it("shows model-generated task labels and the charged Plan generation cost", () => {
    const wrapper = mountThread([{
      id: "assistant-plan", role: "assistant", content: "", state: "waiting_for_user", timestamp: "2026-09-28T08:00:00Z",
      executionPlan: {
        id: "plan-1", state: "pending", objective: "分析项目代码", created_at: "2026-09-28T08:00:00Z", version: 2, generator: "model",
        steps: [
          { id: "step-1", kind: "review_input", label: "确认代码分析范围和入口", position: 1, state: "pending" },
          { id: "step-2", kind: "execute_stage", label: "梳理目录并追踪核心调用链", position: 2, state: "pending" },
          { id: "step-3", kind: "deliver_result", label: "汇总鉴权风险与建议", position: 3, state: "pending" },
        ],
        resources: [], side_effects: [], reasons: ["user_requested"], estimated_model_calls: 1,
        estimated_credit_hundredths: 100, generation_credit_hundredths: 37,
      },
    }]);
    expect(wrapper.get(".execution-plan-steps").text()).toContain("梳理目录并追踪核心调用链");
    expect(wrapper.get(".execution-plan-card").text()).toContain("模型生成 · 0.37 Credits");
    wrapper.unmount();
  });

  it("marks a failed detailed Plan generation as a rule-based fallback", () => {
    const wrapper = mountThread([{
      id: "assistant-plan", role: "assistant", content: "", state: "waiting_for_user", timestamp: "2026-09-28T08:00:00Z",
      executionPlan: {
        id: "plan-1", state: "pending", objective: "分析项目代码", created_at: "2026-09-28T08:00:00Z", version: 2, generator: "model_failed",
        steps: [{ id: "step-1", kind: "execute_stage", label: "", position: 1, state: "pending" }],
        resources: [], side_effects: [], reasons: ["user_requested"], estimated_model_calls: 1,
        estimated_credit_hundredths: 100, generation_credit_hundredths: 0,
      },
    }]);
    expect(wrapper.get(".execution-plan-card [role='status']").text()).toContain("详细计划生成失败");
    wrapper.unmount();
  });

  it("opens the task panel from a task-bearing historical response", async () => {
    const wrapper = mountThread([{
      id: "assistant-task", role: "assistant", content: "Done", state: "succeeded", timestamp: "2026-09-28T08:00:00Z",
      artifacts: [{ id: "artifact-1", kind: "file", name: "report.md", path: "report.md", size: 42, expired: false, created_at: "2026-09-28T08:01:00Z" }],
    }]);
    await wrapper.setProps({ selectedTaskId: "assistant-task" });

    expect(wrapper.get(".message.assistant").classes()).toContain("is-task-selected");
    expect(wrapper.get(".message-task").attributes("aria-pressed")).toBe("true");
    await wrapper.get(".message-task").trigger("click");
    expect(wrapper.emitted("selectTask")?.[0]).toEqual(["assistant-task"]);
    wrapper.unmount();
  });
});
