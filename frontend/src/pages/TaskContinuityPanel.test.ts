import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import TaskContinuityPanel from "./TaskContinuityPanel.vue";

const task = { id: "task-1", team_id: "team-1", agent_release_id: "release-1", title: "Parser", request_text: "Fix it", state: "waiting_for_user", version: 3 };
const session = { id: "session-1", coding_task_id: "task-1", repository_binding_id: "binding-1", target_branch: "main", review_branch: "agent/task-1", run_count: 1, version: 4, memory: { summary: "First run", confirmed_decisions: ["Keep API"], results: [], workspace_snapshots: [] } };

describe("TaskContinuityPanel", () => {
  it("loads persisted continuity records and creates the next Run in the same Session", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api);
    await flushPromises();

    expect(api.listSessionMessages).toHaveBeenCalledWith("task-1", "team-1", 0);
    expect(api.listMemoryCandidates).toHaveBeenCalledWith("task-1", "team-1");
    expect(api.listAgentMemories).toHaveBeenCalledWith("agent-1", "team-1");
    expect(wrapper.text()).toContain("First run request");
    expect(wrapper.text()).toContain("workflow:small-focused-changes");
    expect((wrapper.get(".agent-memory-list select").element as HTMLSelectElement).value).toBe("quality-gate:test:unit");

    await wrapper.get("[data-testid='continue-text']").setValue("Add the regression test");
    expect(wrapper.get("[data-testid='continue-task']").attributes("disabled")).toBeUndefined();
    await wrapper.get(".continuation-form").trigger("submit");
    await flushPromises();

    expect(api.continueCodingTask).toHaveBeenCalledWith("task-1", "team-1", "Add the regression test", 3, 4, expect.any(String));
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  it("makes both candidate decisions explicit and only asks the API to approve the chosen candidate", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api);
    await flushPromises();

    await wrapper.get("[data-testid='approve-memory-candidate-1']").trigger("click");
    await flushPromises();

    expect(api.decideMemoryCandidate).toHaveBeenCalledWith("candidate-1", "team-1", true, expect.any(String));
  });

  it("uses Version-protected human task completion instead of deriving closure from a Run", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api);
    await flushPromises();

    await wrapper.get("[data-testid='complete-task']").trigger("click");
    await flushPromises();

    expect(api.updateCodingTaskState).toHaveBeenCalledWith("task-1", "team-1", "completed", 3, expect.any(String));
  });

  it("reloads the current Agent Memory and rotates intent after a Version conflict", async () => {
    const api = apiStub();
    vi.mocked(api.listAgentMemories).mockResolvedValueOnce([{ id: "memory-1", agent_id: "agent-1", content: "quality-gate:test:unit", enabled: true, version: 1 }])
      .mockResolvedValue([{ id: "memory-1", agent_id: "agent-1", content: "quality-gate:test:integration", enabled: true, version: 2 }]);
    vi.mocked(api.updateAgentMemory).mockRejectedValueOnce(new ApiError("conflict", 412, "version_conflict", ""))
      .mockResolvedValueOnce({ id: "memory-1", version: 3 } as never);
    const wrapper = mountPanel(api);
    await flushPromises();

    await wrapper.get("[data-testid='agent-memory-content-memory-1']").setValue("workflow:tests-before-commit");
    await wrapper.get("[data-testid='save-agent-memory-memory-1']").trigger("click");
    await flushPromises();
    const firstKey = vi.mocked(api.updateAgentMemory).mock.calls[0]![5];
    expect(api.listAgentMemories).toHaveBeenCalledTimes(2);
    expect((wrapper.get("[data-testid='agent-memory-content-memory-1']").element as HTMLSelectElement).value).toBe("quality-gate:test:integration");

    await wrapper.get("[data-testid='agent-memory-content-memory-1']").setValue("workflow:tests-before-commit");
    await wrapper.get("[data-testid='save-agent-memory-memory-1']").trigger("click");
    await flushPromises();
    expect(vi.mocked(api.updateAgentMemory).mock.calls[1]![4]).toBe(2);
    expect(vi.mocked(api.updateAgentMemory).mock.calls[1]![5]).not.toBe(firstKey);
  });
});

function mountPanel(api: PlatformApi) {
  return mount(TaskContinuityPanel, {
    props: { task, session, teamId: "team-1", agentId: "agent-1", canUse: true },
    global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } },
  });
}

function apiStub(): PlatformApi {
  return {
    listSessionMessages: vi.fn(async () => [{ id: 1, run_id: "run-1", author: "user", content: { type: "instruction", text: "First run request" } }]),
    listMemoryCandidates: vi.fn(async () => [{ id: "candidate-1", agent_id: "agent-1", coding_task_id: "task-1", proposed_content: "workflow:small-focused-changes", state: "pending" }]),
    listAgentMemories: vi.fn(async () => [{ id: "memory-1", agent_id: "agent-1", content: "quality-gate:test:unit", approved_by: "user-1", enabled: true, version: 1 }]),
    continueCodingTask: vi.fn(async () => ({ task: { ...task, state: "active", version: 4 }, session: { ...session, run_count: 2, version: 5 }, run_id: "run-2" })),
    updateSessionMemory: vi.fn(async () => session), decideMemoryCandidate: vi.fn(async () => ({})), updateAgentMemory: vi.fn(), deleteAgentMemory: vi.fn(), updateCodingTaskState: vi.fn(async () => ({ ...task, state: "completed", version: 4 })),
  } as unknown as PlatformApi;
}
