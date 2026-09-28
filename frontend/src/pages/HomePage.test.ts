// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type HomeOverview, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import HomePage from "./HomePage.vue";

const overview: HomeOverview = {
  action_items: [{ kind: "approval", id: "approval-1", execution_kind: "run", execution_id: "run-1", parent_id: "workflow-1", title: "飞书任务", state: "pending", created_at: "2026-09-28T08:00:00Z" }],
  recent_tasks: [{ kind: "session", id: "session-1", parent_id: "", title: "季度复盘", state: "completed", updated_at: "2026-09-28T07:00:00Z" }],
  common_workflows: [{ id: "workflow-1", name: "周报整理", run_count: 7, updated_at: "2026-09-28T06:00:00Z" }],
};

async function mountHome(item: HomeOverview = overview) {
  const router = createAppRouter(createMemoryHistory());
  await router.push("/home");
  const getHomeOverview = vi.fn(async () => item);
  const wrapper = mount(HomePage, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: { getHomeOverview } as unknown as PlatformApi },
    },
  });
  await flushPromises();
  return { wrapper, router, getHomeOverview };
}

describe("HomePage", () => {
  it("shows metadata-only actions, recent tasks, and common Workflows", async () => {
    const { wrapper, getHomeOverview } = await mountHome();

    expect(getHomeOverview).toHaveBeenCalledOnce();
    expect(wrapper.text()).toContain("等待审批");
    expect(wrapper.text()).toContain("季度复盘");
    expect(wrapper.text()).toContain("周报整理");
    expect(wrapper.text()).toContain("运行 7 次");
    expect(wrapper.text()).not.toContain("private prompt and answer");
    wrapper.unmount();
  });

  it("opens an action in its original Run", async () => {
    const { wrapper, router } = await mountHome();
    const push = vi.spyOn(router, "push").mockResolvedValue();

    await wrapper.get(".home-action-row").trigger("click");
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: "/workflows/workflow-1", query: { tab: "history", open_run: "run-1" } });
    wrapper.unmount();
  });

  it("renders an explicit empty state", async () => {
    const { wrapper } = await mountHome({ action_items: [], recent_tasks: [], common_workflows: [] });

    expect(wrapper.text()).toContain("还没有任务或工作流");
    wrapper.unmount();
  });
});
