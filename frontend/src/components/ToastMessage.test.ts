// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import ToastMessage from "./ToastMessage.vue";

describe("ToastMessage", () => {
  afterEach(() => { vi.useRealTimers(); });

  it("uses an assertive alert for errors and supports manual dismissal", async () => {
    const wrapper = mount(ToastMessage, { props: { kind: "error", title: "失败", message: "保存失败", closeLabel: "关闭" } });
    expect(wrapper.get('[role="alert"]').text()).toContain("保存失败");
    await wrapper.get('button[aria-label="关闭"]').trigger("click");
    expect(wrapper.emitted("dismiss")).toHaveLength(1);
    wrapper.unmount();
  });

  it("dismisses transient feedback after its configured duration", () => {
    vi.useFakeTimers();
    const wrapper = mount(ToastMessage, { props: { kind: "success", title: "成功", message: "已保存", closeLabel: "关闭", duration: 3000 } });
    expect(wrapper.get('[role="status"]').attributes("aria-live")).toBe("polite");
    vi.advanceTimersByTime(3000);
    expect(wrapper.emitted("dismiss")).toHaveLength(1);
    wrapper.unmount();
  });
});
