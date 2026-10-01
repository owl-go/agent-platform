// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import { platformApiKey, type PlatformApi } from "./api/client";
import { authContextKey, type AuthContext, type AuthState } from "./auth/session";
import { createAppI18n } from "./i18n";
import { createAppRouter } from "./router";

vi.mock("./api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api/client")>();
  return { ...actual, getHealth: vi.fn(async () => ({ status: "ok" })) };
});

function authContext(): AuthContext {
  const state = ref<AuthState>({
    kind: "authenticated",
    currentUser: {
      id: "user-1",
      username: "tester",
      email: "tester@example.com",
      display_name: "Test User",
      administrator: false,
      settings_ready: true,
    },
  });
  return {
    isCallback: false,
    session: {
      state,
      accessToken: () => "token",
      initialize: vi.fn(async () => {}),
      signIn: vi.fn(async () => {}),
      signOut: vi.fn(async () => {}),
      dispose: vi.fn(),
    },
  };
}

const defaultApi = { listImageGenerations: vi.fn(async () => []) } as unknown as PlatformApi;

async function mountAt(path: string, api: PlatformApi = defaultApi) {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path);
  await router.isReady();
  const wrapper = mount(App, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [authContextKey as symbol]: authContext(), [platformApiKey as symbol]: api },
      stubs: { RouterView: true },
    },
  });
  await flushPromises();
  return wrapper;
}

afterEach(() => vi.useRealTimers());

describe("App navigation", () => {
  it("groups primary navigation by product area", async () => {
    const wrapper = await mountAt("/sessions");

    expect(wrapper.findAll(".nav-group h2").map((heading) => heading.text())).toEqual(["工作区", "资源中心", "系统"]);
    expect(wrapper.findAll(".nav-group").map((group) => group.findAll("a").map((link) => link.attributes("href")))).toEqual([
      ["/home", "/sessions", "/workflows", "/ai-apps/assistants", "/ai-apps/image-creation"],
      ["/resources", "/resources?tab=skills", "/resources?tab=connectors", "/resources?tab=knowledge"],
      ["/settings"],
    ]);
    const aiApplicationsNav = wrapper.get('[data-nav-id="ai-applications"]');
    expect(aiApplicationsNav.text()).toContain("AI 应用");
    expect(aiApplicationsNav.attributes("aria-expanded")).toBe("true");
    expect(wrapper.findAll("#ai-applications-submenu a").map((link) => link.text())).toEqual(["智能助手", "图片创作"]);
    expect(wrapper.find('a[href="/ai-creation/image-generation"]').exists()).toBe(false);
    expect(wrapper.find(".ai-applications-tabs").exists()).toBe(false);
    expect(wrapper.get('[data-nav-id="resources"]').text()).toContain("资源库");
    await aiApplicationsNav.trigger("click");
    expect(aiApplicationsNav.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find("#ai-applications-submenu").exists()).toBe(false);
    await aiApplicationsNav.trigger("click");
    expect(wrapper.find("#ai-applications-submenu").exists()).toBe(true);
    wrapper.unmount();
  });

  it("exposes each Resource Library type in its secondary navigation directory", async () => {
    const wrapper = await mountAt("/sessions");

    expect(wrapper.find('a[href="/knowledge-bases"]').exists()).toBe(false);
    const resourcesNav = wrapper.get('[data-nav-id="resources"]');
    expect(resourcesNav.text()).toContain("资源库");
    expect(resourcesNav.attributes("aria-expanded")).toBe("true");
    expect(wrapper.findAll("#resources-submenu a").map((link) => link.text())).toEqual(["专家", "技能", "连接器", "知识库"]);
    await resourcesNav.trigger("click");
    expect(resourcesNav.attributes("aria-expanded")).toBe("false");
    expect(wrapper.find("#resources-submenu").exists()).toBe(false);
    await resourcesNav.trigger("click");
    expect(wrapper.find("#resources-submenu").exists()).toBe(true);
    wrapper.unmount();
  });

  it.each([
    ["/workflows/workflow-1", "/workflows"],
    ["/experts/expert-1", "/resources"],
    ["/experts/new", "/resources"],
    ["/expert-teams/team-1", "/resources"],
    ["/expert-teams/new", "/resources"],
  ])("keeps the parent navigation selected at %s", async (path, selectedHref) => {
    const wrapper = await mountAt(path);

    const selected = wrapper.get(`.sidebar nav a[href="${selectedHref}"]`);
    expect(selected.classes()).toContain("router-link-active");
    expect(selected.attributes("aria-current")).toBe("page");
    expect(wrapper.findAll(".sidebar nav a.router-link-active")).toHaveLength(1);
    wrapper.unmount();
  });

  it("renders AI applications as a secondary navigation directory", async () => {
    const wrapper = await mountAt("/ai-apps/assistants/assistant-1");

    expect(wrapper.get('[data-nav-id="ai-applications"]').classes()).toContain("router-link-active");
    expect(wrapper.get('a[href="/ai-apps/assistants"]').classes()).toContain("router-link-active");
    expect(wrapper.find('a[href="/ai-creation/image-generation"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it.each([
    ["/resources", "/resources"],
    ["/resources?tab=skills", "/resources?tab=skills"],
    ["/resources?tab=connectors", "/resources?tab=connectors"],
    ["/resources?tab=knowledge", "/resources?tab=knowledge"],
    ["/resources/skills/skill-1", "/resources?tab=skills"],
  ])("keeps the matching Resource Library child selected at %s", async (path, selectedHref) => {
    const wrapper = await mountAt(path);

    expect(wrapper.get('[data-nav-id="resources"]').classes()).toContain("router-link-active");
    expect(wrapper.get(`#resources-submenu a[href="${selectedHref}"]`).classes()).toContain("router-link-active");
    expect(wrapper.findAll("#resources-submenu a.router-link-active")).toHaveLength(1);
    wrapper.unmount();
  });

  it("does not keep polling image history when no generation is active", async () => {
    vi.useFakeTimers();
    const listImageGenerations = vi.fn(async () => []);
    const wrapper = await mountAt("/sessions", { listImageGenerations } as unknown as PlatformApi);

    await flushPromises();
    await vi.advanceTimersByTimeAsync(9_100);

    expect(listImageGenerations).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });

  it("polls while an image generation is active and stops after completion", async () => {
    vi.useFakeTimers();
    const pending = { id: "record-1", state: "running" };
    const completed = { id: "record-1", state: "succeeded" };
    const listImageGenerations = vi.fn().mockResolvedValueOnce([pending]).mockResolvedValueOnce([completed]);
    const wrapper = await mountAt("/sessions", { listImageGenerations } as unknown as PlatformApi);

    await flushPromises();
    await vi.advanceTimersByTimeAsync(3_100);
    await vi.advanceTimersByTimeAsync(9_100);

    expect(listImageGenerations).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });

  it("starts monitoring when image generation updates Credits", async () => {
    vi.useFakeTimers();
    const listImageGenerations = vi.fn().mockResolvedValueOnce([]).mockResolvedValueOnce([{ id: "record-1", state: "pending" }]);
    const wrapper = await mountAt("/sessions", { listImageGenerations } as unknown as PlatformApi);
    await flushPromises();

    window.dispatchEvent(new Event("credits-updated"));
    await flushPromises();

    expect(listImageGenerations).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });
});
