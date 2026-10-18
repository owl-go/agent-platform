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
  it("ignores a confirmation response after switching to another preview", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    let finish!: (value: Awaited<ReturnType<typeof api.decideResourceCreationAction>>) => void;
    api.decideResourceCreationAction.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    await button(wrapper,"确认创建").trigger("click");
    api.getResourceCreationAction.mockResolvedValueOnce({ id:"next",kind:"expert",state:"pending",name:"Next",description:"Display",expires_at:"2026-10-10",version:1,proposal:{kind:"expert",expert:{name:"Next",introduction:"Next display",guidance:"# Next"}} });
    await wrapper.setProps({ actionId:"next" }); await flushPromises();
    finish({ id:"action",kind:"expert",state:"confirmed",name:"Review",description:"Display",expires_at:"2026-10-10",version:4 });
    await flushPromises();
    expect(wrapper.emitted("confirmed")).toBeUndefined();
    expect(wrapper.emitted("close")).toBeUndefined();
    expect(api.decideResourceCreationAction).toHaveBeenCalledTimes(1);
    expect(wrapper.get("input").element.value).toBe("Next");
    wrapper.unmount();
  });
  it("keeps edits made during an in-flight revision unsaved", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    let finish!: (value: Awaited<ReturnType<typeof api.reviseResourceCreationAction>>) => void;
    api.reviseResourceCreationAction.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
    await wrapper.get("textarea").setValue("Submitted display");
    await button(wrapper,"保存修订").trigger("click");
    await wrapper.get("textarea").setValue("Later display");
    finish({ id: "action", kind: "expert", state: "pending", name: "Review", description: "Display", expires_at: "2026-10-10", version: 4 });
    await flushPromises();
    expect(api.reviseResourceCreationAction).toHaveBeenCalledWith("action",expect.objectContaining({ expert: expect.objectContaining({ introduction:"Submitted display" }) }),3);
    expect(wrapper.get("textarea").element.value).toBe("Later display");
    expect(button(wrapper,"确认创建").attributes("disabled")).toBeDefined();
    await button(wrapper,"保存修订").trigger("click"); await flushPromises();
    expect(api.reviseResourceCreationAction).toHaveBeenLastCalledWith("action",expect.objectContaining({ expert: expect.objectContaining({ introduction:"Later display" }) }),4);
    expect(button(wrapper,"确认创建").attributes("disabled")).toBeUndefined();
    wrapper.unmount();
  });
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
