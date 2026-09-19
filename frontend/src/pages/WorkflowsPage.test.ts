// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type KnowledgeBase, type PlatformApi, type Workflow } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import WorkflowsPage from "./WorkflowsPage.vue";

const workflow: Workflow = {
  id: "workflow-1",
  name: "每周报告",
  goal: "整理本周进展",
  environment: [],
  api_credential_configured: false,
  deleted: false,
  created_at: "2026-09-12T00:00:00Z",
  updated_at: "2026-09-12T00:00:00Z",
  version: 1,
};

describe("WorkflowsPage", () => {
  it("shows only active Workflows without a deleted-records entry", async () => {
    const listWorkflows = vi.fn(async (deleted = false) => deleted
      ? [{ ...workflow, id: "workflow-deleted", name: "已删除工作流", deleted: true }]
      : [workflow]);
    const api = {
      listWorkflows,
      listExperts: vi.fn(async () => []),
      listExpertTeams: vi.fn(async () => []),
    } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/workflows");
    const wrapper = mount(WorkflowsPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });
    await flushPromises();

    expect(listWorkflows).toHaveBeenCalledTimes(1);
    expect(listWorkflows).toHaveBeenCalledWith();
    expect(wrapper.text()).toContain("每周报告");
    expect(wrapper.get(".workflow-card h2").text()).toBe("每周报告");
    expect(wrapper.get(".workflow-created-at").text()).toContain("2026");
    expect(wrapper.text()).not.toContain("整理本周进展");
    expect(wrapper.find(".workflow-card .workflow-more").exists()).toBe(true);
    expect(wrapper.find(".workflow-card").text()).not.toContain("打开工作流");
    expect(wrapper.text()).not.toContain("已删除记录");
    expect(wrapper.text()).not.toContain("已删除工作流");
    wrapper.unmount();
  });

  it("offers optional Knowledge Base selection when creating a Workflow", async () => {
    const knowledgeBases: KnowledgeBase[] = [
      { id: "kb-1", owner_id: "user-1", name: "产品资料", description: "", visibility: "private", platform: false, deleted: false, created_at: "2026-09-12T00:00:00Z", updated_at: "2026-09-12T00:00:00Z", version: 1 },
      { id: "kb-2", owner_id: "user-1", name: "公开规范", description: "", visibility: "public", platform: true, deleted: false, created_at: "2026-09-12T00:00:00Z", updated_at: "2026-09-12T00:00:00Z", version: 1 },
    ];
    const createWorkflow = vi.fn(async (input: Parameters<PlatformApi["createWorkflow"]>[0]) => ({ ...workflow, ...input }));
    const api = {
      listWorkflows: vi.fn(async () => []),
      listExperts: vi.fn(async () => []),
      listExpertTeams: vi.fn(async () => []),
      listKnowledgeBases: vi.fn(async () => knowledgeBases),
      createWorkflow,
    } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/workflows");
    const wrapper = mount(WorkflowsPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });
    await flushPromises();

    await wrapper.get(".page-header .el-button").trigger("click");
    const dialog = wrapper.get(".el-dialog");
    await dialog.find("input").setValue("带资料的工作流");
    await dialog.find("textarea").setValue("根据资料回答问题");
    const choices = dialog.findAll<HTMLInputElement>(".knowledge-base-option input[type='checkbox']");
    expect(choices).toHaveLength(2);
    expect(choices.every((choice) => !choice.element.checked)).toBe(true);
    await choices[1]!.setValue(true);
    await dialog.get(".el-button--primary").trigger("click");
    await flushPromises();

    expect(createWorkflow).toHaveBeenCalledWith(expect.objectContaining({ knowledge_base_ids: ["kb-2"] }));
    wrapper.unmount();
  });

  it("does not open the workflow when the more menu is clicked", async () => {
    const api = {
      listWorkflows: vi.fn(async () => [workflow]),
      listExperts: vi.fn(async () => []),
      listExpertTeams: vi.fn(async () => []),
    } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/workflows");
    const push = vi.spyOn(router, "push");
    const wrapper = mount(WorkflowsPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });
    await flushPromises();

    wrapper.get(".workflow-more").element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();

    expect(push).not.toHaveBeenCalledWith("/workflows/workflow-1");
    expect(router.currentRoute.value.fullPath).toBe("/workflows");

    wrapper.get(".workflow-card").element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();

    expect(push).toHaveBeenCalledWith("/workflows/workflow-1");
    wrapper.unmount();
  });
});
