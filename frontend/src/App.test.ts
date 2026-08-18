import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { createMemoryHistory } from "vue-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import { authContextKey, type AuthSession, type AuthState } from "./auth/session";
import { createAppI18n, localeStorageKey } from "./i18n";
import { createAppRouter } from "./router";

describe("App identity, Team, route, and locale boundaries", () => {
  beforeEach(() => localStorage.clear());

  it("offers OIDC sign-in without rendering protected content", async () => {
    const { session, signIn } = authSession({ kind: "unauthenticated", reason: "missing" });
    const { wrapper } = await mountApp(session);
    expect(wrapper.get("[data-testid='sign-in']").text()).toContain("Sign in");
    expect(wrapper.find(".shell").exists()).toBe(false);
    await wrapper.get("[data-testid='sign-in-button']").trigger("click");
    expect(signIn).toHaveBeenCalledOnce();
  });

  it("restores a direct route and explicit Team query from authenticated bootstrap data", async () => {
    const { session, signOut } = authSession(authenticatedState());
    const { wrapper, router } = await mountApp(session, "/operations?team=team-b");
    expect(router.currentRoute.value.name).toBe("operations");
    expect(router.currentRoute.value.query.team).toBe("team-b");
    expect((wrapper.get("[data-testid='team-select']").element as HTMLSelectElement).value).toBe("team-b");
    expect(wrapper.get("[data-testid='current-user']").text()).toContain("Beta");
    await wrapper.get("[data-testid='nav-workspace']").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.name).toBe("workspace");
    router.back();
    await flushPromises();
    expect(router.currentRoute.value.name).toBe("operations");
    expect(router.currentRoute.value.query.team).toBe("team-b");
    await wrapper.get("[data-testid='sign-out']").trigger("click");
    expect(signOut).toHaveBeenCalledOnce();
  });

  it("changes Team through the URL and persists locale while updating document language", async () => {
    const { session } = authSession(authenticatedState());
    const { wrapper, router } = await mountApp(session, "/workspace?team=team-a");
    await wrapper.get("[data-testid='team-select']").setValue("team-b");
    await flushPromises();
    expect(router.currentRoute.value.query.team).toBe("team-b");
    await wrapper.get("[data-testid='locale-select']").setValue("zh-CN");
    expect(localStorage.getItem(localeStorageKey)).toBe("zh-CN");
    expect(document.documentElement.lang).toBe("zh-CN");
    expect(wrapper.text()).toContain("运维控制台");
  });

  it("shows a permission explanation for a direct route outside Team grants", async () => {
    const state = authenticatedState();
    state.currentUser.role_grants = [{ role: "agent_builder", team_id: "team-a" }];
    const { session } = authSession(state);
    const { wrapper } = await mountApp(session, "/operations?team=team-a");
    expect(wrapper.get("[data-testid='access-denied']").text()).toContain("unavailable");
  });

  it("translates authentication failures at the presentation boundary", async () => {
    localStorage.setItem(localeStorageKey, "zh-CN");
    const { session } = authSession({ kind: "error", message: "internal provider detail" });
    const { wrapper } = await mountApp(session);
    expect(wrapper.get("[role='alert']").text()).toContain("无法完成身份验证");
    expect(wrapper.text()).not.toContain("internal provider detail");
  });
});

async function mountApp(session: AuthSession, path = "/workspace") {
  const router = createAppRouter(createMemoryHistory());
  await router.push(path);
  await router.isReady();
  const i18n = createAppI18n(localStorage, "en-US");
  const wrapper = mount(App, { global: { plugins: [router, i18n], provide: { [authContextKey as symbol]: { session, isCallback: false } } } });
  await flushPromises();
  return { wrapper, router };
}

function authenticatedState(): Extract<AuthState, { kind: "authenticated" }> {
  return {
    kind: "authenticated",
    currentUser: {
      user_id: "user-1", email: "user@example.test", display_name: "Platform User",
      organization: { id: "org-1", slug: "acme", name: "Acme" },
      role_grants: [{ role: "platform_administrator" }],
      teams: [{ id: "team-a", slug: "alpha", name: "Alpha" }, { id: "team-b", slug: "beta", name: "Beta" }],
    },
  };
}

function authSession(initial: AuthState) {
  const state = ref<AuthState>(initial);
  const signIn = vi.fn(async () => undefined);
  const signOut = vi.fn(async () => undefined);
  const session: AuthSession = { state, initialize: vi.fn(async () => undefined), signIn, signOut, dispose: vi.fn() };
  return { session, signIn, signOut };
}
