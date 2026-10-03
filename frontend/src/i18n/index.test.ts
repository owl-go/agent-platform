// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from ".";

describe("workflow settings translations", () => {
  afterEach(() => vi.restoreAllMocks());

  it.each([
    ["zh-CN", "支持 https:// 地址和 git@host:path 格式的 SSH 地址。"],
    ["en-US", "Supports HTTPS URLs and SSH addresses in git@host:path format."],
  ] as const)("renders the SCP-style Git example in %s", (locale, expected) => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const i18n = createAppI18n({ getItem: () => locale }, locale);

    expect(i18n.global.t("workflows.gitURLHelp")).toBe(expected);
    expect(consoleError).not.toHaveBeenCalled();
  });

  it.each([
    ["zh-CN", "配置消息渠道", "刷新", "2 个环境变量"],
    ["en-US", "Set up message channels", "Refresh", "2 environment variables"],
  ] as const)("renders quick channel setup and bot mentions in %s", (locale, action, refresh, environment) => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const consoleWarn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const i18n = createAppI18n({ getItem: () => locale }, locale);

    expect(i18n.global.t("workflows.configureChannels")).toBe(action);
    expect(i18n.global.t("common.refresh")).toBe(refresh);
    expect(i18n.global.t("workflows.environmentSummary", { count: 2 })).toBe(environment);
    for (const key of ["verifyInstruction", "audienceHint", "setup.feishu", "setup.wecom", "setup.qqbot"]) {
      expect(i18n.global.t(`channels.${key}`)).not.toContain("{'@'}");
    }
    if (locale === "zh-CN") expect(i18n.global.t("channels.audienceHint")).toContain("@ 机器人");
    expect(consoleError).not.toHaveBeenCalled();
    expect(consoleWarn).not.toHaveBeenCalled();
  });
});
