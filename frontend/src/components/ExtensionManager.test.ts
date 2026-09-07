// @vitest-environment jsdom
import { DOMWrapper, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type CLIConnectorDefinitionInput, type MCPServer, type PlatformApi, type Skill } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ExtensionManager from "./ExtensionManager.vue";
import ConfirmDialog from "./ConfirmDialog.vue";

const timestamps = { created_at: "2026-08-30T00:00:00Z", updated_at: "2026-08-30T00:00:00Z", version: 1 };

afterEach(() => { vi.useRealTimers(); document.body.innerHTML = ""; });

function mountManager(api: PlatformApi, administrator = false) {
  const auth = { session: { state: { value: { kind: "authenticated", currentUser: { administrator } } } } } as unknown as AuthContext;
  return mount(ExtensionManager, {
    attachTo: document.body,
    props: { selectable: true, mcpServerIds: [], skillIds: [], cliConnectorDefinitionIds: [] },
    global: {
      plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
      provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth },
    },
  });
}

describe("ExtensionManager", () => {
  it.each([
    [new ApiError("unavailable", 500, "request_failed"), "服务暂时不可用"],
    [new ApiError("unauthenticated", 401, "invalid_authentication"), "重新登录"],
    [new ApiError("forbidden", 403, "forbidden"), "没有权限"],
    [new ApiError("not_found", 404, "not_found"), "已不存在"],
    [new ApiError("conflict", 409, "conflict"), "刷新后重试"],
    [new ApiError("validation", 422, "invalid_input"), "填写内容"],
    [new ApiError("validation", 413, "request_body_too_large"), "上传内容过大"],
    [new ApiError("rate_limited", 429, "rate_limited"), "请求过于频繁"],
    [new TypeError("Failed to fetch"), "网络连接"],
  ])("shows action failures without relying on a parent error listener (%s)", async (cause, message) => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [] };
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), enableCLIConnector: vi.fn().mockRejectedValue(cause) } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
    await flushPromises();
    expect(document.body.querySelector('[role="alert"]')?.textContent).toContain(message);
    expect(document.body.textContent).not.toContain("request_failed");
    wrapper.unmount();
  });

  it("separates platform resources from my resources and protects platform actions", async () => {
    const platformSkill: Skill = { id: "platform-skill", platform: true, name: "平台 PDF", source: "upload", sha256: "a".repeat(64), ...timestamps };
    const mySkill: Skill = { id: "my-skill", name: "我的审查技能", source: "upload", sha256: "b".repeat(64), ...timestamps };
    const platformMCP: MCPServer = { id: "platform-mcp", platform: true, name: "平台检索", transport: "streamable_http", url: "https://platform.example.test/mcp", arguments: [], environment: [], tested: true, test_pending: false, ...timestamps };
    const myMCP: MCPServer = { id: "my-mcp", name: "我的数据源", transport: "streamable_http", url: "https://user.example.test/mcp", arguments: [], environment: [], tested: true, test_pending: false, ...timestamps };
    const api = { listMCPServers: vi.fn(async () => [platformMCP, myMCP]), listSkills: vi.fn(async () => [platformSkill, mySkill]), getSkillDocument: vi.fn(async (id: string) => ({ skill: id === platformSkill.id ? platformSkill : mySkill, content: "# Skill" })) } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    const connectorGroups = wrapper.findAll(".catalog-group");
    expect(connectorGroups[0]!.text()).toContain("平台连接器");
    expect(connectorGroups[0]!.text()).toContain(platformMCP.name);
    expect(connectorGroups[0]!.find('button[aria-label="编辑"]').exists()).toBe(false);
    expect(connectorGroups[1]!.text()).toContain("我的连接器");
    expect(connectorGroups[1]!.text()).toContain(myMCP.name);
    expect(connectorGroups[1]!.find('button[aria-label="编辑"]').exists()).toBe(true);

    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    const skillGroups = wrapper.findAll(".catalog-group");
    expect(skillGroups[0]!.text()).toContain("平台技能");
    expect(skillGroups[0]!.text()).toContain(platformSkill.name);
    expect(skillGroups[0]!.find('button[aria-label="删除"]').exists()).toBe(false);
    expect(skillGroups[1]!.text()).toContain("我的技能");
    expect(skillGroups[1]!.text()).toContain(mySkill.name);
    expect(skillGroups[1]!.find('button[aria-label="删除"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it("creates an MCP Server from the shared manager", async () => {
    const saved: MCPServer = { id: "mcp-1", name: "文档 MCP", transport: "streamable_http", url: "https://mcp.example.test", arguments: [], environment: [], tested: false, test_pending: false, ...timestamps };
    const createMCPServer = vi.fn(async () => saved);
    const api = {
      listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), createMCPServer,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-mcp"]')!).trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".modal-card")!);
    await form.findAll("input")[0]!.setValue(saved.name);
    await form.findAll("input")[1]!.setValue(saved.url);
    await form.trigger("submit");
    await flushPromises();

    expect(createMCPServer).toHaveBeenCalledWith(expect.objectContaining({ name: saved.name, transport: "streamable_http", url: saved.url }));
    wrapper.unmount();
  });

  it("installs a Skill and selects it for the current Expert", async () => {
    const saved: Skill = { id: "skill-1", name: "代码审查", source: "git", git_url: "https://example.test/skill.git", git_ref: "main", sha256: "a".repeat(64), ...timestamps };
    const createGitSkill = vi.fn(async () => saved);
    const api = {
      listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), createGitSkill,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    await wrapper.get(".compact-action").trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".modal-card")!);
    await form.findAll("input")[0]!.setValue(saved.git_url);
    await form.trigger("submit");
    await flushPromises();

    expect(createGitSkill).toHaveBeenCalledWith({ git_url: saved.git_url, git_ref: undefined });
    expect(wrapper.emitted("update:skillIds")?.at(-1)).toEqual([[saved.id]]);
    wrapper.unmount();
  });

  it("names affected Experts before deleting a Skill", async () => {
    const saved: Skill = { id: "skill-1", name: "代码审查", source: "git", git_url: "https://example.test/skill.git", git_ref: "main", sha256: "a".repeat(64), ...timestamps };
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => [saved]),
      getSkillDeletionImpact: vi.fn(async () => ({ affected_experts: [{ id: "expert-1", name: "审查专家", version: 2 }], confirmation_token: "confirmation" })),
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    await wrapper.findAll(".resource-list article button").at(-1)!.trigger("click");
    await flushPromises();

    expect(document.body.textContent).toContain("审查专家");
    wrapper.unmount();
  });

  it("shows the uploaded Skill description from its document", async () => {
    const saved: Skill = { id: "skill-1", name: "PDF", source: "upload", sha256: "a".repeat(64), ...timestamps };
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => [saved]),
      getSkillDocument: vi.fn(async () => ({ skill: saved, content: "---\nname: pdf\ndisplay_name: PDF 文档处理\ndescription: Process PDFs.\ndescription_zh: 创建、读取并检查 PDF 文档。\n---\n# PDF" })),
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await wrapper.findAll(".subtabs button")[0]!.trigger("click");

    expect(wrapper.text()).toContain("创建、读取并检查 PDF 文档。");
    expect(wrapper.text()).toContain("PDF 文档处理");
    wrapper.unmount();
  });

  it("deletes an unreferenced uploaded Skill when protobuf omits the empty impact list", async () => {
    const saved: Skill = { id: "skill-1", name: "PDF", source: "upload", sha256: "a".repeat(64), ...timestamps };
    const listSkills = vi.fn().mockResolvedValueOnce([saved]).mockResolvedValue([]);
    const deleteSkill = vi.fn(async () => undefined);
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills,
      getSkillDeletionImpact: vi.fn(async () => ({ confirmation_token: "confirmation" })),
      deleteSkill,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    await wrapper.get('button[aria-label="删除"]').trigger("click");
    await flushPromises();

    const confirm = new DOMWrapper(Array.from(document.body.querySelectorAll("button")).find((button) => button.textContent?.trim() === "删除")!);
    await confirm.trigger("click");
    await flushPromises();

    expect(deleteSkill).toHaveBeenCalledWith(saved.id, "confirmation");
    expect(wrapper.find(".skill-catalog-card").exists()).toBe(false);
    wrapper.unmount();
  });

  it("lets only an Administrator create definitions while Users can enable available CLI Connectors", async () => {
    const definition = { id: "cli-1", name: "Feishu CLI", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", capabilities: [], state: "available", mutable: false, version: 1 } as const;
    const enableCLIConnector = vi.fn(async () => ({ id: "enable-1", definition_id: definition.id, state: "waiting_for_user" as const, action_url: "https://open.feishu.cn/page/cli", version: 1 }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), listExperts: vi.fn(async () => []), enableCLIConnector } as unknown as PlatformApi;
    const user = mountManager(api);
    await flushPromises();
    expect(user.text()).not.toContain("CLI 连接器定义");
    await user.findAll(".resource-list article button").at(-1)!.trigger("click");
    await flushPromises();
    expect(enableCLIConnector).toHaveBeenCalledWith(definition.id);
    expect(user.text()).toContain("继续完成授权");
    user.unmount();

    const administrator = mountManager(api, true);
    await flushPromises();
    expect(administrator.text()).toContain("新建连接器");
    expect(administrator.findAll(".connector-catalog-card")).toHaveLength(1);
    expect(administrator.findAll("button").some((button) => button.text() === "启用")).toBe(false);
    await administrator.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-cli"]')!).trigger("click");
    await flushPromises();
    for (const field of ["图标", "名称", "能力描述", "安装方式", "npm 安装", "上传 ZIP 包"]) expect(document.body.textContent).toContain(field);
    expect(document.body.textContent).not.toContain("npm 完整性");
    expect(document.body.textContent).not.toContain("可执行命令");
    administrator.unmount();
  });

  it("confirms administrator deletion and keeps the Connector when deletion fails", async () => {
    const definition = { id: "cli-1", name: "Feishu CLI", npm_package: "@larksuite/cli", npm_version: "1.0.93", authentication_driver: "feishu", capabilities: [], state: "disabled", mutable: true, version: 7 };
    const deleteCLIConnectorDefinition = vi.fn().mockRejectedValueOnce(new ApiError("conflict", 409, "conflict")).mockResolvedValueOnce(undefined);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), deleteCLIConnectorDefinition } as unknown as PlatformApi;
    const user = mountManager(api);
    await flushPromises();
    expect(user.find('button[aria-label="删除"]').exists()).toBe(false);
    user.unmount();
    const admin = mountManager(api, true);
    await flushPromises();
    await admin.get('.connector-catalog-card button[aria-label="删除"]').trigger("click");
    const dialog = admin.findAllComponents(ConfirmDialog).find((entry) => entry.props("open"))!;
    expect(dialog.props("message")).toContain("所有用户");
    expect(deleteCLIConnectorDefinition).not.toHaveBeenCalled();
    dialog.vm.$emit("cancel");
    await flushPromises();
    expect(deleteCLIConnectorDefinition).not.toHaveBeenCalled();
    await admin.get('.connector-catalog-card button[aria-label="删除"]').trigger("click");
    dialog.vm.$emit("confirm");
    await flushPromises();
    expect(admin.emitted("error")).toHaveLength(1);
    const toast = document.body.querySelector<HTMLElement>(".app-toast")!;
    expect(toast.textContent).toContain("刷新后重试");
    const dialogZIndex = Math.max(...Array.from(document.body.querySelectorAll<HTMLElement>(".el-overlay")).map((element) => Number(element.style.zIndex)));
    expect(Number(toast.style.zIndex)).toBeGreaterThan(dialogZIndex);
    expect(admin.findAll(".connector-catalog-card")).toHaveLength(1);
    dialog.vm.$emit("confirm");
    await flushPromises();
    expect(deleteCLIConnectorDefinition).toHaveBeenLastCalledWith(definition.id, 7);
    expect(admin.findAll(".connector-catalog-card")).toHaveLength(0);
    admin.unmount();
  });

  it("shows MCP and CLI Connectors in one catalog", async () => {
    const mcp: MCPServer = { id: "mcp-1", name: "行情查询", transport: "streamable_http", url: "https://quotes.example.test/mcp", arguments: [], environment: [], tested: true, test_pending: false, ...timestamps };
    const cli = { id: "cli-1", name: "飞书", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", capabilities: [], state: "available", mutable: false, version: 1 } as const;
    const api = { listMCPServers: vi.fn(async () => [mcp]), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [cli]), listCLIConnectorEnablements: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    expect(wrapper.findAll(".connector-catalog-grid > .connector-catalog-card")).toHaveLength(2);
    expect(wrapper.text()).not.toContain("第三方 CLI");
    wrapper.unmount();
  });

  it("keeps installation inputs after a server failure and clears the error on retry", async () => {
    const draft = { id: "cli-1", name: "Example CLI", state: "draft", version: 1 };
    const createCLIConnectorDefinition = vi.fn().mockRejectedValueOnce(new ApiError("unavailable", 500, "request_failed")).mockResolvedValueOnce(draft);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), createCLIConnectorDefinition, publishCLIConnectorDefinition: vi.fn(async () => ({ ...draft, state: "building" })) } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();
    await wrapper.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-cli"]')!).trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".cli-install-card")!);
    await form.findAll("input")[0]!.setValue(draft.name);
    await form.get("textarea").setValue("读取示例服务数据");
    await form.findAll("input")[1]!.setValue("example-cli@1.2.3");
    await form.trigger("submit");
    await flushPromises();
    expect(document.body.querySelector(".app-toast")?.textContent).toContain("服务暂时不可用");
    expect(document.body.querySelector(".cli-install-card")).not.toBeNull();
    expect((form.findAll("input")[0]!.element as HTMLInputElement).value).toBe(draft.name);
    expect((form.findAll("input")[1]!.element as HTMLInputElement).value).toBe("example-cli@1.2.3");
    expect(form.find('button.is-loading').exists()).toBe(false);
    await form.trigger("submit");
    await flushPromises();
    expect(createCLIConnectorDefinition).toHaveBeenCalledTimes(2);
    expect(document.body.querySelector(".app-toast")).toBeNull();
    expect(document.body.querySelector(".cli-install-card")).toBeNull();
    wrapper.unmount();
  });

  it("shows one status warning while polling fails, retains the setup link, and recovers", async () => {
    vi.useFakeTimers();
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [] };
    const waiting = { id: "enable-1", definition_id: definition.id, state: "waiting_for_user", action_url: "https://example.test/setup", version: 1 };
    const completeCLIConnectorEnablement = vi.fn().mockRejectedValueOnce(new ApiError("unavailable", 503, "request_failed")).mockRejectedValueOnce(new TypeError("Failed to fetch")).mockResolvedValueOnce({ ...waiting, state: "enabled" });
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [waiting]), completeCLIConnectorEnablement } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await vi.advanceTimersByTimeAsync(6000);
    expect(wrapper.findAll('[data-testid="resource-status-error"]')).toHaveLength(1);
    expect(wrapper.get('a[href="https://example.test/setup"]').text()).toBe("继续完成授权");
    await vi.advanceTimersByTimeAsync(6000);
    expect(wrapper.findAll('[data-testid="resource-status-error"]')).toHaveLength(1);
    expect(document.body.querySelector(".app-toast")).toBeNull();
    await vi.advanceTimersByTimeAsync(6000);
    expect(wrapper.find('[data-testid="resource-status-error"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("installs an npm CLI from only its icon, name, capability description, and source", async () => {
    const draft = { id: "cli-1", name: "Example CLI", icon: "code", description: "读取示例服务数据", installation_type: "npm" as const, npm_package: "example-cli", npm_version: "1.2.3", npm_integrity: "", executable: "", authentication_driver: "none" as const, capabilities: [], supported_architectures: [], recommended_skills: [], recommended_skill_ids: [], state: "draft" as const, mutable: true, version: 1, conformance_runtime_digests: [] };
    const createCLIConnectorDefinition = vi.fn(async () => draft);
    const publishCLIConnectorDefinition = vi.fn(async () => ({ ...draft, state: "building" as const, version: 2 }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), listCLIConnectorHealth: vi.fn(async () => []), createCLIConnectorDefinition, publishCLIConnectorDefinition } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();
    await wrapper.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-cli"]')!).trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".cli-install-card")!);
    await form.findAll("select")[0]!.setValue("code");
    await form.findAll("input")[0]!.setValue(draft.name);
    await form.get("textarea").setValue(draft.description);
    await form.findAll("input")[1]!.setValue("npm install example-cli@1.2.3");
    await form.trigger("submit");
    await flushPromises();

    expect(createCLIConnectorDefinition).toHaveBeenCalledWith({ name: draft.name, icon: "code", description: draft.description, installation_type: "npm", npm_package: "example-cli", npm_version: "1.2.3", archive: undefined });
    expect(publishCLIConnectorDefinition).toHaveBeenCalledWith(draft.id, draft.version);
    wrapper.unmount();
  });

  it("uploads a ZIP package through the same minimal CLI installer", async () => {
    const draft = { id: "cli-zip", name: "Local CLI", icon: "terminal", description: "处理本地数据", installation_type: "upload" as const, npm_package: "", npm_version: "", npm_integrity: "", executable: "", authentication_driver: "none" as const, capabilities: [], supported_architectures: [], recommended_skills: [], recommended_skill_ids: [], state: "draft" as const, mutable: true, version: 1, conformance_runtime_digests: [] };
    const createCLIConnectorDefinition = vi.fn(async () => draft);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), listCLIConnectorHealth: vi.fn(async () => []), createCLIConnectorDefinition, publishCLIConnectorDefinition: vi.fn(async () => ({ ...draft, state: "building" as const, version: 2 })) } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();
    await wrapper.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-cli"]')!).trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".cli-install-card")!);
    await form.findAll("input")[0]!.setValue(draft.name);
    await form.get("textarea").setValue(draft.description);
    await form.findAll("select")[1]!.setValue("upload");
    await flushPromises();
    const file = new File(["zip-content"], "connector.zip", { type: "application/zip" });
    Object.defineProperty(file, "arrayBuffer", { value: vi.fn(async () => new TextEncoder().encode("zip-content").buffer) });
    const input = form.get('input[type="file"]');
    Object.defineProperty(input.element, "files", { value: [file] });
    await input.trigger("change");
    await flushPromises();
    await form.trigger("submit");
    await flushPromises();

    expect(createCLIConnectorDefinition).toHaveBeenCalledWith(expect.objectContaining({ name: draft.name, description: draft.description, installation_type: "upload", npm_package: "", npm_version: "", archive: btoa("zip-content") }));
    wrapper.unmount();
  });

  it("lets an Administrator correct a mutable CLI Connector definition", async () => {
    const definition = {
      id: "cli-1",
      name: "Example CLI",
      icon: "terminal",
      description: "读取示例服务数据",
      installation_type: "npm" as const,
      npm_package: "example-cli",
      npm_version: "1.0.0",
      npm_integrity: "sha512-old",
      executable: "example",
      authentication_driver: "none" as const,
      capabilities: [{ id: "read", argv_prefix: ["read"], risk: "low" as const, identities: ["user" as const], scopes: [], egress_hosts: ["api.example.test"], timeout_seconds: 60 }],
      supported_architectures: ["linux-amd64" as const],
      recommended_skill_ids: [],
      recommended_skills: [],
      state: "failed" as const,
      mutable: true,
      version: 3,
    };
    const updateCLIConnectorDefinition = vi.fn(async (_id: string, input: CLIConnectorDefinitionInput) => ({ ...definition, ...input, version: 4 }));
    const publishCLIConnectorDefinition = vi.fn(async () => ({ ...definition, version: 5, state: "building" as const }));
    const listCLIConnectorHealth = vi.fn(async () => [{ definition_id: definition.id, definition_name: definition.name, definition_state: definition.state, enablement_count: 4, enabled_count: 3, waiting_for_user_count: 0, active_authorization_count: 2, attention_authorization_count: 1 }]);
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => []),
      listCLIConnectorDefinitions: vi.fn(async () => [definition]),
      listCLIConnectorEnablements: vi.fn(async () => []),
      listCLIConnectorHealth,
      updateCLIConnectorDefinition,
      publishCLIConnectorDefinition,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();
    expect(wrapper.text()).not.toContain("个用户已启用");
    expect(wrapper.text()).not.toContain("个有效授权");
    expect(listCLIConnectorHealth).not.toHaveBeenCalled();

    await wrapper.get('button[aria-label="编辑"]').trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".modal-card")!);
    expect((form.findAll("input")[1]!.element as HTMLInputElement).value).toBe("example-cli@1.0.0");
    await form.findAll("input")[1]!.setValue("example-cli@1.0.1");
    await form.trigger("submit");
    await flushPromises();

    expect(updateCLIConnectorDefinition).toHaveBeenCalledWith(definition.id, { name: definition.name, icon: "terminal", description: "读取示例服务数据", installation_type: "npm", npm_package: "example-cli", npm_version: "1.0.1", archive: undefined }, definition.version);
    expect(publishCLIConnectorDefinition).toHaveBeenCalledWith(definition.id, 4);
    wrapper.unmount();
  });

  it("opens Connector details from the card and edits from the details", async () => {
    const definition = { id: "cli-1", name: "Example CLI", icon: "terminal", description: "读取示例服务数据", installation_type: "npm", npm_package: "example-cli", npm_version: "1.0.0", npm_integrity: "sha512-test", executable: "example", authentication_driver: "none", capabilities: [], supported_architectures: [], recommended_skill_ids: [], recommended_skills: [], conformance_runtime_digests: [], state: "available", mutable: true, version: 1 } as const;
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();

    await wrapper.get(".connector-catalog-card").trigger("click");
    await flushPromises();
    expect(document.body.textContent).toContain("npm 安装 · example-cli@1.0.0");
    const detailEdit = [...document.body.querySelectorAll("button")].find((button) => button.textContent?.trim() === "编辑")!;
    detailEdit.click();
    await flushPromises();
    expect(document.body.querySelector(".cli-install-card")).not.toBeNull();
    wrapper.unmount();
  });

  it("selects only an enabled CLI Connector for the current Expert", async () => {
    const definition = { id: "cli-1", name: "No-auth CLI", npm_package: "example-cli", npm_version: "1.0.0", npm_integrity: "sha512-test", executable: "example", authentication_driver: "none", capabilities: [], state: "available", mutable: false, version: 1 } as const;
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [{ id: "enable-1", definition_id: definition.id, state: "enabled" as const, version: 1 }]) } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.get('input[type="checkbox"]').setValue(true);

    expect(wrapper.emitted("update:cliConnectorDefinitionIds")?.at(-1)).toEqual([[definition.id]]);
    wrapper.unmount();
  });

  it("installs a recommended CLI Skill explicitly and selects it for the Expert", async () => {
    const recommendation = { name: "Calendar Skill", git_url: "https://example.test/calendar-skill.git", git_ref: "main" };
    const definition = { id: "cli-1", name: "Calendar CLI", npm_package: "calendar-cli", npm_version: "1.0.0", npm_integrity: "sha512-test", executable: "calendar", authentication_driver: "none", capabilities: [], supported_architectures: ["linux-amd64"], recommended_skill_ids: [], recommended_skills: [recommendation], conformance_runtime_digests: [], state: "available", mutable: false, version: 1 } as const;
    const saved: Skill = { id: "skill-1", name: recommendation.name, source: "git", git_url: recommendation.git_url, git_ref: recommendation.git_ref, sha256: "a".repeat(64), ...timestamps };
    const createGitSkill = vi.fn(async () => saved);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [{ id: "enable-1", definition_id: definition.id, state: "enabled" as const, version: 1 }]), createGitSkill } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    await wrapper.setProps({ cliConnectorDefinitionIds: [definition.id] });

    expect(wrapper.text()).toContain("建议为当前专家选择技能“Calendar Skill”");
    await wrapper.findAll("button").find((button) => button.text().includes("安装技能"))!.trigger("click");
    await flushPromises();

    expect(createGitSkill).toHaveBeenCalledWith({ git_url: recommendation.git_url, git_ref: recommendation.git_ref });
    expect(wrapper.emitted("update:skillIds")?.at(-1)).toEqual([[saved.id]]);
    wrapper.unmount();
  });

  it("polls an active Feishu application registration until it is enabled", async () => {
    vi.useFakeTimers();
    const definition = { id: "cli-1", name: "Feishu CLI", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", capabilities: [], state: "available", mutable: false, version: 1 } as const;
    const waiting = { id: "enable-1", definition_id: definition.id, state: "waiting_for_user" as const, action_url: "https://open.feishu.cn/page/cli", version: 1 };
    const completeCLIConnectorEnablement = vi.fn(async () => ({ ...waiting, state: "enabled" as const, action_url: undefined, provider_name: "用户的飞书CLI", developer_console_url: "https://open.feishu.cn/app/cli-1", version: 2 }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [waiting]), completeCLIConnectorEnablement } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await vi.advanceTimersByTimeAsync(6000);
    await flushPromises();

    expect(completeCLIConnectorEnablement).toHaveBeenCalledWith(waiting.id);
    expect(wrapper.text()).toContain("已启用");
    expect(wrapper.text()).toContain("用户的飞书CLI");
    expect(wrapper.get('a[href="https://open.feishu.cn/app/cli-1"]').text()).toBe("开发者后台");
    wrapper.unmount();
  });

  it("starts and completes Feishu account authorization", async () => {
    vi.useFakeTimers();
    const definition = { id: "cli-1", name: "Feishu CLI", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", capabilities: [{ id: "calendar", argv_prefix: ["calendar"], risk: "low", identities: ["user" as const], scopes: ["calendar:calendar:read"], egress_hosts: ["open.feishu.cn"], timeout_seconds: 60 }], state: "available", mutable: false, version: 1 } as const;
    const enablement = { id: "enable-1", definition_id: definition.id, state: "enabled" as const, provider_name: "用户的飞书CLI", version: 2 };
    const waiting = { id: "flow-1", enablement_id: enablement.id, identity: "user" as const, scopes: ["calendar:calendar:read"], state: "waiting_for_user" as const, action_url: "https://accounts.feishu.cn/authorize" };
    const authorization = { id: "auth-1", enablement_id: enablement.id, identity: "user" as const, external_identity_id: "ou_user", external_display_name: "吴粤威", scopes: waiting.scopes, state: "active" as const, version: 1 };
    const beginCLIConnectorAuthorization = vi.fn(async () => waiting);
    const completeCLIConnectorAuthorization = vi.fn(async () => ({ ...waiting, state: "completed" as const, action_url: undefined, authorization }));
    const listCLIConnectorAuthorizations = vi.fn(async () => [] as typeof authorization[]);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [enablement]), listCLIConnectorAuthorizations, beginCLIConnectorAuthorization, completeCLIConnectorAuthorization } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.findAll("button").find((button) => button.text().includes("授权飞书账号"))!.trigger("click");
    await flushPromises();
    expect(beginCLIConnectorAuthorization).toHaveBeenCalledWith(enablement.id, "user", ["calendar:calendar:read"]);
    expect(wrapper.get('a[href="https://accounts.feishu.cn/authorize"]').text()).toBe("打开飞书授权");

    listCLIConnectorAuthorizations.mockResolvedValueOnce([authorization]);
    await vi.advanceTimersByTimeAsync(6000);
    await flushPromises();
    expect(completeCLIConnectorAuthorization).toHaveBeenCalledWith(waiting.id);
    expect(wrapper.text()).toContain("已授权：吴粤威");
    wrapper.unmount();
  });
});
