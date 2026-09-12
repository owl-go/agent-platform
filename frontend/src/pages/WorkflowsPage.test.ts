// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi, type Workflow } from "../api/client";
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
    expect(wrapper.text()).not.toContain("已删除记录");
    expect(wrapper.text()).not.toContain("已删除工作流");
    wrapper.unmount();
  });
});
