// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import IconPicker from "./IconPicker.vue";

describe("IconPicker", () => {
  it("keeps the upload control compact without a redundant help panel", () => {
    const wrapper = mount(IconPicker, { props: { modelValue: "sparkles", fallback: "sparkles" } });

    expect(wrapper.find(".icon-picker-help").exists()).toBe(false);
    expect(wrapper.find('[data-testid="icon-picker-upload"] span').exists()).toBe(false);
    expect(wrapper.get('[data-testid="icon-picker-upload"]').classes()).toContain("icon-picker-option");
    expect(wrapper.get('[data-testid="icon-picker-upload"] > svg').classes()).toContain("icon-picker-upload-icon");
    wrapper.unmount();
  });

  it("selects a preset icon", async () => {
    const wrapper = mount(IconPicker, { props: { modelValue: "sparkles", fallback: "sparkles" } });

    await wrapper.get('button[aria-label="code"]').trigger("click");

    expect(wrapper.emitted("update:modelValue")).toEqual([["code"]]);
    wrapper.unmount();
  });

  it("loads a supported custom image as a data URL", async () => {
    const wrapper = mount(IconPicker, { props: { modelValue: "sparkles", fallback: "sparkles" } });
    const file = new File(["png-content"], "icon.png", { type: "image/png" });
    const input = wrapper.get('[data-testid="icon-picker-file"]');
    Object.defineProperty(input.element, "files", { configurable: true, value: [file] });

    await input.trigger("change");
    await flushPromises();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(wrapper.emitted("update:modelValue")?.at(-1)?.[0]).toMatch(/^data:image\/png;base64,/);
    wrapper.unmount();
  });

  it("rejects unsupported image formats and oversized files", async () => {
    const wrapper = mount(IconPicker, { props: { modelValue: "sparkles", fallback: "sparkles" } });
    const input = wrapper.get('[data-testid="icon-picker-file"]');
    const invalid = new File(["svg"], "icon.svg", { type: "image/svg+xml" });
    Object.defineProperty(input.element, "files", { configurable: true, value: [invalid] });
    await input.trigger("change");
    const oversized = new File([new Uint8Array(384 * 1024 + 1)], "large.png", { type: "image/png" });
    Object.defineProperty(input.element, "files", { configurable: true, value: [oversized] });
    await input.trigger("change");

    expect(wrapper.emitted("update:modelValue")).toBeUndefined();
    wrapper.unmount();
  });
});
