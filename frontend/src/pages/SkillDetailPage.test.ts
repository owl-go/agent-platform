// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi, type Skill } from "../api/client";
import { createAppI18n } from "../i18n";
import SkillDetailPage from "./SkillDetailPage.vue";

describe("SkillDetailPage", () => {
  it("shows localized metadata and returns to the catalog route", async () => {
    const skill = { id: "pdf", name: "PDF", source: "upload", version: 2 } as Skill;
    const api = { getSkillDocument: vi.fn(async () => ({
      skill,
      content: "---\ndisplay_name: PDF 文档处理\nversion: 1.0.0\ndescription: Process PDFs.\ndescription_zh: 处理 PDF 文档。\n---\n# 使用说明\n",
    })) } as unknown as PlatformApi;
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: "/resources", component: { template: "<div />" } },
      { path: "/resources/skills/:skillId", component: SkillDetailPage },
    ] });
    await router.push("/resources?tab=skills");
    await router.push({ path: "/resources/skills/pdf", state: { skillReturnTo: "/resources?tab=skills" } });
    await router.isReady();
    const wrapper = mount(SkillDetailPage, { global: {
      plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api },
    } });
    await flushPromises();

    expect(wrapper.text()).toContain("PDF 文档处理");
    expect(wrapper.text()).toContain("处理 PDF 文档。");
    expect(wrapper.find(".skill-document").text()).toContain("使用说明");
    expect(wrapper.find(".skill-document").text()).not.toContain("description_zh");
    await wrapper.get(".skill-detail-back").trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.fullPath).toBe("/resources?tab=skills");
  });
});
