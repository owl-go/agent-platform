import { ref, type InjectionKey, type Ref } from "vue";
import type { components } from "../api/generated";

export type CurrentUser = components["schemas"]["v1CurrentUser"];

export interface OIDCUser {
  accessToken: string;
  expired: boolean | undefined;
}

export interface OIDCClient {
  getUser(): Promise<OIDCUser | null>;
  completeSignIn(): Promise<OIDCUser | null>;
  signIn(): Promise<void>;
  signOut(): Promise<void>;
  onExpired(listener: () => void): () => void;
}

export type AuthState =
  | { kind: "checking" }
  | { kind: "authenticated"; currentUser: CurrentUser }
  | { kind: "unauthenticated"; reason: "missing" | "expired" }
  | { kind: "error"; message: string };

export interface AuthSession {
  state: Readonly<Ref<AuthState>>;
  initialize(isCallback: boolean): Promise<void>;
  signIn(): Promise<void>;
  signOut(): Promise<void>;
  dispose(): void;
}

export interface AuthContext {
  session: AuthSession;
  isCallback: boolean;
}

export const authContextKey: InjectionKey<AuthContext> = Symbol("agent-platform-auth");

export function createUnavailableAuthSession(message: string): AuthSession {
  const state = ref<AuthState>({ kind: "error", message });
  return {
    state,
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
  const removeExpiredListener = client.onExpired(() => {
    state.value = { kind: "unauthenticated", reason: "expired" };
  });

  return {
    state,
    async initialize(isCallback: boolean) {
      state.value = { kind: "checking" };
      try {
        const user = isCallback ? await client.completeSignIn() : await client.getUser();
        if (isCallback) replaceCallbackURL();
        if (!user || user.expired) {
          state.value = { kind: "unauthenticated", reason: "missing" };
          return;
        }
        const currentUser = await loadCurrentUser(user.accessToken);
        state.value = { kind: "authenticated", currentUser };
      } catch (error) {
        state.value = { kind: "error", message: safeErrorMessage(error) };
      }
    },
    async signIn() {
      await client.signIn();
    },
    async signOut() {
      await client.signOut();
    },
    dispose() {
      removeExpiredListener();
    },
  };
}

function safeErrorMessage(_error: unknown): string {
  return "Authentication could not be completed";
}
