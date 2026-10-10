// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi } from "../api/client";
import ResourceCreationPreview from "./ResourceCreationPreview.vue";

function fixture(state = "pending") {
  const action = { id: "action", kind: "expert", state, name: "Review", description: "Display", expires_at: "2026-10-10", version: 3 };
  const api = {
    getResourceCreationAction: vi.fn(async () => ({ ...action, proposal: { kind: "expert", expert: { name: "Review", introduction: "Display", guidance: "# Guide" } } })),
    listMCPServers: vi.fn(async () => []), listSkills: vi.fn(async () => []),
    listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []),
    reviseResourceCreationAction: vi.fn(async () => ({ ...action, version: 4 })),
    decideResourceCreationAction: vi.fn(async () => ({ ...action, state: "confirmed" })),
  };
  const wrapper = mount(ResourceCreationPreview, {
    props: { actionId: "action" },
    global: { provide: { [platformApiKey as symbol]: api as unknown as PlatformApi }, plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
  });
  return { wrapper, api };
}
const button = (wrapper: ReturnType<typeof fixture>["wrapper"], label: string) => wrapper.findAll("button").find(item => item.text() === label)!;

describe("Expert creation preview", () => {
  it("requires saving the CAS revision before confirming edited guidance", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    await wrapper.get("textarea").setValue("Revised display");
    expect(button(wrapper,"确认创建").attributes("disabled")).toBeDefined();
    await button(wrapper,"保存修订").trigger("click"); await flushPromises();
    expect(api.reviseResourceCreationAction).toHaveBeenCalledWith("action",expect.objectContaining({ expert: expect.objectContaining({ introduction:"Revised display" }) }),3);
    await button(wrapper,"确认创建").trigger("click"); await flushPromises();
    expect(api.decideResourceCreationAction).toHaveBeenCalledWith("action","confirm");
    expect(wrapper.emitted("confirmed")).toHaveLength(1); wrapper.unmount();
  });
  it("keeps edits and blocks confirmation when revision fails", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    api.reviseResourceCreationAction.mockRejectedValueOnce(new Error("version conflict"));
    await wrapper.get("textarea").setValue("Keep this edit");
    await button(wrapper,"保存修订").trigger("click"); await flushPromises();
    expect(wrapper.text()).toContain("修订失败");
    expect(wrapper.get("textarea").element.value).toBe("Keep this edit");
    expect(button(wrapper,"确认创建").attributes("disabled")).toBeDefined(); wrapper.unmount();
  });
  it("does not offer another write for terminal historical actions", async () => {
    const { wrapper } = fixture("expired"); await flushPromises();
    expect(button(wrapper,"确认创建").attributes("disabled")).toBeDefined();
    expect(button(wrapper,"保存修订").attributes("disabled")).toBeDefined(); wrapper.unmount();
  });
});
