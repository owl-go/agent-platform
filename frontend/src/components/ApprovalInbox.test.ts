// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, platformApiKey, type CommandApproval, type PlatformApi } from "../api/client";
import { embeddedSessionApprovalID } from "../commandApprovalPlacement";
import { createAppI18n } from "../i18n";
import ApprovalInbox from "./ApprovalInbox.vue";

beforeEach(() => { vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible"); });
afterEach(() => { embeddedSessionApprovalID.value = undefined; document.querySelector("#session-command-approval-slot")?.remove(); vi.useRealTimers(); vi.restoreAllMocks(); });

describe("ApprovalInbox", () => {
  it("checks idle approvals less frequently and pauses while the tab is hidden", async () => {
    vi.useFakeTimers();
    const api = { listCommandApprovals: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await vi.advanceTimersByTimeAsync(25_000);
    expect(api.listCommandApprovals).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(5_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    document.dispatchEvent(new Event("visibilitychange"));
    await flushPromises();
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(3);
    wrapper.unmount();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(3);
  });

  it("checks pending approvals quickly without overlapping slow requests or restarting after unmount", async () => {
    vi.useFakeTimers();
    const approval = { id: "approval-pending", state: "pending", identity: "user", connector_name: "Connector", operation: "send", target: "target", expires_at: "2026-09-06T12:00:00Z" } as CommandApproval;
    let finish: ((items: CommandApproval[]) => void) | undefined;
    const api = { listCommandApprovals: vi.fn<PlatformApi["listCommandApprovals"]>()
      .mockResolvedValueOnce([approval])
      .mockImplementationOnce(() => new Promise<CommandApproval[]>((resolve) => { finish = resolve; })) } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    await vi.advanceTimersByTimeAsync(5_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    wrapper.unmount(); finish?.([]); await flushPromises();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
  });
  it("explains a Feishu message approval in Chinese without exposing raw command details", async () => {
    vi.useFakeTimers();
    const approval: CommandApproval = { id: "approval-1", execution_kind: "run", execution_id: "run-1", connector_name: "飞书", operation: "im_messages_send", target: "oc_chat-1", redacted_arguments: "im +messages-send [arguments redacted]", state: "pending", expires_at: "2026-09-05T12:00:00Z", version: 4 };
    const decideCommandApproval = vi.fn(async () => ({ ...approval, state: "approved" as const, identity: "bot" as const }));
    const api = { listCommandApprovals: vi.fn().mockResolvedValueOnce([approval]).mockResolvedValueOnce([]), decideCommandApproval } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.text()).toContain("请确认飞书操作");
    expect(wrapper.text()).toContain("发送消息");
    expect(wrapper.text()).toContain("目标群聊标识：oc_chat-1");
    expect(wrapper.text()).toContain("拒绝则不会执行这次发送");
    expect(wrapper.text()).toContain("消息内容及命令参数不会在此处展示");
    expect(wrapper.text()).not.toContain("im_messages_send");
    expect(wrapper.text()).not.toContain("im +messages-send");
    expect(wrapper.text()).not.toContain("arguments redacted");
    await wrapper.get("select").setValue("bot");
    await wrapper.findAll("button")[1]!.trigger("click");
    await flushPromises();

    expect(decideCommandApproval).toHaveBeenCalledWith("approval-1", "approved", "bot", 4);
    wrapper.unmount();
  });

  it("locks a single-identity command to the persisted identity", async () => {
    vi.useFakeTimers();
    const approval: CommandApproval = { id: "approval-2", execution_kind: "session", execution_id: "42", connector_name: "Feishu CLI", operation: "send message", target: "chat-1", redacted_arguments: "message send [arguments redacted]", state: "pending", identity: "bot", expires_at: "2026-09-05T12:00:00Z", version: 1 };
    const api = { listCommandApprovals: vi.fn().mockResolvedValue([approval]), decideCommandApproval: vi.fn() } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    const identity = wrapper.get<HTMLSelectElement>("select");
    expect(identity.element.value).toBe("bot");
    expect(identity.attributes("disabled")).toBeDefined();
    wrapper.unmount();
  });

  it("disables repeated decisions and removes an accepted approval before the execution consumes it", async () => {
    const approval: CommandApproval = { id: "approval-3", execution_kind: "run", execution_id: "run-3", connector_name: "飞书", operation: "task_create", target: "owner", redacted_arguments: "", state: "pending", expires_at: "2026-09-05T12:00:00Z", version: 1 };
    let finish: ((value: CommandApproval) => void) | undefined;
    const decideCommandApproval = vi.fn(() => new Promise<CommandApproval>((resolve) => { finish = resolve; }));
    const api = { listCommandApprovals: vi.fn().mockResolvedValueOnce([approval]).mockResolvedValueOnce([{ ...approval, state: "approved" }]), decideCommandApproval } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.findAll("button")[1]!.trigger("click");
    expect(wrapper.findAll("button").every((button) => button.attributes("disabled") !== undefined)).toBe(true);
    await wrapper.findAll("button")[1]!.trigger("click");
    expect(decideCommandApproval).toHaveBeenCalledTimes(1);

    finish?.({ ...approval, state: "approved" });
    await flushPromises();
    expect(wrapper.find(".approval-card").exists()).toBe(false);
    expect(wrapper.get("[role=status]").text()).toContain("已批准本次操作");
    wrapper.unmount();
  });

  it("explains a stale decision and refreshes the approval list", async () => {
    const approval: CommandApproval = { id: "approval-4", execution_kind: "run", execution_id: "run-4", connector_name: "飞书", operation: "task_create", target: "owner", redacted_arguments: "", state: "pending", expires_at: "2026-09-05T12:00:00Z", version: 1 };
    const api = { listCommandApprovals: vi.fn().mockResolvedValueOnce([approval]).mockResolvedValueOnce([]), decideCommandApproval: vi.fn().mockRejectedValue(new ApiError("conflict", 412, "version_conflict")) } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.findAll("button")[1]!.trigger("click");
    await flushPromises();
    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    expect(wrapper.find(".approval-card").exists()).toBe(false);
    expect(wrapper.get("[role=alert]").text()).toContain("已处理或过期");
    wrapper.unmount();
  });

  it("keeps an approval retryable after a network failure", async () => {
    const approval: CommandApproval = { id: "approval-5", execution_kind: "run", execution_id: "run-5", connector_name: "飞书", operation: "task_create", target: "owner", redacted_arguments: "", state: "pending", expires_at: "2026-09-05T12:00:00Z", version: 1 };
    const api = { listCommandApprovals: vi.fn().mockResolvedValue([approval]), decideCommandApproval: vi.fn().mockRejectedValue(new TypeError("Failed to fetch")) } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    await wrapper.findAll("button")[1]!.trigger("click");
    await flushPromises();
    expect(wrapper.find(".approval-card").exists()).toBe(true);
    expect(wrapper.findAll("button")[1]!.attributes("disabled")).toBeUndefined();
    expect(wrapper.get("[role=alert]").text()).toContain("提交失败");
    wrapper.unmount();
  });

  it("explains Feishu task creation before one-use approval", async () => {
    vi.useFakeTimers();
    const approval: CommandApproval = { id: "approval-task", execution_kind: "session", execution_id: "42", connector_name: "飞书", operation: "task_create", target: "ou_assignee", redacted_arguments: "task +create [arguments redacted]", state: "pending", identity: "user", expires_at: "2026-09-05T12:00:00Z", version: 1 };
    const api = { listCommandApprovals: vi.fn().mockResolvedValue([approval]), decideCommandApproval: vi.fn() } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    expect(wrapper.text()).toContain("创建任务");
    expect(wrapper.text()).toContain("负责人");
    expect(wrapper.text()).toContain("ou_assignee");
    expect(wrapper.text()).not.toContain("task_create");
    wrapper.unmount();
  });

  it("places the current Session approval inside the conversation composer and keeps background approvals global", async () => {
    const target = document.createElement("div");
    target.id = "session-command-approval-slot";
    document.body.append(target);
    const current = { id: "approval-session", execution_kind: "session", execution_id: "42", connector_name: "Feishu CLI", operation: "send", target: "current-chat", redacted_arguments: "message [redacted]", state: "pending", identity: "user", expires_at: "2026-09-08T12:00:00Z", version: 1 } as CommandApproval;
    const background = { ...current, id: "approval-run", execution_kind: "run", execution_id: "run-1", target: "background-chat" } as CommandApproval;
    const api = { listCommandApprovals: vi.fn().mockResolvedValueOnce([]).mockResolvedValue([current, background]) } as unknown as PlatformApi;

    const wrapper = mount(ApprovalInbox, { attachTo: document.body, global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();
    embeddedSessionApprovalID.value = "42";
    await flushPromises();

    expect(api.listCommandApprovals).toHaveBeenCalledTimes(2);
    expect(target.textContent).toContain("current-chat");
    expect(target.textContent).not.toContain("background-chat");
    expect(wrapper.text()).toContain("background-chat");
    expect(wrapper.text()).not.toContain("current-chat");
    wrapper.unmount();
  });
});
