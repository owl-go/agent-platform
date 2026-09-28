// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createMemoryHistory } from "vue-router";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import AIApplicationsPage from "./AIApplicationsPage.vue";

describe("AIApplicationsPage", () => {
  it("does not render a redundant parent heading", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push({ name: "smart-assistants" });
    await router.isReady();

    const wrapper = mount(AIApplicationsPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        stubs: { RouterView: true },
      },
    });

    expect(wrapper.find(".ai-applications-header").exists()).toBe(false);
    expect(wrapper.find(".ai-applications-page").exists()).toBe(true);
    expect(wrapper.find(".ai-applications-page").classes()).not.toContain(
      "ai-applications-page--conversation",
    );
  });

  it("uses the viewport layout for assistant conversations", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push({
      name: "smart-assistant-conversation",
      params: { assistantId: "assistant-1", conversationId: "conversation-1" },
    });
    await router.isReady();

    const wrapper = mount(AIApplicationsPage, {
      global: {
        plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        stubs: { RouterView: true },
      },
    });

    expect(wrapper.find(".ai-applications-page").classes()).toContain(
      "ai-applications-page--conversation",
    );
  });
});
