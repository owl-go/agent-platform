// @vitest-environment jsdom
import { DOMWrapper, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { platformApiKey, type MCPServer, type PlatformApi, type Skill } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ExtensionManager from "./ExtensionManager.vue";

const timestamps = { created_at: "2026-08-30T00:00:00Z", updated_at: "2026-08-30T00:00:00Z", version: 1 };

afterEach(() => { document.body.innerHTML = ""; });

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
  it("creates an MCP Server from the shared manager", async () => {
    const saved: MCPServer = { id: "mcp-1", name: "文档 MCP", transport: "streamable_http", url: "https://mcp.example.test", arguments: [], environment: [], tested: false, test_pending: false, ...timestamps };
    const createMCPServer = vi.fn(async () => saved);
    const api = {
      listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []), createMCPServer,
    } as unknown as PlatformApi;
    const wrapper = mountManager(api);
    await flushPromises();

    await wrapper.get(".compact-action").trigger("click");
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
    await form.findAll("input")[0]!.setValue(saved.name);
    await form.findAll("input")[1]!.setValue(saved.git_url);
    await form.trigger("submit");
    await flushPromises();

    expect(createGitSkill).toHaveBeenCalledWith({ name: saved.name, git_url: saved.git_url, git_ref: "main" });
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
    expect(administrator.text()).toContain("CLI 连接器定义");
    await administrator.findAll("button").find((button) => button.text().includes("CLI 连接器定义"))!.trigger("click");
    await flushPromises();
    expect(document.body.textContent).toContain("能力策略");
    expect(document.body.textContent).toContain("风险等级");
    administrator.unmount();
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
});
