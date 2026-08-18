import { mount } from "@vue/test-utils";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import { authContextKey, type AuthSession, type AuthState } from "./auth/session";

describe("App authentication boundary", () => {
  it("offers OIDC sign-in without rendering protected preview content", async () => {
    const { session, signIn } = authSession({ kind: "unauthenticated", reason: "missing" });
    const wrapper = mount(App, { global: { provide: { [authContextKey as symbol]: { session, isCallback: false } } } });

    expect(wrapper.get("[data-testid='sign-in']").text()).toContain("Sign in");
    expect(wrapper.find(".shell").exists()).toBe(false);
    await wrapper.get("[data-testid='sign-in-button']").trigger("click");
    expect(signIn).toHaveBeenCalledOnce();
  });

  it("shows the User returned by the protected bootstrap API and signs out", async () => {
    const { session, signOut } = authSession({
      kind: "authenticated",
      currentUser: {
        user_id: "user-1", email: "user@example.test", display_name: "Platform User",
        organization: { id: "org-1", slug: "acme", name: "Acme" },
        role_grants: [{ role: "platform_administrator" }],
      },
    });
    const wrapper = mount(App, { global: { provide: { [authContextKey as symbol]: { session, isCallback: false } } } });

    expect(wrapper.get("[data-testid='current-user']").text()).toContain("Platform User");
    expect(wrapper.get("[data-testid='current-user']").text()).toContain("Acme");
    await wrapper.get("[data-testid='sign-out']").trigger("click");
    expect(signOut).toHaveBeenCalledOnce();
  });
});

function authSession(initial: AuthState) {
  const state = ref<AuthState>(initial);
  const signIn = vi.fn(async () => undefined);
  const signOut = vi.fn(async () => undefined);
  const session: AuthSession = {
    state,
    initialize: vi.fn(async () => undefined),
    signIn,
    signOut,
    dispose: vi.fn(),
  };
  return { session, signIn, signOut };
}
