import type { ConversationMessage } from "./conversationThread";

export function hasTaskWorkspaceContent(message: ConversationMessage): boolean {
  if (message.role !== "assistant") return false;
  return Boolean(
    message.executionPlan ||
    message.activities?.length ||
    message.stages?.length ||
    message.evidence?.length ||
    message.artifacts?.length ||
    message.taskAttachments?.length ||
    message.attachments?.length,
  );
}

export function latestTaskWorkspaceMessage(messages: ConversationMessage[]): ConversationMessage | undefined {
  return [...messages].reverse().find(hasTaskWorkspaceContent);
}
