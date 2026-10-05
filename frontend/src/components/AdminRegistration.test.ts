// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import { platformApiKey, type PlatformApi, type RegistrationMethod } from "../api/client";
import AdminRegistration from "./AdminRegistration.vue";
const method: RegistrationMethod = { provider: "feishu", enabled: false, ready: true, app_id: "app", app_secret_configured: true, tenant_key: "enterprise", official_account_id: "", verification_token_configured: false, encoding_aes_key_configured: false, version: 1, callback_url: "https://workspace.test/api/v1/registration/feishu/callback" };
function fixture(available = true, locale = "zh-CN") {
  const api = { getRegistrationSettings: vi.fn(async () => ({ available, items: [method] })), updateRegistrationMethod: vi.fn(async () => ({ ...method, enabled: true, version: 2 })) };
  const wrapper = mount(AdminRegistration, { global: { plugins: [createAppI18n({ getItem: () => locale }, locale)], provide: { [platformApiKey as symbol]: api as unknown as PlatformApi } } });
  return { wrapper, api };
}
describe("registration governance", () => {
  it("retains write-only secrets and requires a reason before changing enablement", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    expect(wrapper.text()).toContain("已验证企业");
    const passwords = wrapper.findAll('input[type="password"]'); expect(passwords).toHaveLength(1); expect((passwords[0]!.element as HTMLInputElement).value).toBe("");
    const save = wrapper.get('button[type="submit"]'); expect(save.attributes("disabled")).toBeDefined();
    await wrapper.get('[role="switch"]').trigger("click");
    await wrapper.findAll("input").at(-1)!.setValue("开放内部员工注册"); await wrapper.get("form").trigger("submit"); await flushPromises();
    expect(api.updateRegistrationMethod).toHaveBeenCalledWith("feishu", expect.objectContaining({ enabled: true, app_secret: "", expected_version: 1, reason: "开放内部员工注册" }), expect.any(AbortSignal));
    wrapper.unmount();
  });
  it("explains unavailable deployment setup without offering nonfunctional controls", async () => {
    const { wrapper } = fixture(false, "en-US"); await flushPromises();
    expect(wrapper.text()).toContain("deployment administrator"); expect(wrapper.find('input[type="password"]').exists()).toBe(false); wrapper.unmount();
  });
  it("reloads the persisted version and clears typed secrets after synchronization fails", async () => {
    const { wrapper, api } = fixture(); await flushPromises();
    api.updateRegistrationMethod.mockRejectedValueOnce(new Error("identity unavailable"));
    await wrapper.get('input[type="password"]').setValue("new-secret");
    await wrapper.findAll("input").at(-1)!.setValue("rotate application"); await wrapper.get("form").trigger("submit"); await flushPromises();
    expect(api.getRegistrationSettings).toHaveBeenCalledTimes(2); expect(wrapper.text()).toContain("配置未完成"); expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe(""); wrapper.unmount();
  });
});
