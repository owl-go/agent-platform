import { createApp } from "vue";
import App from "./App.vue";
import { createPlatformApi, getCurrentUser, platformApiKey } from "./api/client";
import { createBrowserOIDC } from "./auth/oidc";
import { authContextKey, createAuthSession, createUnavailableAuthSession, type AuthContext } from "./auth/session";
import { createAppI18n } from "./i18n";
import { createAppRouter } from "./router";
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

const app = createApp(App);
app.provide(authContextKey, authContext);
app.provide(platformApiKey, createPlatformApi(() => authContext.session.accessToken()));
app.use(createAppI18n());
app.use(createAppRouter());
app.mount("#app");
