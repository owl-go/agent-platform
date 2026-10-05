// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

const bootstrap = vi.hoisted(() => ({
  app: { provide: vi.fn(), use: vi.fn(), mount: vi.fn() },
  oidc: vi.fn(() => { throw new Error("OIDC is intentionally unavailable"); }),
  router: vi.fn(() => ({})),
}));
vi.mock("vue", async (original) => ({ ...await original<typeof import("vue")>(), createApp: () => bootstrap.app }));
vi.mock("./auth/oidc", () => ({ createBrowserOIDC: bootstrap.oidc }));
vi.mock("./router", () => ({ createAppRouter: bootstrap.router }));

beforeEach(() => { vi.resetModules(); vi.clearAllMocks(); });
describe("registration entry bootstrap", () => {
  it.each(["/register/wechat?id=" + "a".repeat(43), "/register/failed"])("keeps %s public without starting the product router or OIDC", async (path) => {
    window.history.replaceState({}, "", path);
    await import("./main");
    expect(bootstrap.oidc).not.toHaveBeenCalled();
    expect(bootstrap.router).not.toHaveBeenCalled();
    expect(window.location.pathname).toBe(path.split("?")[0]);
    expect(bootstrap.app.mount).toHaveBeenCalledWith("#app");
  });
  it("initializes normal product routes", async () => {
    window.history.replaceState({}, "", "/home");
    await import("./main");
    expect(bootstrap.oidc).toHaveBeenCalledOnce();
    expect(bootstrap.router).toHaveBeenCalledOnce();
  });
});
