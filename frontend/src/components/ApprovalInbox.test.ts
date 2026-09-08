// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { platformApiKey, type CommandApproval, type PlatformApi } from "../api/client";
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
    const approval = { id: "approval-pending", identity: "user", connector_name: "Connector", operation: "send", target: "target", expires_at: "2026-09-06T12:00:00Z" } as CommandApproval;
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
  it("shows redacted command details and submits one-use identity consent", async () => {
    vi.useFakeTimers();
    const approval: CommandApproval = { id: "approval-1", execution_kind: "run", execution_id: "run-1", connector_name: "Feishu CLI", operation: "send message", target: "chat-1", redacted_arguments: "--content [REDACTED]", state: "pending", expires_at: "2026-09-05T12:00:00Z", version: 4 };
    const decideCommandApproval = vi.fn(async () => ({ ...approval, state: "approved" as const, identity: "bot" as const }));
    const api = { listCommandApprovals: vi.fn().mockResolvedValueOnce([approval]).mockResolvedValueOnce([]), decideCommandApproval } as unknown as PlatformApi;
    const wrapper = mount(ApprovalInbox, { global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: api } } });
    await flushPromises();

    expect(wrapper.text()).toContain("[REDACTED]");
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

  it("places the current Session approval beside the conversation composer and keeps background approvals global", async () => {
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
