// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import App from "./App.vue";
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

async function mountAt(path: string) {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path);
  await router.isReady();
  const wrapper = mount(App, {
    global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [authContextKey as symbol]: authContext() },
      stubs: { RouterView: true },
    },
  });
  await flushPromises();
  return wrapper;
}

describe("App navigation", () => {
  it.each([
    ["/workflows/workflow-1", "/workflows"],
    ["/experts/expert-1", "/experts"],
    ["/experts/new", "/experts"],
    ["/expert-teams/team-1", "/experts"],
    ["/expert-teams/new", "/experts"],
  ])("keeps the parent navigation selected at %s", async (path, selectedHref) => {
    const wrapper = await mountAt(path);

    const selected = wrapper.get(`.sidebar nav a[href="${selectedHref}"]`);
    expect(selected.classes()).toContain("router-link-active");
    expect(selected.attributes("aria-current")).toBe("page");
    expect(wrapper.findAll(".sidebar nav a.router-link-active")).toHaveLength(1);
    wrapper.unmount();
  });
});
