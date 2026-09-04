// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import CreditConsumption from "./CreditConsumption.vue";

describe("CreditConsumption", () => {
  it("shows the collapsed total and reported Token breakdown", async () => {
    const wrapper = mount(CreditConsumption, {
      props: { value: { total_hundredths: 173, stages: [{ stage_position: 1, provider_model: "gpt-5", runtime_engine: "codex", input_tokens: 12_345, output_tokens: 5_000, usage_reported: true, input_multiplier_micros: 1_000_000, output_multiplier_micros: 1_000_000, fallback_hundredths: 1_000, amount_hundredths: 173, estimated: false, rate_revision_id: "default-v1" }] } },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.get("summary").text()).toBe("共消耗 ✧ 1.73");
    expect(wrapper.text()).toContain("输入 12,345 token × 1.00");
    expect(wrapper.text()).toContain("输出 5,000 token × 1.00");
  });

  it("labels missing Usage as an estimate without fabricated Token values", () => {
    const wrapper = mount(CreditConsumption, {
      props: { value: { total_hundredths: 1_000, stages: [{ stage_position: 1, provider_model: "model", runtime_engine: "openclaw", input_tokens: 0, output_tokens: 0, usage_reported: false, input_multiplier_micros: 1_000_000, output_multiplier_micros: 1_000_000, fallback_hundredths: 1_000, amount_hundredths: 1_000, estimated: true, rate_revision_id: "default-v1" }] } },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.text()).toContain("按回退值 10.00 积分估算");
    expect(wrapper.text()).not.toContain("输入 0 token");
  });
});
