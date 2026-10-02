import type { User, UserManagerSettings } from "oidc-client-ts";
import { describe, expect, it, vi } from "vitest";
import { createBrowserOIDC, readOIDCSettings } from "./oidc";
import { localeStorageKey } from "../i18n";

describe("readOIDCSettings", () => {
  const environment = {
    VITE_OIDC_AUTHORITY: "https://identity.example.test",
    VITE_OIDC_CLIENT_ID: "agent-platform-web",
    VITE_OIDC_REDIRECT_URI: "https://app.example.test/auth/callback",
    VITE_OIDC_POST_LOGOUT_REDIRECT_URI: "https://app.example.test",
  };

  it("builds an Authorization Code with PKCE configuration", () => {
    const settings = readOIDCSettings(environment, "https://app.example.test");

    expect(settings).toMatchObject({
      authority: "https://identity.example.test",
      clientId: "agent-platform-web",
      redirectURI: "https://app.example.test/auth/callback",
      postLogoutRedirectURI: "https://app.example.test",
    });
  });

  it("fails closed when configuration is missing or redirects leave the app origin", () => {
    expect(() => readOIDCSettings({}, "https://app.example.test")).toThrow("incomplete");
    expect(() => readOIDCSettings({ ...environment, VITE_OIDC_REDIRECT_URI: "https://attacker.example.test/callback" }, "https://app.example.test")).toThrow("same origin");
    expect(() => readOIDCSettings({ ...environment, VITE_OIDC_AUTHORITY: "http://identity.example.test" }, "https://app.example.test")).toThrow("HTTPS");
    expect(() => readOIDCSettings({ ...environment, VITE_OIDC_AUTHORITY: "https://identity.example.test?issuer=other" }, "https://app.example.test")).toThrow("query");
  });

  it("persists the OIDC user and automatically renews an expired access token", async () => {
    let managerSettings: UserManagerSettings | undefined;
    const freshUser = { access_token: "fresh-token", expired: false } as User;
    const manager = {
      getUser: vi.fn(async () => ({ access_token: "expired-token", expired: true }) as User),
      signinSilent: vi.fn(async () => freshUser),
      signinRedirect: vi.fn(async () => undefined),
      signinRedirectCallback: vi.fn(async () => freshUser),
      signoutRedirect: vi.fn(async () => undefined),
      events: { addAccessTokenExpired: vi.fn(), removeAccessTokenExpired: vi.fn(), addUserLoaded: vi.fn(), removeUserLoaded: vi.fn() },
    };
    const browser = {
      location: { origin: "https://app.example.test", pathname: "/", search: "" },
      history: { replaceState: vi.fn() },
      sessionStorage: new MemoryStorage(),
      localStorage: new MemoryStorage(),
    };
    const legacyKey = "oidc.user:https://identity.example.test:agent-platform-web";
    browser.sessionStorage.setItem(legacyKey, "legacy-session");
    const oidc = createBrowserOIDC(environment, browser as unknown as Window, (settings) => {
      managerSettings = settings;
      return manager as never;
    });

    expect(managerSettings?.automaticSilentRenew).toBe(true);
    expect(browser.localStorage.getItem(legacyKey)).toBe("legacy-session");
    expect(browser.sessionStorage.getItem(legacyKey)).toBeNull();
    await managerSettings?.userStore?.set("user", "persisted");
    expect(browser.localStorage.length).toBe(2);
    expect(browser.sessionStorage.length).toBe(0);
    await expect(oidc.client.getUser()).resolves.toEqual({ accessToken: "fresh-token", expired: false });
    expect(manager.signinSilent).toHaveBeenCalledOnce();
  });

  it.each([
    [null, "zh-Hans"],
    ["zh-CN", "zh-Hans"],
    ["en-US", "en"],
  ])("passes the saved language to the identity form: %s", async (locale, expected) => {
    const manager = { signinRedirect: vi.fn(async () => undefined) };
    const browser = {
      location: { origin: "https://app.example.test", pathname: "/", search: "" },
      history: { replaceState: vi.fn() },
      sessionStorage: new MemoryStorage(),
      localStorage: new MemoryStorage(),
    };
    if (locale) browser.localStorage.setItem(localeStorageKey, locale);
    const oidc = createBrowserOIDC(environment, browser as unknown as Window, () => manager as never);

    await oidc.client.signIn();
    expect(manager.signinRedirect).toHaveBeenLastCalledWith({ ui_locales: expected });
    browser.localStorage.setItem(localeStorageKey, "en-US");
    await oidc.client.signIn();
    expect(manager.signinRedirect).toHaveBeenLastCalledWith({ ui_locales: "en" });
  });

  it.each([
    [null, false],
    [{ access_token: "valid-token", expired: false }, false],
    [{ access_token: "expired-token", expired: true }, true],
  ] as const)("returns the stored session when silent renewal is unavailable: %j", async (storedUser, attemptsRenewal) => {
    const manager = {
      getUser: vi.fn(async () => storedUser as User | null),
      signinSilent: vi.fn(async () => { throw new Error("login_required"); }),
      events: { addAccessTokenExpired: vi.fn(), removeAccessTokenExpired: vi.fn(), addUserLoaded: vi.fn(), removeUserLoaded: vi.fn() },
    };
    const browser = {
      location: { origin: "https://app.example.test", pathname: "/", search: "" },
      history: { replaceState: vi.fn() },
      sessionStorage: new MemoryStorage(),
      localStorage: new MemoryStorage(),
    };
    const oidc = createBrowserOIDC(environment, browser as unknown as Window, () => manager as never);

    await expect(oidc.client.getUser()).resolves.toEqual(storedUser ? { accessToken: storedUser.access_token, expired: storedUser.expired } : null);
    expect(manager.signinSilent).toHaveBeenCalledTimes(attemptsRenewal ? 1 : 0);
  });
});

class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>();
  get length() { return this.values.size; }
  clear() { this.values.clear(); }
  getItem(key: string) { return this.values.get(key) ?? null; }
  key(index: number) { return [...this.values.keys()][index] ?? null; }
  removeItem(key: string) { this.values.delete(key); }
  setItem(key: string, value: string) { this.values.set(key, value); }
}
