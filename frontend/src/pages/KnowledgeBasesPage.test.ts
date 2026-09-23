// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { defineComponent, h, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type KnowledgeBase, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import KnowledgeBasesPage from "./KnowledgeBasesPage.vue";

const base: KnowledgeBase = { id: "base-1", owner_id: "user-1", name: "产品文档", description: "", visibility: "private", platform: false, deleted: false, created_at: "2026-09-12T00:00:00Z", updated_at: "2026-09-12T00:00:00Z", version: 1 };

const inputStub = defineComponent({
  props: ["modelValue", "placeholder"],
  emits: ["update:modelValue"],
  setup(props, { emit, attrs }) {
    return () => h("input", { ...attrs, value: props.modelValue, placeholder: props.placeholder, onInput: (event: Event) => emit("update:modelValue", (event.target as HTMLInputElement).value) });
  },
});

function mountPage(searchKnowledgeBase: PlatformApi["searchKnowledgeBase"]) {
  const api = {
    listKnowledgeBases: vi.fn(async () => [base]),
    listKnowledgeCategories: vi.fn(async () => []),
    listKnowledgeDocuments: vi.fn(async () => []),
    searchKnowledgeBase,
  } as unknown as PlatformApi;
  const auth: AuthContext = { isCallback: false, session: { state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "u@example.test", display_name: "User", administrator: false, settings_ready: true } }), accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn() } };
  return mount(KnowledgeBasesPage, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth }, stubs: { ElInput: inputStub } } });
}

describe("KnowledgeBasesPage search", () => {
  it("shows retrieved excerpts with their document and category source", async () => {
    const search = vi.fn(async () => ({ index_ready: true, items: [{ document_id: "doc-1", revision_id: "rev-1", document_name: "安装指南.txt", category_name: "指南", text: "请先安装客户端。", relevance: 0.9 }] }));
    const wrapper = mountPage(search);
    await flushPromises();
    await wrapper.get(".knowledge-card").trigger("click");
    await flushPromises();
    await wrapper.get(".knowledge-search-controls input").setValue("如何安装");
    await wrapper.get(".knowledge-search-controls").trigger("submit");
    await flushPromises();
    expect(search).toHaveBeenCalledWith("base-1", "如何安装");
    expect(wrapper.get(".knowledge-search-results").text()).toContain("请先安装客户端。");
    expect(wrapper.get(".knowledge-search-results").text()).toContain("安装指南.txt · 指南");
    wrapper.unmount();
  });

  it("distinguishes an index that is not ready from no matches", async () => {
    const search = vi.fn(async () => ({ index_ready: false, items: [] }));
    const wrapper = mountPage(search);
    await flushPromises();
    await wrapper.get(".knowledge-card").trigger("click");
    await flushPromises();
    await wrapper.get(".knowledge-search-controls input").setValue("问题");
    await wrapper.get(".knowledge-search-controls").trigger("submit");
    await flushPromises();
    expect(wrapper.get(".knowledge-search-feedback").text()).toContain("尚未完成索引");
    search.mockResolvedValueOnce({ index_ready: true, items: [] });
    await wrapper.get(".knowledge-search-controls").trigger("submit");
    await flushPromises();
    expect(wrapper.get(".knowledge-search-feedback").text()).toContain("没有找到相关内容");
    wrapper.unmount();
  });
});
