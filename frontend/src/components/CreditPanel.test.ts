// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import CreditPanel from "./CreditPanel.vue";

describe("CreditPanel", () => {
  it("loads subsequent ledger pages from the returned cursor", async () => {
    const api = {
      getCreditBalance: vi.fn(async () => ({ total_hundredths: 60000, reserved_hundredths: 1000, available_hundredths: 9000, daily_remaining_hundredths: 60000, persistent_hundredths: 0, today_consumed_hundredths: 50000, daily_allocation_hundredths: 60000, credit_day: "2026-09-04", timezone: "Asia/Shanghai", next_allocation_at: "2026-09-05T00:00:00Z", version: 1, warning_threshold_percent: 80, redemption_codes_enabled: false, group_budget: { group_id: "group-1", group_name: "财务部", limit_hundredths: 100000, consumed_hundredths: 90000, reserved_hundredths: 1000, available_hundredths: 9000 } })),
      listCreditLedger: vi.fn(async (cursor = "") => cursor ? { items: [{ id: "older", type: "adjustment", amount_hundredths: 100, resulting_balance_hundredths: 60000, credit_day: "2026-09-04", created_at: "2026-09-04T09:00:00Z" }] } : { items: [{ id: "newer", type: "daily_allocation", amount_hundredths: 60000, resulting_balance_hundredths: 60000, credit_day: "2026-09-04", created_at: "2026-09-04T10:00:00Z" }], next_cursor: "newer" }),
    } as unknown as PlatformApi;
    const wrapper = mount(CreditPanel, { props: { open: true }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    const loadMore = wrapper.findAll("button").find((button) => button.text().includes("加载更多"));
    expect(loadMore).toBeDefined();
    await loadMore!.trigger("click");
    await flushPromises();

    expect(api.listCreditLedger).toHaveBeenNthCalledWith(2, "newer");
    expect(wrapper.text()).toContain("管理员调整");
    expect(wrapper.text()).toContain("可用额度");
    expect(wrapper.text()).toContain("今日额度使用已达到 80% 提醒线");
    expect(wrapper.text()).toContain("部门预算：财务部");
    expect(wrapper.find(".credit-redeem").exists()).toBe(false);
  });
});
