// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import CreditConsumption from "./CreditConsumption.vue";

describe("CreditConsumption", () => {
  it("shows only the total Credits consumed", () => {
    const wrapper = mount(CreditConsumption, {
      props: { value: { total_hundredths: 173, stages: [{ stage_position: 1, provider_model: "gpt-5", runtime_engine: "codex", input_tokens: 12_345, output_tokens: 5_000, usage_reported: true, input_multiplier_micros: 1_000_000, output_multiplier_micros: 1_000_000, fallback_hundredths: 1_000, amount_hundredths: 173, estimated: false, rate_revision_id: "default-v1" }] } },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.text()).toBe("共消耗 ✧ 1.73");
    expect(wrapper.find("details").exists()).toBe(false);
    expect(wrapper.text()).not.toContain("token");
    expect(wrapper.text()).not.toContain("gpt-5");
  });
});
