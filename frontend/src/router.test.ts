import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { createAppRouter, installStaleReleaseRecovery } from "./router";

describe("application routes", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("uses Home as the authenticated entry route", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/");
    await router.isReady();

    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.meta.surface).toBe("home");
  });

  it("redirects the deployed legacy image route to Image Creation", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/ai-creation/image-generation");
    await router.isReady();

    expect(router.currentRoute.value.name).toBe("image-creation");
    expect(router.currentRoute.value.fullPath).toBe("/ai-apps/image-creation");
  });

  it("keeps the canonical assistant detail route under AI Applications", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/ai-apps/assistants/assistant-1");
    expect(router.currentRoute.value.name).toBe("smart-assistant-detail");
  });

  it("does not expose the removed Digital Human routes", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/ai-apps/digital-humans/human-1");
    expect(router.currentRoute.value.fullPath).toBe("/home");
  });

  it("converges legacy Knowledge Base routes into the Resource Library", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/knowledge-bases");
    expect(router.currentRoute.value.name).toBe("resources");
    expect(router.currentRoute.value.fullPath).toBe("/resources?tab=knowledge");
    expect(router.currentRoute.value.meta.surface).toBe("resources");

    await router.push("/ai-apps/knowledge-bases");
    expect(router.currentRoute.value.name).toBe("resources");
    expect(router.currentRoute.value.fullPath).toBe("/resources?tab=knowledge");
  });

  it("reloads the intended route when a deployed lazy module is unavailable", () => {
    const onError = vi.fn();
    const afterEach = vi.fn();
    const assign = vi.fn();
    const storage = {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    };

    installStaleReleaseRecovery(
      { onError, afterEach } as never,
      { location: { assign }, sessionStorage: storage } as never,
    );
    const recover = onError.mock.calls[0]?.[0];
    recover?.(
      new TypeError("Failed to fetch dynamically imported module: /assets/SmartAssistantDetailPage-old.js"),
      { fullPath: "/ai-apps/assistants/assistant-1" },
    );

    expect(storage.setItem).toHaveBeenCalledWith(
      "agent-workspace:stale-release-reload",
      "/ai-apps/assistants/assistant-1",
    );
    expect(assign).toHaveBeenCalledWith("/ai-apps/assistants/assistant-1");
  });

  it("does not loop when the refreshed release still cannot load the route", () => {
    const target = "/ai-apps/assistants/assistant-1";
    const onError = vi.fn();
    const assign = vi.fn();
    const storage = {
      getItem: vi.fn(() => target),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    };

    installStaleReleaseRecovery(
      { onError, afterEach: vi.fn() } as never,
      { location: { assign }, sessionStorage: storage } as never,
    );
    const recover = onError.mock.calls[0]?.[0];
    recover?.(
      new TypeError("Failed to fetch dynamically imported module: /assets/SmartAssistantDetailPage-old.js"),
      { fullPath: target },
    );

    expect(assign).not.toHaveBeenCalled();
    expect(storage.removeItem).toHaveBeenCalledWith(
      "agent-workspace:stale-release-reload",
    );
  });
});
