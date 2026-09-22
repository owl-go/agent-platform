// @vitest-environment jsdom
import { DOMWrapper, flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, createPlatformApi, platformApiKey, type CLIConnectorDefinitionInput, type MCPServer, type PlatformApi, type Skill } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ExtensionManager from "./ExtensionManager.vue";
import ConfirmDialog from "./ConfirmDialog.vue";

const timestamps = { created_at: "2026-08-30T00:00:00Z", updated_at: "2026-08-30T00:00:00Z", version: 1 };

beforeEach(() => { vi.spyOn(window, "open").mockReturnValue(null); });
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks(); document.body.innerHTML = ""; });

function mountManager(api: PlatformApi, administrator = false, language = "zh-CN", mineOnly = false) {
  const auth = { session: { state: { value: { kind: "authenticated", currentUser: { administrator } } } } } as unknown as AuthContext;
  return mount(ExtensionManager, {
    attachTo: document.body,
    props: { selectable: true, mineOnly, mcpServerIds: [], skillIds: [], cliConnectorDefinitionIds: [] },
    global: {
      plugins: [createAppI18n({ getItem: () => language }, language)],
      provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth },
    },
  });
}

describe("ExtensionManager", () => {
  function setupCLIFlow(initialState: "waiting_for_user" | "enabled" = "waiting_for_user", blocked = false) {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", authentication_driver: "feishu", capabilities: [] };
    const enabled = { id: "enable-1", definition_id: definition.id, state: "enabled", version: 2 };
    const waiting = { ...enabled, state: "waiting_for_user", action_url: "https://open.feishu.cn/page/cli" };
    const popup = { opener: {}, closed: false, location: { href: "about:blank" }, close: vi.fn() };
    const open = vi.mocked(window.open).mockReturnValue(blocked ? null : popup as unknown as Window);
    const beginCLIConnectorAuthorization = vi.fn(async () => ({ id: "flow-1", enablement_id: enabled.id, state: "waiting_for_user", action_url: "https://accounts.feishu.cn/authorize" }));
    const enableCLIConnector = vi.fn(async () => initialState === "enabled" ? enabled : waiting);
    const completeCLIConnectorEnablement = vi.fn(async () => enabled);
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => []), listCLIConnectorAuthorizations: vi.fn(async () => []), enableCLIConnector, completeCLIConnectorEnablement, beginCLIConnectorAuthorization, completeCLIConnectorAuthorization: vi.fn(async () => ({ id: "flow-1", enablement_id: enabled.id, state: "waiting_for_user" })) } as unknown as PlatformApi;
    return { api, popup, open, waiting, enabled, enableCLIConnector, completeCLIConnectorEnablement, beginCLIConnectorAuthorization };
  }

  it("opens Feishu on enable and reuses the tab for account authorization after registration", async () => {
    vi.useFakeTimers();
    const flow = setupCLIFlow();
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      expect(flow.open).toHaveBeenCalledWith("about:blank", "_blank");
      expect(flow.open.mock.invocationCallOrder[0]).toBeLessThan(flow.enableCLIConnector.mock.invocationCallOrder[0]!);
      expect(flow.popup.opener).toBeNull();
      expect(flow.popup.location.href).toBe(flow.waiting.action_url);
      expect(flow.beginCLIConnectorAuthorization).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(6000);
      await flushPromises();
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledExactlyOnceWith(flow.enabled.id, "user", []);
      expect(flow.popup.location.href).toBe("https://accounts.feishu.cn/authorize");
      expect(flow.open).toHaveBeenCalledTimes(1);
      expect(wrapper.text()).toContain("等待你在飞书完成授权");
      await vi.advanceTimersByTimeAsync(6000);
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledTimes(1);
    } finally { wrapper.unmount(); }
    expect(flow.popup.close).not.toHaveBeenCalled();
  });

  it.each([false, true])("starts account authorization immediately for an existing application (popup blocked: %s)", async (blocked) => {
    const flow = setupCLIFlow("enabled", blocked);
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledExactlyOnceWith(flow.enabled.id, "user", []);
      expect(flow.completeCLIConnectorEnablement).not.toHaveBeenCalled();
      expect(wrapper.get('a[href="https://accounts.feishu.cn/authorize"]').text()).toBe("打开飞书授权");
      if (!blocked) expect(flow.popup.location.href).toBe("https://accounts.feishu.cn/authorize");
    } finally { wrapper.unmount(); }
  });

  it("resumes registration through the fallback link and continues to account authorization", async () => {
    vi.useFakeTimers();
    const flow = setupCLIFlow();
    vi.mocked(flow.api.listCLIConnectorEnablements).mockResolvedValue([flow.waiting as Awaited<ReturnType<PlatformApi["enableCLIConnector"]>>]);
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      expect(flow.open).not.toHaveBeenCalled();
      await wrapper.get('a[href="https://open.feishu.cn/page/cli"]').trigger("click");
      expect(flow.popup.location.href).toBe(flow.waiting.action_url);
      await vi.advanceTimersByTimeAsync(6000);
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledTimes(1);
      expect(flow.popup.location.href).toBe("https://accounts.feishu.cn/authorize");
    } finally { wrapper.unmount(); }
  });

  it("does not repeat slow registration completion requests or automatically authorize twice", async () => {
    vi.useFakeTimers();
    const flow = setupCLIFlow();
    let complete!: (value: typeof flow.enabled) => void;
    flow.completeCLIConnectorEnablement.mockImplementation(() => new Promise((resolve) => { complete = resolve; }));
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      await vi.advanceTimersByTimeAsync(18000);
      expect(flow.completeCLIConnectorEnablement).toHaveBeenCalledTimes(1);
      complete(flow.enabled);
      await flushPromises();
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledTimes(1);
    } finally { wrapper.unmount(); }
  });

  it.each(["enable", "authorize"])("closes the reserved blank tab when %s fails", async (step) => {
    const flow = setupCLIFlow("enabled");
    if (step === "enable") flow.enableCLIConnector.mockRejectedValue(new Error("failed"));
    else flow.beginCLIConnectorAuthorization.mockRejectedValue(new Error("failed"));
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      expect(flow.popup.close).toHaveBeenCalled();
      expect(document.body.querySelector('[role="alert"]')).not.toBeNull();
    } finally { wrapper.unmount(); }
  });

  it("opens manual authorization during the click and prevents duplicate requests", async () => {
    const flow = setupCLIFlow("enabled");
    vi.mocked(flow.api.listCLIConnectorEnablements).mockResolvedValue([flow.enabled as Awaited<ReturnType<PlatformApi["enableCLIConnector"]>>]);
    let complete!: (value: Awaited<ReturnType<typeof flow.beginCLIConnectorAuthorization>>) => void;
    flow.beginCLIConnectorAuthorization.mockImplementation(() => new Promise((resolve) => { complete = resolve; }));
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      const button = wrapper.findAll("button").find((item) => item.text() === "授权飞书账号")!;
      await button.trigger("click");
      await button.trigger("click");
      expect(flow.open.mock.invocationCallOrder[0]).toBeLessThan(flow.beginCLIConnectorAuthorization.mock.invocationCallOrder[0]!);
      expect(flow.open).toHaveBeenCalledTimes(1);
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledTimes(1);
      complete({ id: "flow-1", enablement_id: flow.enabled.id, state: "waiting_for_user", action_url: "https://accounts.feishu.cn/authorize" });
      await flushPromises();
      expect(flow.popup.location.href).toBe("https://accounts.feishu.cn/authorize");
    } finally { wrapper.unmount(); }
  });

  it("keeps an authorization link when the registration tab was closed", async () => {
    vi.useFakeTimers();
    const flow = setupCLIFlow();
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      flow.popup.closed = true;
      await vi.advanceTimersByTimeAsync(6000);
      expect(flow.beginCLIConnectorAuthorization).toHaveBeenCalledTimes(1);
      expect(flow.open).toHaveBeenCalledTimes(1);
      expect(wrapper.get('a[href="https://accounts.feishu.cn/authorize"]').text()).toBe("打开飞书授权");
    } finally { wrapper.unmount(); }
  });

  it("enables a no-auth Connector without opening Feishu or starting authorization", async () => {
    const flow = setupCLIFlow("enabled");
    const definition = (await flow.api.listCLIConnectorDefinitions())[0]!;
    vi.mocked(flow.api.listCLIConnectorDefinitions).mockResolvedValue([{ ...definition, name: "No-auth CLI", authentication_driver: "none" }]);
    const wrapper = mountManager(flow.api);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "启用")!.trigger("click");
      await flushPromises();
      expect(wrapper.text()).toContain("已启用");
      expect(flow.open).not.toHaveBeenCalled();
      expect(flow.beginCLIConnectorAuthorization).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); }
  });

  it.each([
    { capabilities: undefined, scopes: [] },
    { capabilities: [{ identities: ["user"] }], scopes: [] },
    { capabilities: [{ identities: ["user"] }, { identities: ["user"], scopes: ["calendar:calendar:read"] }, { identities: ["user"], scopes: ["calendar:calendar:read"] }, { identities: ["bot"], scopes: ["im:message"] }, { scopes: ["im:message"] }], scopes: ["calendar:calendar:read"] },
  ])("authorizes with valid JSON when protobuf omits empty capability fields ($capabilities)", async ({ capabilities, scopes }) => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities };
    const authorizationBodies: unknown[] = [];
    vi.stubGlobal("fetch", vi.fn(async (input: string, init?: RequestInit) => {
      let body: unknown = { items: [] };
      if (input === "/api/v1/connectors/cli") body = { items: [definition] };
      if (input === "/api/v1/connectors/cli/enablements") body = { items: [{ id: "enable-1", definition_id: definition.id, state: "enabled" }] };
      if (input.endsWith("/authorizations") && init?.method === "POST") {
        authorizationBodies.push(JSON.parse(String(init.body)));
        body = { id: "flow-1", enablement_id: "enable-1", state: "waiting_for_user", action_url: "https://accounts.feishu.cn/authorize" };
      }
      return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    }));
    const wrapper = mountManager(createPlatformApi(() => "test-token"));
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === "授权飞书账号")!.trigger("click");
      await flushPromises();
      expect(authorizationBodies).toEqual([{ identity: "user", scopes }]);
      expect(wrapper.get('a[href="https://accounts.feishu.cn/authorize"]').text()).toBe("打开飞书授权");
    } finally { wrapper.unmount(); }
  });

  it.each([
    ["zh-CN", "授权飞书账号", "无法发起飞书账号授权", "安装包"],
    ["en", "Authorize Feishu account", "Could not start Feishu account authorization", "package"],
  ])("shows authorization validation failures without package installation advice (%s)", async (language, label, message, packageAdvice) => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", capabilities: [] };
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [{ id: "enable-1", definition_id: definition.id, state: "enabled" }]), beginCLIConnectorAuthorization: vi.fn().mockRejectedValue(new ApiError("validation", 400, "invalid_request_body")) } as unknown as PlatformApi;
    const wrapper = mountManager(api, false, language);
    try {
      await flushPromises();
      await wrapper.findAll("button").find((button) => button.text() === label)!.trigger("click");
      await flushPromises();
      const notice = document.body.querySelector('[role="alert"]')?.textContent;
      expect(notice).toContain(message);
      expect(notice).not.toContain(packageAdvice);
      expect(notice).not.toContain("invalid_request_body");
    } finally { wrapper.unmount(); }
  });

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
    expect(connectorGroups).toHaveLength(1);
    expect(connectorGroups[0]!.text()).toContain("平台连接器");
    expect(connectorGroups[0]!.text()).toContain(platformMCP.name);
    expect(connectorGroups[0]!.find('button[aria-label="编辑"]').exists()).toBe(false);
    expect(wrapper.findAll(".catalog-group-title").map((title) => title.text())).not.toContain("我的连接器");

    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    const skillGroups = wrapper.findAll(".catalog-group");
    expect(skillGroups).toHaveLength(1);
    expect(skillGroups[0]!.text()).toContain("平台技能");
    expect(skillGroups[0]!.text()).toContain(platformSkill.name);
    expect(skillGroups[0]!.find('button[aria-label="删除"]').exists()).toBe(false);
    expect(wrapper.findAll(".catalog-group-title").map((title) => title.text())).not.toContain("我的技能");

    await wrapper.setProps({ mineOnly: true });
    await flushPromises();
    const mineGroups = wrapper.findAll(".catalog-group");
    expect(mineGroups).toHaveLength(1);
    expect(mineGroups[0]!.text()).toContain("我的技能");
    expect(mineGroups[0]!.text()).toContain(mySkill.name);
    expect(mineGroups[0]!.find('button[aria-label="删除"]').exists()).toBe(true);
    await wrapper.findAll(".subtabs button")[1]!.trigger("click");
    await flushPromises();
    const mineConnectorGroups = wrapper.findAll(".catalog-group");
    expect(mineConnectorGroups).toHaveLength(1);
    expect(mineConnectorGroups[0]!.text()).toContain("我的连接器");
    expect(mineConnectorGroups[0]!.text()).toContain(myMCP.name);
    wrapper.unmount();
  });

  it("does not expose legacy MCP mutations for managed package installations", async () => {
    const managed: MCPServer = { id: "installation-1", name: "统一包", managed_installation: true, transport: "streamable_http", url: "https://package.example.test/mcp", arguments: [], environment: [], tested: true, test_pending: false, ...timestamps };
    const api = { listMCPServers: vi.fn(async () => [managed]), listSkills: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();
    expect(wrapper.find('button[aria-label="重试"]').exists()).toBe(false);
    expect(wrapper.find('button[aria-label="编辑"]').exists()).toBe(false);
    expect(wrapper.find('button[aria-label="删除"]').exists()).toBe(false);
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
    const textInputs = form.findAll("input").filter((input) => input.attributes("type") !== "file");
    await textInputs[0]!.setValue(saved.name);
    await textInputs[1]!.setValue(saved.url);
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
    const wrapper = mountManager(api, false, "zh-CN", true);
    await flushPromises();

    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    await wrapper.get(".compact-action").trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".modal-card")!);
    expect(form.text()).not.toContain("技能名称将直接读取");
    expect(form.text()).not.toContain("Skill 根目录必须包含");
    await form.get('input[placeholder="https://github.com/owner/skill.git"]').setValue(saved.git_url);
    await form.trigger("submit");
    await flushPromises();

    expect(createGitSkill).toHaveBeenCalledWith({ git_url: saved.git_url, git_ref: undefined, icon: "sparkles" });
    expect(wrapper.emitted("update:skillIds")?.at(-1)).toEqual([[saved.id]]);
    expect(wrapper.find(`.skill-catalog-card[aria-label="${saved.name}"]`).exists()).toBe(true);
    wrapper.unmount();
  });

  it("keeps a newly created platform Skill visible when the catalog read is stale", async () => {
    const saved: Skill = { id: "platform-skill", platform: true, name: "平台技能", source: "git", git_url: "https://example.test/skill.git", sha256: "a".repeat(64), ...timestamps };
    const createGitSkill = vi.fn(async () => saved);
    const api = {
      listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), createGitSkill,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();

    await wrapper.findAll(".subtabs button")[0]!.trigger("click");
    await wrapper.get(".compact-action").trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".modal-card")!);
    await form.get('input[placeholder="https://github.com/owner/skill.git"]').setValue(saved.git_url);
    await form.trigger("submit");
    await flushPromises();

    expect(createGitSkill).toHaveBeenCalled();
    expect(wrapper.find(`.skill-catalog-card[aria-label="${saved.name}"]`).exists()).toBe(true);
    expect(wrapper.findAll(".catalog-group")[0]!.text()).toContain(saved.name);
    wrapper.unmount();
  });

  it("names affected Experts before deleting a Skill", async () => {
    const saved: Skill = { id: "skill-1", name: "代码审查", source: "git", git_url: "https://example.test/skill.git", git_ref: "main", sha256: "a".repeat(64), ...timestamps };
    const api = {
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => [saved]),
      getSkillDeletionImpact: vi.fn(async () => ({ affected_experts: [{ id: "expert-1", name: "审查专家", version: 2 }], confirmation_token: "confirmation" })),
    } as unknown as PlatformApi;
    const wrapper = mountManager(api, false, "zh-CN", true);
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
    const wrapper = mountManager(api, false, "zh-CN", true);
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
    const wrapper = mountManager(api, false, "zh-CN", true);
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
    const mcp: MCPServer = { id: "mcp-1", name: "行情查询", platform: true, transport: "streamable_http", url: "https://quotes.example.test/mcp", arguments: [], environment: [], tested: true, test_pending: false, ...timestamps };
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
    await form.get('input[maxlength="100"]').setValue(draft.name);
    await form.get("textarea").setValue("读取示例服务数据");
    await form.get('input[placeholder="@scope/package@1.2.3"]').setValue("example-cli@1.2.3");
    await form.trigger("submit");
    await flushPromises();
    expect(document.body.querySelector(".app-toast")?.textContent).toContain("服务暂时不可用");
    expect(document.body.querySelector(".cli-install-card")).not.toBeNull();
    expect((form.get('input[maxlength="100"]').element as HTMLInputElement).value).toBe(draft.name);
    expect((form.get('input[placeholder="@scope/package@1.2.3"]').element as HTMLInputElement).value).toBe("example-cli@1.2.3");
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
    await form.get('button[aria-label="code"]').trigger("click");
    await form.get('input[maxlength="100"]').setValue(draft.name);
    await form.get("textarea").setValue(draft.description);
    await form.get('input[placeholder="@scope/package@1.2.3"]').setValue("npm install -g example-cli@1.2.3");
    await form.trigger("submit");
    await flushPromises();

    expect(createCLIConnectorDefinition).toHaveBeenCalledWith({ name: draft.name, icon: "code", description: draft.description, installation_type: "npm", npm_package: "example-cli", npm_version: "1.2.3", archive: undefined });
    expect(publishCLIConnectorDefinition).toHaveBeenCalledWith(draft.id, draft.version);
    wrapper.unmount();
  });

  it("explains that a pasted npm install command still needs an exact package version", async () => {
    const createCLIConnectorDefinition = vi.fn();
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []), createCLIConnectorDefinition } as unknown as PlatformApi;
    const wrapper = mountManager(api, true);
    await flushPromises();
    await wrapper.get(".compact-action").trigger("click");
    await new DOMWrapper(document.body.querySelector<HTMLElement>('[data-testid="connector-kind-cli"]')!).trigger("click");
    const form = new DOMWrapper(document.body.querySelector<HTMLFormElement>(".cli-install-card")!);
    await form.get('input[maxlength="100"]').setValue("钉钉");
    await form.get("textarea").setValue("管理钉钉产品能力");
    await form.get('input[placeholder="@scope/package@1.2.3"]').setValue("npm install -g dingtalk-workspace-cli");
    await form.trigger("submit");
    await flushPromises();

    expect(createCLIConnectorDefinition).not.toHaveBeenCalled();
    expect(document.body.querySelector(".app-toast")?.textContent).toContain("必须指定精确版本");
    expect(document.body.querySelector(".cli-install-card")).not.toBeNull();
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
    await form.get('input[maxlength="100"]').setValue(draft.name);
    await form.get("textarea").setValue(draft.description);
    await form.findAll("select")[0]!.setValue("upload");
    await flushPromises();
    const file = new File(["zip-content"], "connector.zip", { type: "application/zip" });
    Object.defineProperty(file, "arrayBuffer", { value: vi.fn(async () => new TextEncoder().encode("zip-content").buffer) });
    const input = form.get('input[type="file"][accept=".zip,application/zip"]');
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
    expect((form.get('input[placeholder="@scope/package@1.2.3"]').element as HTMLInputElement).value).toBe("example-cli@1.0.0");
    await form.get('input[placeholder="@scope/package@1.2.3"]').setValue("example-cli@1.0.1");
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

  it("offers permission expansion when an active Feishu authorization lacks reviewed capability scopes", async () => {
    const definition = { id: "cli-1", name: "Feishu CLI", state: "available", authentication_driver: "feishu", capabilities: [{ id: "im_chat_search", identities: ["user" as const], scopes: ["im:chat:read"] }] };
    const enablement = { id: "enable-1", definition_id: definition.id, state: "enabled" as const, version: 2 };
    const authorization = { id: "auth-1", enablement_id: enablement.id, identity: "user" as const, external_identity_id: "ou_user", external_display_name: "吴粤威", scopes: ["offline_access"], state: "active" as const, version: 1 };
    const beginCLIConnectorAuthorization = vi.fn(async () => ({ id: "flow-1", enablement_id: enablement.id, state: "waiting_for_user" as const, action_url: "https://accounts.feishu.cn/authorize" }));
    const api = { listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => [definition]), listCLIConnectorEnablements: vi.fn(async () => [enablement]), listCLIConnectorAuthorizations: vi.fn(async () => [authorization]), beginCLIConnectorAuthorization } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.findAll("button").find((button) => button.text() === "扩展飞书权限")!.trigger("click");
    await flushPromises();
    expect(beginCLIConnectorAuthorization).toHaveBeenCalledWith(enablement.id, "user", ["im:chat:read"]);
    wrapper.unmount();
  });
});
