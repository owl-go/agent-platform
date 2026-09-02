import { UserManager, WebStorageStateStore, type User, type UserManagerSettings } from "oidc-client-ts";
import type { OIDCClient, OIDCUser } from "./session";

export interface OIDCEnvironment {
  VITE_OIDC_AUTHORITY?: string;
  VITE_OIDC_CLIENT_ID?: string;
  VITE_OIDC_REDIRECT_URI?: string;
  VITE_OIDC_POST_LOGOUT_REDIRECT_URI?: string;
}

export interface OIDCSettings {
  authority: string;
  clientId: string;
  redirectURI: string;
  postLogoutRedirectURI: string;
}

export interface BrowserOIDC {
  client: OIDCClient;
  isCallback: boolean;
  replaceCallbackURL(): void;
}

interface UserManagerLike {
  getUser(): Promise<User | null>;
  signinSilent(): Promise<User | null>;
  signinRedirect(): Promise<void>;
  signinRedirectCallback(): Promise<User>;
  signoutRedirect(): Promise<void>;
  events: UserManager["events"];
}

type UserManagerFactory = (settings: UserManagerSettings) => UserManagerLike;

export function readOIDCSettings(environment: OIDCEnvironment, applicationOrigin: string): OIDCSettings {
  const authority = environment.VITE_OIDC_AUTHORITY?.trim() ?? "";
  const clientId = environment.VITE_OIDC_CLIENT_ID?.trim() ?? "";
  const redirectURI = environment.VITE_OIDC_REDIRECT_URI?.trim() ?? "";
  const postLogoutRedirectURI = environment.VITE_OIDC_POST_LOGOUT_REDIRECT_URI?.trim() ?? "";
  if (!authority || !clientId || !redirectURI || !postLogoutRedirectURI) {
    throw new Error("OIDC configuration is incomplete");
  }

  const parsedAuthority = secureURL(authority, "OIDC authority");
  const parsedRedirect = secureURL(redirectURI, "OIDC redirect URI");
  const parsedLogout = secureURL(postLogoutRedirectURI, "OIDC post-logout redirect URI");
  if (parsedRedirect.origin !== applicationOrigin || parsedLogout.origin !== applicationOrigin) {
    throw new Error("OIDC redirects must use the same origin as Agent Platform");
  }
  return {
    authority: parsedAuthority.toString().replace(/\/$/, ""),
    clientId,
    redirectURI,
    postLogoutRedirectURI,
  };
}

export function createBrowserOIDC(
  environment: OIDCEnvironment,
  browser: Pick<Window, "location" | "history" | "sessionStorage" | "localStorage"> = window,
  managerFactory: UserManagerFactory = (settings) => new UserManager(settings),
): BrowserOIDC {
  const settings = readOIDCSettings(environment, browser.location.origin);
  migrateStoredUser(browser.sessionStorage, browser.localStorage, settings.authority, settings.clientId);
  const manager = managerFactory({
    authority: settings.authority,
    client_id: settings.clientId,
    redirect_uri: settings.redirectURI,
    post_logout_redirect_uri: settings.postLogoutRedirectURI,
    response_type: "code",
    scope: "openid profile email",
    loadUserInfo: false,
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: browser.localStorage }),
    stateStore: new WebStorageStateStore({ store: browser.sessionStorage }),
  });
  const callbackURL = new URL(settings.redirectURI);
  const search = new URLSearchParams(browser.location.search);
  return {
    client: new UserManagerClient(manager),
    isCallback: browser.location.pathname === callbackURL.pathname && (search.has("code") || search.has("error")),
    replaceCallbackURL() {
      browser.history.replaceState({}, document.title, "/");
    },
  };
}

function migrateStoredUser(sessionStore: Storage, persistentStore: Storage, authority: string, clientID: string): void {
  const key = `oidc.user:${authority}:${clientID}`;
  const stored = sessionStore.getItem(key);
  if (stored !== null && persistentStore.getItem(key) === null) persistentStore.setItem(key, stored);
  if (stored !== null) sessionStore.removeItem(key);
}

class UserManagerClient implements OIDCClient {
  constructor(private readonly manager: UserManagerLike) {}

  async getUser(): Promise<OIDCUser | null> {
    const stored = await this.manager.getUser();
    if (!stored?.expired) return mapUser(stored);
    try {
      return mapUser(await this.manager.signinSilent());
    } catch {
      return mapUser(stored);
    }
  }

  async completeSignIn(): Promise<OIDCUser | null> {
    return mapUser(await this.manager.signinRedirectCallback());
  }

  async signIn(): Promise<void> {
    await this.manager.signinRedirect();
  }

  async signOut(): Promise<void> {
    await this.manager.signoutRedirect();
  }

  onUserLoaded(listener: (user: OIDCUser) => void): () => void {
    const mapped = (user: User) => listener(mapUser(user)!);
    this.manager.events.addUserLoaded(mapped);
    return () => this.manager.events.removeUserLoaded(mapped);
  }

  onExpired(listener: () => void): () => void {
    this.manager.events.addAccessTokenExpired(listener);
    return () => this.manager.events.removeAccessTokenExpired(listener);
  }
}

function mapUser(user: User | null): OIDCUser | null {
  if (!user) return null;
  return { accessToken: user.access_token, expired: user.expired };
}

function secureURL(value: string, field: string): URL {
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error(`${field} must be an absolute URL`);
  }
  const loopback = parsed.hostname === "localhost" || parsed.hostname === "127.0.0.1" || parsed.hostname === "::1";
  if (parsed.protocol !== "https:" && !(parsed.protocol === "http:" && loopback)) {
    throw new Error(`${field} must use HTTPS except on loopback`);
  }
  if (parsed.username || parsed.password || parsed.hash || parsed.search) {
    throw new Error(`${field} cannot contain user info, a query, or a fragment`);
  }
  return parsed;
}
