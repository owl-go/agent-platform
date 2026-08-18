import { describe, expect, it } from "vitest";
import { readOIDCSettings } from "./oidc";

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
});
