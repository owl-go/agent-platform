/// <reference types="vite/client" />

interface Window {
  __AGENT_WORKSPACE_CONFIG__?: import("./auth/oidc").OIDCEnvironment;
}

interface ImportMetaEnv {
  readonly VITE_OIDC_AUTHORITY?: string;
  readonly VITE_OIDC_CLIENT_ID?: string;
  readonly VITE_OIDC_REDIRECT_URI?: string;
  readonly VITE_OIDC_POST_LOGOUT_REDIRECT_URI?: string;
}
