// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi, type RedemptionCodeStatus } from "../api/client";
import { createAppI18n } from "../i18n";
import AdminCredits from "./AdminCredits.vue";

describe("AdminCredits", () => {
  it("lists safe code identifiers and voids an available code", async () => {
    const available: RedemptionCodeStatus = { id: "code-1", batch_id: "batch-1", identifier: "safe-id", state: "available", value_hundredths: 10000, created_at: "2026-09-04T10:00:00Z" };
    const api = {
      listRedemptionCodes: vi.fn(async () => ({ items: [available] })),
      voidRedemptionCode: vi.fn(async () => ({ ...available, state: "void", voided_at: "2026-09-04T11:00:00Z" })),
    } as unknown as PlatformApi;
    const wrapper = mount(AdminCredits, { props: { section: "codes" }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.text()).toContain("safe-id");
    expect(wrapper.text()).not.toContain("AWC-");
    const voidButton = wrapper.findAll("button").find((button) => button.text().includes("作废"));
    expect(voidButton).toBeDefined();
    await voidButton!.trigger("click");
    await flushPromises();

    expect(api.voidRedemptionCode).toHaveBeenCalledWith("code-1");
    expect(wrapper.text()).toContain("已作废");
  });
});
