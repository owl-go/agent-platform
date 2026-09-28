// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import ExecutionStatusBar from "./ExecutionStatusBar.vue";

describe("ExecutionStatusBar", () => {
  it("renders real execution metadata and exposes a persistent stop action", async () => {
    const wrapper = mount(ExecutionStatusBar, {
      props: {
        state: "generating",
        elapsedMs: 62_000,
        model: "GPT",
        canStop: true,
        creditConsumption: { total_hundredths: 1, stages: [{ stage_position: 1, provider_model: "gpt", runtime_engine: "codex", input_tokens: 7200, output_tokens: 1400, usage_reported: true, input_multiplier_micros: 1, output_multiplier_micros: 1, fallback_hundredths: 1, amount_hundredths: 1, estimated: false, rate_revision_id: "rate" }] },
      },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.attributes("role")).toBe("status");
    expect(wrapper.classes()).toContain("is-running");
    expect(wrapper.text()).toContain("1:02");
    expect(wrapper.text()).toContain("8.6k tokens");
    await wrapper.get(".execution-status-stop").trigger("click");
    expect(wrapper.emitted("stop")).toHaveLength(1);
  });
});
