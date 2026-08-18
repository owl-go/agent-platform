import { createApp } from "vue";
import App from "./App.vue";
import { getCurrentUser } from "./api/client";
import { createBrowserOIDC } from "./auth/oidc";
import { authContextKey, createAuthSession, createUnavailableAuthSession, type AuthContext } from "./auth/session";
import "./styles.css";

let authContext: AuthContext;
try {
  const browserOIDC = createBrowserOIDC(import.meta.env);
  authContext = {
    session: createAuthSession(browserOIDC.client, getCurrentUser, browserOIDC.replaceCallbackURL),
    isCallback: browserOIDC.isCallback,
  };
} catch (error) {
  const message = error instanceof Error ? error.message : "OIDC configuration is invalid";
  authContext = { session: createUnavailableAuthSession(message), isCallback: false };
}

createApp(App).provide(authContextKey, authContext).mount("#app");
