// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import ScanRegistrationPage from "./ScanRegistrationPage.vue";
const id = "a".repeat(43);
function mountPage(locale = "zh-CN") { window.history.replaceState({}, "", `/register/wechat?id=${id}`); return mount(ScanRegistrationPage, { global: { plugins: [createAppI18n({ getItem: () => locale }, locale)] } }); }
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });
describe("WeChat scan registration", () => {
  it("polls only the browser-bound attempt and stops after unmount", async () => {
    vi.useFakeTimers(); const fetcher = vi.fn(async () => ({ ok: true, json: async () => ({ status: "waiting", login_code: "AW-ABCD-EFGH-JKLM", qr_url: "https://open.weixin.qq.com/qr/code?username=gh_fixture", expires_at: new Date(Date.now() + 300_000).toISOString() }) })); vi.stubGlobal("fetch", fetcher);
    const wrapper = mountPage(); await flushPromises(); expect(wrapper.text()).toContain("关注公众号"); expect(wrapper.get("img").attributes("src")).toContain("/qr/code");
    expect(wrapper.get("code").text()).toBe("AW-ABCD-EFGH-JKLM");
    expect(fetcher).toHaveBeenCalledWith(expect.stringContaining(`id=${id}`), expect.objectContaining({ credentials: "same-origin", cache: "no-store" }));
    wrapper.unmount(); await vi.advanceTimersByTimeAsync(3000); expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("copies the code shown by the browser-bound status endpoint", async () => {
    const copy = vi.fn(async () => undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText: copy } });
    vi.stubGlobal("fetch", vi.fn(async () => ({ ok: true, json: async () => ({ status: "waiting", login_code: "AW-ABCD-EFGH-JKLM", qr_url: "https://open.weixin.qq.com/qr/code?username=gh_fixture", expires_at: new Date(Date.now() + 300_000).toISOString() }) })));
    const wrapper = mountPage(); await flushPromises();
    const button = wrapper.findAll("button").find((entry) => entry.text() === "复制")!;
    await button.trigger("click"); await flushPromises();
    expect(copy).toHaveBeenCalledWith("AW-ABCD-EFGH-JKLM");
    expect(wrapper.text()).toContain("已复制");
    wrapper.unmount();
  });
  it.each(["", "123456", "AW-ABCD-EFGH-JKL0"])("refuses an invalid login challenge (%s)", async (login_code) => {
    vi.stubGlobal("fetch", vi.fn(async () => ({ ok: true, json: async () => ({ status: "waiting", login_code, qr_url: "https://open.weixin.qq.com/qr/code?username=gh_fixture", expires_at: new Date(Date.now() + 300_000).toISOString() }) })));
    const wrapper = mountPage(); await flushPromises();
    expect(wrapper.text()).toContain("登录未完成");
    expect(wrapper.find("code").exists()).toBe(false);
    wrapper.unmount();
  });
  it("shows expiry and a return action in English without restarting OIDC", async () => {
    const fetcher = vi.fn(async () => ({ ok: false, status: 410 })); vi.stubGlobal("fetch", fetcher);
    const wrapper = mountPage("en-US"); await flushPromises(); expect(wrapper.text()).toContain("code expired"); expect(wrapper.get("a").text()).toBe("Back to sign in"); expect(wrapper.find("img").exists()).toBe(false); wrapper.unmount();
  });
  it("accepts a verified follow proof only through the protected completion request", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ status: "verified", login_code: "AW-ABCD-EFGH-JKLM", qr_url: "https://open.weixin.qq.com/qr/code?username=gh_fixture", expires_at: new Date(Date.now() + 300_000).toISOString() }) }).mockResolvedValueOnce({ ok: false }); vi.stubGlobal("fetch", fetcher);
    const wrapper = mountPage(); await flushPromises(); expect(fetcher).toHaveBeenLastCalledWith(expect.stringContaining("complete?id="), expect.objectContaining({ method: "POST", headers: { "X-Registration-Request": "1" }, credentials: "same-origin" })); expect(wrapper.text()).toContain("登录未完成"); wrapper.unmount();
  });
});
