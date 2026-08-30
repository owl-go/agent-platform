// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type Expert, type MCPServer, type PlatformApi, type Skill } from "../api/client";
import { createAppI18n } from "../i18n";
import ExpertsPage from "./ExpertsPage.vue";

describe("ExpertsPage", () => {
  it("renders Experts whose optional binding arrays are omitted by JSON", async () => {
    const expert = {
      id: "expert-1",
      name: "代码专家",
      description: "分析代码",
      created_at: "2026-08-29T00:00:00Z",
      updated_at: "2026-08-29T00:00:00Z",
      version: 1,
    } as Expert;
    const api = {
      listExperts: vi.fn(async () => [expert]),
      listMCPServers: vi.fn(async () => []),
      listSkills: vi.fn(async () => []),
    } as unknown as PlatformApi;
    const wrapper = mount(ExpertsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });

    await flushPromises();

    expect(wrapper.get(".expert-card h2").text()).toBe("代码专家");
    expect(wrapper.get(".tag-row").text()).toContain("未绑定扩展");
    wrapper.unmount();
  });

  it("uses the shared extension manager to bind MCP Servers and Skills", async () => {
    const expert: Expert = {
      id: "expert-1", name: "代码专家", description: "分析代码", mcp_server_ids: [], skill_ids: [],
      created_at: "2026-08-29T00:00:00Z", updated_at: "2026-08-29T00:00:00Z", version: 1,
    };
    const mcp: MCPServer = {
      id: "mcp-1", name: "文档 MCP", transport: "streamable_http", url: "https://mcp.example.test", arguments: [], environment: [], tested: true, test_pending: false,
      created_at: "2026-08-29T00:00:00Z", updated_at: "2026-08-29T00:00:00Z", version: 1,
    };
    const skill: Skill = {
      id: "skill-1", name: "代码审查", source: "git", git_url: "https://example.test/skill.git", git_ref: "main", sha256: "a".repeat(64),
      created_at: "2026-08-29T00:00:00Z", updated_at: "2026-08-29T00:00:00Z", version: 1,
    };
    const updateExpert = vi.fn(async (_id, input) => ({ ...expert, ...input, version: 2 }));
    const api = {
      listExperts: vi.fn(async () => [expert]),
      listMCPServers: vi.fn(async () => [mcp]),
      listSkills: vi.fn(async () => [skill]),
      updateExpert,
    } as unknown as PlatformApi;
    const wrapper = mount(ExpertsPage, {
      global: {
        plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")],
        provide: { [platformApiKey as symbol]: api },
      },
    });
    await flushPromises();

    await wrapper.get(".card-menu button").trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".extension-manager .subtabs button")).toHaveLength(3);
    expect(wrapper.get(".extension-manager .compact-action").text()).toContain("MCP");
    await wrapper.get(".extension-manager .extension-choice input").setValue(true);
    await wrapper.findAll(".extension-manager .subtabs button")[1]!.trigger("click");
    expect(wrapper.get(".extension-manager .compact-action").text()).toContain("Skill");
    await wrapper.get(".extension-manager .extension-choice input").setValue(true);
    await wrapper.get(".expert-form-modal").trigger("submit");
    await flushPromises();

    expect(updateExpert).toHaveBeenCalledWith(expert.id, expect.objectContaining({ mcp_server_ids: [mcp.id], skill_ids: [skill.id] }), expert.version);
    wrapper.unmount();
  });
});
