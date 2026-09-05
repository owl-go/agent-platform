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
});
