import { describe, expect, it } from "vitest";
import type { ConversationMessage } from "./conversationThread";
import { hasTaskWorkspaceContent, latestTaskWorkspaceMessage } from "./taskWorkspace";

const message = (id: string, role: "user" | "assistant", extras: Partial<ConversationMessage> = {}): ConversationMessage => ({
  id,
  role,
  content: "",
  state: "succeeded",
  timestamp: "2026-09-28T08:00:00Z",
  ...extras,
});

describe("task workspace selection", () => {
  it("only selects assistant turns with inspectable task content", () => {
    expect(hasTaskWorkspaceContent(message("plain", "assistant", { content: "plain answer" }))).toBe(false);
    expect(hasTaskWorkspaceContent(message("user", "user", { attachments: [{ id: "file", name: "input.txt", content_type: "text/plain", size: 1, sha256: "digest", image: false }] }))).toBe(false);
    expect(hasTaskWorkspaceContent(message("planned", "assistant", { executionPlan: {
      id: "plan", state: "pending", objective: "Prepare report", steps: [], resources: [], side_effects: [], reasons: [], estimated_model_calls: 1, estimated_credit_hundredths: 100, generation_credit_hundredths: 0, generator: "platform_rules", created_at: "2026-09-28T08:00:00Z", version: 1,
    } }))).toBe(true);
  });

  it("uses the newest task-bearing assistant turn", () => {
    const items = [
      message("old", "assistant", { activities: [{ id: 1, label: "Started", items: [] }] }),
      message("plain", "assistant", { content: "Answer" }),
      message("new", "assistant", { artifacts: [{ id: "artifact", kind: "file", name: "report.md", path: "report.md", size: 12, expired: false, created_at: "2026-09-28T08:01:00Z" }] }),
    ];

    expect(latestTaskWorkspaceMessage(items)?.id).toBe("new");
  });
});
