import { afterEach, describe, expect, it } from "vitest";
import { createMemoryHistory } from "vue-router";
import { createAppRouter } from "./router";

describe("application routes", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("redirects the deployed legacy image route to Image Creation", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/ai-creation/image-generation");
    await router.isReady();

    expect(router.currentRoute.value.name).toBe("image-creation");
    expect(router.currentRoute.value.fullPath).toBe("/ai-apps/image-creation");
  });

  it("keeps canonical assistant and digital human detail routes under AI Applications", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/ai-apps/assistants/assistant-1");
    expect(router.currentRoute.value.name).toBe("smart-assistant-detail");
    await router.push("/ai-apps/digital-humans/human-1");
    expect(router.currentRoute.value.name).toBe("digital-human-detail");
  });

  it("keeps the top-level Knowledge Bases route distinct from the AI Applications route", async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push("/knowledge-bases");
    expect(router.currentRoute.value.name).toBe("knowledge-bases");
    expect(router.currentRoute.value.meta.surface).toBe("knowledge-bases");

    await router.push("/ai-apps/knowledge-bases");
    expect(router.currentRoute.value.name).toBe("ai-application-knowledge-bases");
    expect(router.currentRoute.value.meta.surface).toBe("ai-applications");
  });
});
