import { ref, type InjectionKey, type Ref } from "vue";
import type { CurrentUser } from "../api/client";

export interface OIDCUser {
  accessToken: string;
  expired: boolean | undefined;
}

export interface OIDCClient {
  getUser(): Promise<OIDCUser | null>;
  completeSignIn(): Promise<OIDCUser | null>;
  signIn(): Promise<void>;
  signOut(): Promise<void>;
  onUserLoaded(listener: (user: OIDCUser) => void): () => void;
  onExpired(listener: () => void): () => void;
}

export type AuthState =
  | { kind: "checking" }
  | { kind: "authenticated"; currentUser: CurrentUser }
  | { kind: "unauthenticated"; reason: "missing" | "expired" }
  | { kind: "error"; message: string };

export interface AuthSession {
  state: Readonly<Ref<AuthState>>;
  accessToken(): string | undefined;
  initialize(isCallback: boolean): Promise<void>;
  signIn(): Promise<void>;
  signOut(): Promise<void>;
  dispose(): void;
}

export interface AuthContext {
  session: AuthSession;
  isCallback: boolean;
}

export const authContextKey: InjectionKey<AuthContext> = Symbol("agent-workspace-auth");

export function createUnavailableAuthSession(message: string): AuthSession {
  const state = ref<AuthState>({ kind: "error", message });
  return {
    state,
    accessToken: () => undefined,
    async initialize() {},
    async signIn() { throw new Error(message); },
    async signOut() {},
    dispose() {},
  };
}

export function createAuthSession(
  client: OIDCClient,
  loadCurrentUser: (accessToken: string) => Promise<CurrentUser>,
  replaceCallbackURL: () => void,
): AuthSession {
  const state = ref<AuthState>({ kind: "checking" });
  let activeAccessToken: string | undefined;
  const removeUserLoadedListener = client.onUserLoaded((user) => {
    if (!user.expired) activeAccessToken = user.accessToken;
  });
  const removeExpiredListener = client.onExpired(() => {
    activeAccessToken = undefined;
    state.value = { kind: "unauthenticated", reason: "expired" };
  });

  return {
    state,
    accessToken: () => activeAccessToken,
    async initialize(isCallback: boolean) {
      activeAccessToken = undefined;
      state.value = { kind: "checking" };
      try {
        let user: OIDCUser | null;
        if (isCallback) {
          try {
            user = await client.completeSignIn();
          } finally {
            replaceCallbackURL();
          }
        } else {
          user = await client.getUser();
        }
        if (!user || user.expired) {
          state.value = { kind: "unauthenticated", reason: user?.expired ? "expired" : "missing" };
          return;
        }
        const currentUser = await loadCurrentUser(user.accessToken);
        activeAccessToken = user.accessToken;
        state.value = { kind: "authenticated", currentUser };
      } catch (error) {
        activeAccessToken = undefined;
        state.value = { kind: "error", message: safeErrorMessage(error) };
      }
    },
    async signIn() {
      await client.signIn();
    },
    async signOut() {
      activeAccessToken = undefined;
      state.value = { kind: "unauthenticated", reason: "missing" };
      await client.signOut();
    },
    dispose() {
      removeUserLoadedListener();
      removeExpiredListener();
    },
  };
}

function safeErrorMessage(_error: unknown): string {
  return "Authentication could not be completed";
}
