import { describe, expect, it, vi } from "vitest";
import { createAuthSession, type OIDCClient, type OIDCUser } from "./session";

const currentUser = {
  user_id: "user-1",
  email: "user@example.test",
  display_name: "Platform User",
  organization: { id: "org-1", slug: "acme", name: "Acme" },
  role_grants: [{ role: "agent_user" }],
};

describe("AuthSession", () => {
  it("exchanges an OIDC callback and loads the authenticated User", async () => {
    const user: OIDCUser = { accessToken: "access-token", expired: false };
    const client = new OIDCClientStub();
    client.callbackUser = user;
    const loadCurrentUser = vi.fn(async () => currentUser);
    const replaceCallback = vi.fn();
    const session = createAuthSession(client, loadCurrentUser, replaceCallback);

    await session.initialize(true);

    expect(client.callbackCalls).toBe(1);
    expect(replaceCallback).toHaveBeenCalledOnce();
    expect(loadCurrentUser).toHaveBeenCalledWith("access-token");
    expect(session.state.value).toEqual({ kind: "authenticated", currentUser });
  });

  it("restores a valid session and clears it when the access token expires", async () => {
    const client = new OIDCClientStub();
    client.storedUser = { accessToken: "restored-token", expired: false };
    const session = createAuthSession(client, async () => currentUser, vi.fn());

    await session.initialize(false);
    expect(session.state.value.kind).toBe("authenticated");

    client.expire();
    expect(session.state.value).toEqual({ kind: "unauthenticated", reason: "expired" });
  });

  it("never calls the API for a missing or expired OIDC session", async () => {
    const client = new OIDCClientStub();
    client.storedUser = { accessToken: "expired-token", expired: true };
    const loadCurrentUser = vi.fn(async () => currentUser);
    const session = createAuthSession(client, loadCurrentUser, vi.fn());

    await session.initialize(false);

    expect(loadCurrentUser).not.toHaveBeenCalled();
    expect(session.state.value).toEqual({ kind: "unauthenticated", reason: "missing" });
  });

  it("delegates sign-in and sign-out without persisting tokens", async () => {
    const client = new OIDCClientStub();
    const session = createAuthSession(client, async () => currentUser, vi.fn());

    await session.signIn();
    await session.signOut();

    expect(client.signInCalls).toBe(1);
    expect(client.signOutCalls).toBe(1);
    expect(session.state.value).toEqual({ kind: "unauthenticated", reason: "missing" });
  });

  it("hides protected state before a failing provider logout", async () => {
    const client = new OIDCClientStub();
    client.storedUser = { accessToken: "access-token", expired: false };
    client.signOutError = new Error("provider unavailable");
    const session = createAuthSession(client, async () => currentUser, vi.fn());
    await session.initialize(false);

    await expect(session.signOut()).rejects.toThrow("provider unavailable");

    expect(session.state.value).toEqual({ kind: "unauthenticated", reason: "missing" });
  });

  it("does not expose provider or API error details", async () => {
    const client = new OIDCClientStub();
    client.storedUser = { accessToken: "sensitive-token", expired: false };
    const session = createAuthSession(client, async () => {
      throw new Error("request failed with sensitive-token");
    }, vi.fn());

    await session.initialize(false);

    expect(session.state.value).toEqual({ kind: "error", message: "Authentication could not be completed" });
  });

  it("removes callback parameters even when the OIDC exchange fails", async () => {
    const client = new OIDCClientStub();
    client.callbackError = new Error("authorization code rejected");
    const replaceCallback = vi.fn();
    const session = createAuthSession(client, async () => currentUser, replaceCallback);

    await session.initialize(true);

    expect(replaceCallback).toHaveBeenCalledOnce();
    expect(session.state.value).toEqual({ kind: "error", message: "Authentication could not be completed" });
  });
});

class OIDCClientStub implements OIDCClient {
  storedUser: OIDCUser | null = null;
  callbackUser: OIDCUser | null = null;
  callbackCalls = 0;
  callbackError: Error | undefined;
  signInCalls = 0;
  signOutCalls = 0;
  signOutError: Error | undefined;
  private expiredListener: (() => void) | undefined;

  async getUser() { return this.storedUser; }
  async completeSignIn() {
    this.callbackCalls += 1;
    if (this.callbackError) throw this.callbackError;
    return this.callbackUser;
  }
  async signIn() { this.signInCalls += 1; }
  async signOut() {
    this.signOutCalls += 1;
    if (this.signOutError) throw this.signOutError;
  }
  onExpired(listener: () => void) { this.expiredListener = listener; return () => { this.expiredListener = undefined; }; }
  expire() { this.expiredListener?.(); }
}
