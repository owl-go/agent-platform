import type { ExecutionActivity } from "./api/client";

export type ExecutionActivitySummaryKind =
  | "runtime"
  | "reasoning"
  | "feishuChatSearchHelp"
  | "feishuChatSearch"
  | "feishuMessageSendHelp"
  | "feishuMessageSend"
  | "tool"
  | "file"
  | "activity";

export interface ExecutionActivitySummary {
  id: number;
  kind: ExecutionActivitySummaryKind;
  detail?: string;
  state: "running" | "completed";
  activities: ExecutionActivity[];
}

function capabilityFromCommand(command: string) {
  const match = command.match(/(?:^|\s)--capability(?:=|\s+)(?:"([^"]+)"|'([^']+)'|([^\s"']+))/);
  return match?.[1] ?? match?.[2] ?? match?.[3];
}

function commandSummaryKind(command: string): ExecutionActivitySummaryKind {
  const capability = capabilityFromCommand(command);
  const help = /(?:^|\s)--help(?:\s|["']|$)/.test(command);
  if (capability === "im_chat_search") return help ? "feishuChatSearchHelp" : "feishuChatSearch";
  if (capability === "im_messages_send") return help ? "feishuMessageSendHelp" : "feishuMessageSend";
  return "tool";
}

export function summarizeExecutionActivities(activities: ExecutionActivity[]): ExecutionActivitySummary[] {
  const summaries: ExecutionActivitySummary[] = [];
  let current: ExecutionActivitySummary | undefined;

  const finishCurrent = () => {
    if (current) summaries.push(current);
    current = undefined;
  };
  const start = (kind: ExecutionActivitySummaryKind, activity: ExecutionActivity, state: ExecutionActivitySummary["state"], detail?: string) => {
    current = { id: summaries.length, kind, detail, state, activities: [activity] };
  };

  for (const activity of activities) {
    if (activity.type === "runtime.started") {
      finishCurrent();
      start("runtime", activity, "completed");
      finishCurrent();
      continue;
    }
    if (activity.type === "reasoning.summary") {
      finishCurrent();
      start("reasoning", activity, "completed", activity.detail);
      continue;
    }
    if (activity.type === "command.requested") {
      if (current?.kind === "reasoning") {
        current.activities.push(activity);
        current.state = "running";
      } else {
        finishCurrent();
        start(commandSummaryKind(activity.detail), activity, "running", activity.detail);
      }
      continue;
    }
    if (activity.type === "command.completed") {
      if (current?.kind === "reasoning") {
        current.activities.push(activity);
        current.state = "completed";
      } else if (current?.state === "running" && current.detail === activity.detail) {
        current.activities.push(activity);
        current.state = "completed";
        finishCurrent();
      } else {
        finishCurrent();
        start(commandSummaryKind(activity.detail), activity, "completed", activity.detail);
        finishCurrent();
      }
      continue;
    }
    if (current?.kind === "reasoning") {
      current.activities.push(activity);
      continue;
    }
    finishCurrent();
    start(activity.type === "file.changed" ? "file" : "activity", activity, "completed");
    finishCurrent();
  }

  finishCurrent();
  return summaries;
}
