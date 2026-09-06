// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import { createMemoryHistory } from "vue-router";
import { platformApiKey, type PlatformApi, type Skill } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import CatalogDetails from "./CatalogDetails.vue";

afterEach(() => document.body.replaceChildren());
it("renders installed Skill Markdown safely and launches a preselected new Session", async () => {
 const skill = { id: "pdf", name: "PDF 文档处理", source: "upload", version: 2 } as Skill;
 const api = { getSkillDocument: vi.fn(async () => ({ skill, content: '# Document\n\n<script>window.untrusted=true</script>\n\n[unsafe](javascript:alert(1))\n\n![remote](https://example.test/tracker.png)' })) } as unknown as PlatformApi;
 const router = createAppRouter(createMemoryHistory()); await router.push("/resources"); await router.isReady();
 const wrapper = mount(CatalogDetails, { attachTo: document.body, props: { skill }, global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
 await flushPromises();
 expect(api.getSkillDocument).toHaveBeenCalledWith("pdf"); expect(document.body.querySelector(".skill-document h1")?.textContent).toBe("Document");
 expect(document.body.querySelector(".skill-document script, .skill-document img, a[href^='javascript:']")).toBeNull();
 const launch = [...document.body.querySelectorAll<HTMLButtonElement>("button")].find(button => button.textContent?.includes("去使用")); expect(launch).toBeDefined(); launch!.click(); await flushPromises();
 expect(router.currentRoute.value.path).toBe("/sessions"); expect(router.currentRoute.value.query.skill_id).toBe("pdf"); expect(router.currentRoute.value.query.new).toBeTruthy(); expect(wrapper.emitted("close")).toBeTruthy(); wrapper.unmount();
});
