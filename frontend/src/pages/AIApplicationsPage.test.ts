// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import AIApplicationsPage from "./AIApplicationsPage.vue";

describe("AIApplicationsPage", () => {
  it("does not render a redundant parent heading", () => {
    const wrapper = mount(AIApplicationsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        stubs: { RouterView: true },
      },
    });

    expect(wrapper.find(".ai-applications-header").exists()).toBe(false);
    expect(wrapper.find(".ai-applications-page").exists()).toBe(true);
  });
});
