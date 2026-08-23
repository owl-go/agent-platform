import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type PlatformApi, type RuntimeImage } from "../api/client";
import { authContextKey, type AuthSession, type AuthState } from "../auth/session";
import { createAppI18n } from "../i18n";
import RuntimeCatalogPanel from "./RuntimeCatalogPanel.vue";

const image: RuntimeImage = {
  id: "image-1", runtime: "codex", cli_version: "1.2.3", adapter_version: "2026.08",
  image_digest: `registry.example/codex@sha256:${"a".repeat(64)}`,
  capabilities: { streaming: true, subagents: false }, status: "experimental",
  version: 1, created_at: "2026-08-18T10:00:00Z",
};

describe("RuntimeCatalogPanel", () => {
  it("renders real records and hides writes from Team-scoped administrators", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api, [{ role: "platform_administrator", team_id: "team-1" }]);
    await flushPromises();

    expect(wrapper.text()).toContain("codex");
    expect(wrapper.text()).toContain("Read-only catalog");
    expect(wrapper.find("[data-testid='register-runtime']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='save-runtime-status']").exists()).toBe(false);
  });

  it("registers one immutable record with a stable intent key", async () => {
    const api = apiStub();
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();
    await wrapper.get("[data-testid='register-runtime']").trigger("click");
    const inputs = wrapper.findAll("[data-testid='register-runtime-form'] input");
    await inputs[0]!.setValue("2.0.0");
    await inputs[1]!.setValue("2026.09");
    await inputs[2]!.setValue(`registry.example/claude@sha256:${"b".repeat(64)}`);
    await wrapper.get("[data-testid='register-runtime-form']").trigger("submit");
    await flushPromises();

    expect(api.registerRuntimeImage).toHaveBeenCalledOnce();
    const [input, key] = vi.mocked(api.registerRuntimeImage).mock.calls[0]!;
    expect(input).toMatchObject({ runtime: "claude", cli_version: "2.0.0", adapter_version: "2026.09" });
    expect(key).toEqual(expect.any(String));
  });

  it("reuses an idempotency key for a retry and rotates it after the intent changes", async () => {
    const api = apiStub();
    vi.mocked(api.registerRuntimeImage)
      .mockRejectedValueOnce(new ApiError("unavailable", 503, "authorization_failed", "request-1"))
      .mockRejectedValueOnce(new ApiError("unavailable", 503, "authorization_failed", "request-2"))
      .mockResolvedValueOnce({ ...image, id: "image-2" });
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();
    await wrapper.get("[data-testid='register-runtime']").trigger("click");
    await wrapper.get("[data-testid='runtime-cli-version']").setValue("2.0.0");
    await wrapper.get("[data-testid='runtime-adapter-version']").setValue("2026.09");
    await wrapper.get("[data-testid='runtime-digest']").setValue(`registry.example/claude@sha256:${"b".repeat(64)}`);

    await wrapper.get("[data-testid='register-runtime-form']").trigger("submit");
    await flushPromises();
    await wrapper.get("[data-testid='register-runtime-form']").trigger("submit");
    await flushPromises();
    const firstKey = vi.mocked(api.registerRuntimeImage).mock.calls[0]![1];
    expect(vi.mocked(api.registerRuntimeImage).mock.calls[1]![1]).toBe(firstKey);

    await wrapper.get("[data-testid='runtime-cli-version']").setValue("2.0.1");
    await wrapper.get("[data-testid='register-runtime-form']").trigger("submit");
    await flushPromises();
    expect(vi.mocked(api.registerRuntimeImage).mock.calls[2]![1]).not.toBe(firstKey);
  });

  it("refreshes the current Version after a status conflict", async () => {
    const api = apiStub();
    vi.mocked(api.changeRuntimeImageStatus).mockRejectedValueOnce(new ApiError("conflict", 412, "version_conflict", "request-1"));
    vi.mocked(api.getRuntimeImage).mockResolvedValueOnce({ ...image, status: "production", conformance_evidence_key: "evidence/phase-0/codex.json", version: 2 });
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();
    await wrapper.get("[data-testid='runtime-status']").setValue("production");
    await wrapper.get("[data-testid='conformance-evidence-key']").setValue("evidence/phase-0/codex.json");
    await wrapper.get("form.status-form").trigger("submit");
    await flushPromises();

    expect(api.changeRuntimeImageStatus).toHaveBeenCalledWith("image-1", { status: "production", blocked_reason: undefined, conformance_evidence_key: "evidence/phase-0/codex.json" }, 1, expect.any(String));
    expect(api.getRuntimeImage).toHaveBeenCalledWith("image-1");
    expect(wrapper.get("[role='alert']").text()).toContain("Data changed");
    expect(wrapper.get("[data-testid='runtime-detail']").text()).toContain("Production Runtime");
  });

  it("distinguishes retained evidence from Production status", async () => {
    const api = apiStub();
    vi.mocked(api.listRuntimeImages).mockResolvedValueOnce({ items: [{
      ...image, status: "blocked", blocked_reason: "security hold",
      conformance_evidence_key: "phase-0/codex/evidence.tar", conformance_evidence_sha256: "b".repeat(64),
    }], nextPageToken: "" });
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();

    expect(wrapper.get(".evidence-state").text()).toContain("Conformance evidence recorded");
    expect(wrapper.get(".evidence-state").text()).not.toContain("No Conformance evidence");
  });

  it("preserves failed pagination navigation and retries the same cursor", async () => {
    const api = apiStub();
    vi.mocked(api.listRuntimeImages)
      .mockResolvedValueOnce({ items: [image], nextPageToken: "page-2" })
      .mockRejectedValueOnce(new ApiError("unavailable", 503, "runtime_catalog_query_failed", "request-1"))
      .mockResolvedValueOnce({ items: [{ ...image, id: "image-2", runtime: "hermes" }], nextPageToken: "" })
      .mockResolvedValueOnce({ items: [image], nextPageToken: "page-2" });
    const wrapper = mountPanel(api, [{ role: "platform_administrator" }]);
    await flushPromises();

    await wrapper.findAll(".catalog-pagination button")[1]!.trigger("click");
    await flushPromises();
    expect(wrapper.get("[role='alert']").text()).toContain("Retry");
    expect(wrapper.text()).toContain("codex");
    await wrapper.get("[role='alert'] button").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("hermes");
    await wrapper.findAll(".catalog-pagination button")[0]!.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("codex");
    expect(vi.mocked(api.listRuntimeImages).mock.calls.map((call) => call[0])).toEqual(["", "page-2", "page-2", ""]);
  });
});

function mountPanel(api: PlatformApi, roleGrants: Array<{ role: string; team_id?: string }>) {
  const state = ref<AuthState>({ kind: "authenticated", currentUser: {
    user_id: "user-1", organization: { id: "org-1", slug: "acme", name: "Acme" },
    role_grants: roleGrants, teams: [{ id: "team-1", slug: "platform", name: "Platform" }],
  } });
  const session: AuthSession = {
    state, accessToken: () => "token", initialize: vi.fn(), signIn: vi.fn(), signOut: vi.fn(), dispose: vi.fn(),
  };
  return mount(RuntimeCatalogPanel, { global: {
    plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")],
    provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: { session, isCallback: false } },
  } });
}

function apiStub(): PlatformApi {
  return {
    listRuntimeImages: vi.fn(async () => ({ items: [image], nextPageToken: "" })),
    getRuntimeImage: vi.fn(async () => image),
    registerRuntimeImage: vi.fn(async () => ({ ...image, id: "image-2" })),
    changeRuntimeImageStatus: vi.fn(async (_id, input) => ({ ...image, status: input.status, blocked_reason: input.blocked_reason, conformance_evidence_key: input.conformance_evidence_key, version: 2 })),
    listCredentialProfiles: vi.fn(async () => []), getCredentialProfile: vi.fn(), registerCredentialProfile: vi.fn(), changeCredentialProfileStatus: vi.fn(),
    listConfiguredModels: vi.fn(async () => []), getConfiguredModel: vi.fn(), registerConfiguredModel: vi.fn(), changeConfiguredModelStatus: vi.fn(),
    listSourceControlProviders: vi.fn(async () => []), getSourceControlProvider: vi.fn(), registerSourceControlProvider: vi.fn(), changeSourceControlProviderStatus: vi.fn(),
    listRepositoryBindings: vi.fn(async () => []), getRepositoryBinding: vi.fn(), registerRepositoryBinding: vi.fn(), updateRepositoryBinding: vi.fn(), validateRepositoryBinding: vi.fn(),
    listAgents: vi.fn(async () => []), getAgent: vi.fn(), createAgent: vi.fn(), updateAgent: vi.fn(), listAgentDrafts: vi.fn(async () => []), getAgentDraft: vi.fn(), createAgentDraft: vi.fn(), updateAgentDraft: vi.fn(), validateAgentDraft: vi.fn(), getAgentDraftApproval: vi.fn(), requestAgentDraftApproval: vi.fn(), decideAgentDraftApproval: vi.fn(), publishAgentDraft: vi.fn(), listAgentReleases: vi.fn(async () => []), getAgentRelease: vi.fn(), deprecateAgentRelease: vi.fn(), blockAgentRelease: vi.fn(),
    listCodingTaskLaunchOptions: vi.fn(async () => ({ items: [], prerequisite: "release" })), listCodingTasks: vi.fn(async () => []), getCodingTask: vi.fn(), createCodingTask: vi.fn(), getCodingTaskSession: vi.fn(), listRuns: vi.fn(async () => []), getRun: vi.fn(), listRunApprovals: vi.fn(async () => []), decideRunApproval: vi.fn(), controlRun: vi.fn(), streamRunEvents: vi.fn(async () => ({ cursor: 0, terminal: true })), listRunArtifacts: vi.fn(async () => []), getArtifactDownload: vi.fn(),
  };
}
