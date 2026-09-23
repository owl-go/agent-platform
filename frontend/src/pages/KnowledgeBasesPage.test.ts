// @vitest-environment jsdom
import { ref } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type KnowledgeBase, type KnowledgeDocument, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import KnowledgeBasesPage from "./KnowledgeBasesPage.vue";

const base: KnowledgeBase = { id: "base-1", owner_id: "user-1", name: "测试知识库", description: "", visibility: "private", platform: false, deleted: false, created_at: "2026-09-23T00:00:00Z", updated_at: "2026-09-23T00:00:00Z", version: 1 };
const documents: KnowledgeDocument[] = [
  { id: "ready-1", knowledge_base_id: base.id, name: "已索引.txt", source_type: "upload", state: "ready", deleted: false, created_at: "2026-09-23T00:00:00Z", updated_at: "2026-09-23T00:00:00Z", version: 1 },
  { id: "failed-1", knowledge_base_id: base.id, name: "失败.txt", source_type: "upload", state: "ready", error: "向量服务不可用", deleted: false, created_at: "2026-09-23T00:00:00Z", updated_at: "2026-09-23T00:00:00Z", version: 1, latest_revision: { id: "revision-2", document_id: "failed-1", revision: 2, sha256: "a".repeat(64), size: 12, content_type: "text/plain", state: "failed", error: "向量服务不可用", created_at: "2026-09-23T00:00:00Z" } },
];

function authContext(): AuthContext {
  return {
    isCallback: false,
    session: {
      state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "user@example.test", display_name: "User", administrator: false, settings_ready: true } }),
      accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn(),
    },
  };
}

describe("KnowledgeBasesPage document indexing", () => {
  it("shows outcome tags and allows regenerating an already ready document", async () => {
    const api = {
      listKnowledgeBases: vi.fn(async () => [base]),
      listKnowledgeCategories: vi.fn(async () => []),
      listKnowledgeDocuments: vi.fn(async () => documents),
      regenerateKnowledgeDocument: vi.fn(async () => {}),
      retryKnowledgeDocument: vi.fn(async () => {}),
    } as unknown as PlatformApi;
    const wrapper = mount(KnowledgeBasesPage, {
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: authContext() } },
    });
    await flushPromises();
    await wrapper.get(".knowledge-card").trigger("click");
    await flushPromises();

    const rows = wrapper.findAll(".document-table tbody tr");
    expect(rows).toHaveLength(2);
    expect(rows[0]!.find(".el-tag--success").text()).toBe("成功");
    expect(rows[1]!.find(".el-tag--danger").text()).toBe("失败");
    await rows[0]!.get('button[aria-label="重新生成"]').trigger("click");
    await flushPromises();
    expect(api.regenerateKnowledgeDocument).toHaveBeenCalledWith(base.id, "ready-1");
    await wrapper.findAll(".document-table tbody tr")[1]!.get('button[aria-label="重试"]').trigger("click");
    await flushPromises();
    expect(api.retryKnowledgeDocument).toHaveBeenCalledWith(base.id, "failed-1");
    wrapper.unmount();
  });
});
