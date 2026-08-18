import { describe, expect, it } from "vitest";
import { formatCount, formatDuration, formatModelCost, formatTokenUsage, resolveInitialLocale, runStateLabel, runStates } from "./index";

describe("locale boundary", () => {
  it("uses a persisted supported locale, browser Chinese, then deterministic English fallback", () => {
    expect(resolveInitialLocale("zh-CN", "en-US")).toBe("zh-CN");
    expect(resolveInitialLocale(null, "zh-Hans-CN")).toBe("zh-CN");
    expect(resolveInitialLocale(null, "fr-FR")).toBe("en-US");
    expect(resolveInitialLocale("unsupported", "en-US")).toBe("en-US");
  });

  it("formats duration, Token Usage, and Model Cost without changing backend values", () => {
    expect(formatDuration(90_000, "en-US")).toMatch(/2 min/);
    expect(formatTokenUsage(12_500, "en-US")).toMatch(/12\.5K/i);
    expect(formatCount(12_500, "en-US")).toBe("12,500");
    expect(formatModelCost(3.18, "en-US")).toContain("$3.18");
    expect(formatModelCost(3.18, "zh-CN")).toContain("3.18");
  });

  it("maps every stable backend Run state without changing its enum value", () => {
    expect(runStates).toEqual(["queued", "provisioning", "running", "waiting_confirmation", "interrupting", "interrupted", "resuming", "completed", "failed", "cancelled"]);
    for (const state of runStates) {
      expect(runStateLabel(state, "zh-CN")).not.toBe("");
      expect(runStateLabel(state, "en-US")).not.toBe("");
    }
  });
});
