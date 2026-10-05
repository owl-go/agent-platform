// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory } from "vue-router";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type CreditPolicy, type IdentityGroup, type PlatformApi, type UserAccount } from "../api/client";
import { createAppI18n } from "../i18n";
import { createAppRouter } from "../router";
import UsersPage from "./UsersPage.vue";

const policy: CreditPolicy = { default_daily_allocation_hundredths: 60_000, warning_threshold_percent: 80, redemption_codes_enabled: false, version: 1, updated_at: "2026-09-28T00:00:00Z" };
const group: IdentityGroup = { id: "group-1", external_id: "kc-finance", name: "财务部", path: "/财务部", department: true, daily_credit_limit_hundredths: 100_000, member_count: 1, last_synced_at: "2026-09-28T00:00:00Z", version: 1 };
const user: UserAccount = { id: "user-1", username: "analyst", email: "analyst@example.test", display_name: "Analyst", administrator: false, resource_publisher: true, groups: [group], enabled: true, created_at: "2026-09-28T00:00:00Z", version: 1, credit_balance: { total_hundredths: 6_000, reserved_hundredths: 1_000, available_hundredths: 5_000, daily_remaining_hundredths: 6_000, persistent_hundredths: 0, today_consumed_hundredths: 54_000, daily_allocation_hundredths: 60_000, credit_day: "2026-09-28", timezone: "Asia/Shanghai", next_allocation_at: "2026-09-29T00:00:00Z", version: 1, warning_threshold_percent: 80, redemption_codes_enabled: false, group_budget: { group_id: group.id, group_name: group.name, limit_hundredths: 100_000, consumed_hundredths: 95_000, reserved_hundredths: 0, available_hundredths: 5_000 } } };

describe("UsersPage enterprise Credits", () => {
  it("shows policy and budget warnings while hiding the disabled Redemption Code channel", async () => {
    const api = { listUsers: vi.fn(async () => [user]), getCreditPolicy: vi.fn(async () => policy), listIdentityGroups: vi.fn(async () => [group]), listGovernanceAuditEvents: vi.fn(async () => []) } as unknown as PlatformApi;
    const router = createAppRouter(createMemoryHistory());
    await router.push("/admin/users");
    const wrapper = mount(UsersPage, { global: { plugins: [router, createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.text()).toContain("企业额度策略");
    expect(wrapper.text()).toContain("90%");
    expect(wrapper.text()).toContain("50.00");
    expect(wrapper.text()).not.toContain("60.00");
    expect(wrapper.findAll(".admin-tabs button").map((item) => item.text())).toEqual(["用户", "注册方式", "部门与群组", "治理审计", "模型倍率"]);
    expect(wrapper.text()).toContain("受 财务部 部门预算约束");
    expect(wrapper.text()).not.toContain("兑换码仅本次显示");
    wrapper.unmount();
  });
});
