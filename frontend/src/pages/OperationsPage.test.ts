import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type PlatformApi, type Run } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import OperationsPage from "./OperationsPage.vue";

const run = {
  id: "00000000-0000-4000-8000-000000000101", session_id: "session-1", coding_task_id: "00000000-0000-4000-8000-000000000201",
  agent_id: "00000000-0000-4000-8000-000000000301", agent_release_id: "release-1", runtime_image_id: "runtime-1",
  repository_binding_id: "binding-1", state: "completed", attempt_count: 1, created_at: "2026-08-20T00:00:00Z", version: 2,
  runtime_image_snapshot: { runtime: "codex", cli_version: "0.1.0", image_digest: "registry.example/codex@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" },
  repository_binding_snapshot: { name: "Payments API" }, configured_model_snapshot: { name: "Primary", model_id: "gpt-enterprise" },
  model_budget: { max_input_tokens: 1000 }, usage: { input_tokens: 100 }, cost_amount: "0.25", execution_limits: { cpus: 2 },
  attempts: [{ id: "attempt-1", number: 1, worker_id: "worker-1", state: "completed", infrastructure_failure: false, started_at: "2026-08-20T00:00:01Z", ended_at: "2026-08-20T00:01:00Z" }],
} as unknown as Run;

describe("Operations Console", () => {
  it("restores compound filters, sort, page, and selected Run from the URL", async () => {
    const api = platformApi();
    const { wrapper } = await mountOperations(api, `/operations?team=team-1&agent=${run.agent_id}&state=completed&runtime=codex&sort=asc&page=cursor-2&run=${run.id}`);
    expect(api.searchRuns).toHaveBeenCalledWith(expect.objectContaining({ teamID: "team-1", agentID: run.agent_id, state: "completed", runtime: "codex", sortDirection: "asc", pageToken: "cursor-2", limit: 25 }));
    expect(api.getRun).toHaveBeenCalledWith(run.id, expect.any(AbortSignal));
    expect(wrapper.text()).toContain("registry.example/codex@sha256");
    expect(wrapper.text()).toContain("gpt-enterprise");
    expect(wrapper.text()).toContain("Payments API");
  });

  it("writes filters and pagination back to the shareable URL", async () => {
    const api = platformApi({ searchRuns: vi.fn(async () => ({ items: [run], nextPageToken: "next-cursor" })) });
    const { wrapper, router } = await mountOperations(api);
    await wrapper.get("[data-testid='filter-state']").setValue("failed");
    await wrapper.get("[data-testid='filter-runtime']").setValue("hermes");
    await wrapper.get("[data-testid='filter-sort']").setValue("asc");
    await wrapper.get("[data-testid='operations-filters']").trigger("submit");
    await flushPromises();
    expect(router.currentRoute.value.query).toMatchObject({ team: "team-1", state: "failed", runtime: "hermes", sort: "asc" });
    await wrapper.get("[data-testid='next-page']").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.query.page).toBe("next-cursor");
  });

  it("distinguishes empty, permission, rate-limit, and server result states", async () => {
    const empty = await mountOperations(platformApi({ searchRuns: vi.fn(async () => ({ items: [], nextPageToken: "" })) }));
    expect(empty.wrapper.get("[data-testid='operations-empty']").text()).toContain("没有 Run");
    empty.wrapper.unmount();
    for (const [kind, code] of [["forbidden", "run_search_denied"], ["rate_limited", "rate_limited"], ["unavailable", "run_search_failed"]] as const) {
      const mounted = await mountOperations(platformApi({ searchRuns: vi.fn(async () => { throw new ApiError(kind, kind === "forbidden" ? 403 : kind === "rate_limited" ? 429 : 503, code, "request-1"); }) }));
      expect(mounted.wrapper.get("[role='alert']").text()).toContain(code);
      mounted.wrapper.unmount();
    }
  });
});

function platformApi(overrides: Partial<PlatformApi> = {}) {
  return {
    searchRuns: vi.fn(async () => ({ items: [run], nextPageToken: "" })),
    getRun: vi.fn(async () => run), listRunArtifacts: vi.fn(async () => [{ id: "artifact-1", kind: "diff", sha256: "b".repeat(64), size_bytes: "12", content_type: "text/plain" }]),
    getCodingTask: vi.fn(async () => ({ id: run.coding_task_id, title: "Repair parser", state: "active" })),
    streamRunEvents: vi.fn(async (_id: string, _after: number, onEvent: (event: never) => void) => { onEvent({ run_id: run.id, sequence: 1, event_type: "run.completed", payload: {}, created_at: "2026-08-20T00:01:00Z" } as never); return { cursor: 1, terminal: true }; }),
    ...overrides,
  } as unknown as PlatformApi;
}

async function mountOperations(api: PlatformApi, path = "/operations?team=team-1") {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path); await router.isReady();
  const wrapper = mount(OperationsPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
  await flushPromises();
  return { wrapper, router };
}
