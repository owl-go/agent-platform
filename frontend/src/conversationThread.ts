import type { Artifact, Attachment, CreditConsumption, Evidence, ExpertStage, ResourceCreationAction } from "./api/client";

export interface ConversationActivityItem {
  id: string | number;
  label: string;
  detail?: string;
}

export type ConversationActivityKind = "runtime" | "reasoning" | "tool" | "file" | "activity";

export interface ConversationActivityGroup {
  id: string | number;
  label: string;
  detail?: string;
  kind?: ConversationActivityKind;
  toolCallCount?: number;
  fileChangeCount?: number;
  state?: "running" | "completed";
  items: ConversationActivityItem[];
}

export interface ConversationMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  copyText?: string;
  state: string;
  stateLabel?: string;
  timestamp: string;
  elapsedMs?: number;
  error?: string;
  pending?: boolean;
  finalizing?: boolean;
  streaming?: boolean;
  progressTitle?: string;
  progressDetail?: string;
  currentActivity?: ConversationActivityItem;
  activities?: ConversationActivityGroup[];
  executionEvidenceCounts?: { toolCalls: number; fileChanges: number };
  stages?: ExpertStage[];
  creditConsumption?: CreditConsumption;
  artifacts?: Artifact[];
  evidence?: Evidence[];
  attachments?: Attachment[];
  resourceAction?: ResourceCreationAction;
  skills?: Array<{ id: string; name: string }>;
  meta?: { label: string; title?: string };
  retryable?: boolean;
}
