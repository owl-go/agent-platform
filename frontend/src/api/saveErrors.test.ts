import { describe, expect, it } from "vitest";
import { createAppI18n } from "../i18n";
import { ApiError } from "./client";
import { saveErrorMessage } from "./saveErrors";

const i18n = createAppI18n();
i18n.global.locale.value = "zh-CN";
const t = i18n.global.t;
describe("save failure feedback", () => {
  it.each([
    [new ApiError("conflict", 409, "model_provider_name_conflict"), "供应商名称已存在"],
    [new ApiError("conflict", 412, "version_conflict"), "数据已被更新"],
    [new ApiError("validation", 422, "invalid_input", "", undefined, "API Key is required"), "API Key 不能为空"],
    [new ApiError("forbidden", 403, "access_denied"), "没有保存此资源的权限"],
    [new ApiError("unauthenticated", 401, "authentication_required"), "重新登录"],
    [new ApiError("validation", 413, "too_large"), "大小限制"],
    [new ApiError("rate_limited", 429, "insufficient_credits"), "可用积分不足"],
    [new ApiError("network", 0, "network_error"), "网络连接失败"],
  ])("provides a cause and recovery action", (cause, expected) => {
    expect(saveErrorMessage(cause, t)).toContain(expected);
  });
  it("preserves a public validation cause not yet translated", () => {
    expect(saveErrorMessage(new ApiError("validation", 422, "invalid_input", "", undefined, "Custom guidance is required"), t)).toBe("Custom guidance is required");
  });
  it("retains the request ID for an unavailable service", () => {
    const message = saveErrorMessage(new ApiError("unavailable", 500, "request_failed", "trace-42"), t);
    expect(message).toContain("服务暂时不可用");
    expect(message).toContain("请求编号: trace-42");
  });
  it("does not render raw exceptions", () => {
    const message = saveErrorMessage(new Error("SQL private token"), t);
    expect(message).toContain("未收到可识别的错误原因");
    expect(message).not.toContain("private token");
  });
});
