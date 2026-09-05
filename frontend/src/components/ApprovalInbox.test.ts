// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { platformApiKey, type CommandApproval, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import ApprovalInbox from "./ApprovalInbox.vue";

afterEach(() => { vi.useRealTimers(); });

describe("ApprovalInbox", () => {
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
});
