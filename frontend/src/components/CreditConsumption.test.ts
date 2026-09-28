// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import CreditConsumption from "./CreditConsumption.vue";

describe("CreditConsumption", () => {
  it("shows a privacy-safe stage summary without Tokens, rates, or model identifiers", async () => {
    const wrapper = mount(CreditConsumption, {
      props: { value: { total_hundredths: 273, stages: [{ stage_position: 1, provider_model: "gpt-5", runtime_engine: "codex", input_tokens: 12_345, output_tokens: 5_000, usage_reported: true, input_multiplier_micros: 1_000_000, output_multiplier_micros: 1_000_000, fallback_hundredths: 1_000, amount_hundredths: 173, estimated: false, rate_revision_id: "default-v1" }, { stage_position: 2, provider_model: "secret-model", runtime_engine: "claude", input_tokens: 0, output_tokens: 0, usage_reported: false, input_multiplier_micros: 2_000_000, output_multiplier_micros: 2_000_000, fallback_hundredths: 100, amount_hundredths: 100, estimated: true, rate_revision_id: "private-rate" }] } },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.text()).toContain("共消耗 ✧ 2.73");
    expect(wrapper.find("details").exists()).toBe(true);
    expect(wrapper.text()).toContain("阶段 2");
    expect(wrapper.text()).toContain("估算结算");
    expect(wrapper.text()).not.toContain("token");
    expect(wrapper.text()).not.toContain("gpt-5");
    expect(wrapper.text()).not.toContain("private-rate");
  });

  it("explains a failed execution without model consumption", () => {
    const wrapper = mount(CreditConsumption, { props: { state: "failed", value: { total_hundredths: 0, stages: [] } }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
    expect(wrapper.text()).toContain("未记录模型消耗");
    expect(wrapper.text()).not.toContain("停止前已产生");
  });

  it("treats an omitted protobuf zero total as zero Credits", () => {
    const wrapper = mount(CreditConsumption, {
      props: { value: { total_hundredths: undefined as unknown as number, stages: [] } },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
    });

    expect(wrapper.text()).toBe("共消耗 ✧ 0.00");
    expect(wrapper.text()).not.toContain("NaN");
  });
});
