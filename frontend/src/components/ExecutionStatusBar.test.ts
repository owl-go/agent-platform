// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import ExecutionStatusBar from "./ExecutionStatusBar.vue";

describe("ExecutionStatusBar", () => {
  afterEach(() => vi.useRealTimers());

  it("renders real execution metadata and exposes a persistent stop action", async () => {
    const wrapper = mount(ExecutionStatusBar, {
      props: {
        state: "generating",
        elapsedMs: 62_000,
        model: "GPT",
        currentActivity: "正在调用工具",
        modelCallCount: 1,
        canStop: true,
        creditConsumption: { total_hundredths: 1, stages: [{ stage_position: 1, provider_model: "gpt", runtime_engine: "codex", input_tokens: 7200, output_tokens: 1400, usage_reported: true, input_multiplier_micros: 1, output_multiplier_micros: 1, fallback_hundredths: 1, amount_hundredths: 1, estimated: false, rate_revision_id: "rate" }] },
      },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.attributes("role")).toBe("status");
    expect(wrapper.classes()).toContain("is-running");
    expect(wrapper.text()).toContain("1:02");
    expect(wrapper.text()).toContain("正在调用工具");
    expect(wrapper.text()).toContain("模型调用 1 次");
    expect(wrapper.text()).toContain("完成后结算");
    expect(wrapper.text()).not.toContain("8.6k tokens");
    await wrapper.get(".execution-status-stop").trigger("click");
    expect(wrapper.emitted("stop")).toHaveLength(1);

    await wrapper.setProps({ state: "completed", canStop: false });
    expect(wrapper.text()).toContain("8.6k tokens");
  });

  it("stops the activity pulse and shows the last update time after ten seconds without activity", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-28T06:00:00Z"));
    const wrapper = mount(ExecutionStatusBar, {
      props: { state: "running", lastActivityAt: Date.now(), currentActivity: "正在处理" },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    await vi.advanceTimersByTimeAsync(10_000);

    expect(wrapper.classes()).toContain("is-stale");
    expect(wrapper.text()).toContain("任务仍在运行 · 最后更新于");
  });

  it("does not call a pending user decision a stale running task", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-28T06:00:20Z"));
    const wrapper = mount(ExecutionStatusBar, {
      props: { state: "waiting_for_user", lastActivityAt: Date.now() - 20_000 },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.text()).toContain("等待用户操作");
    expect(wrapper.text()).not.toContain("任务仍在运行");
    expect(wrapper.classes()).not.toContain("is-stale");
    wrapper.unmount();
  });
});
