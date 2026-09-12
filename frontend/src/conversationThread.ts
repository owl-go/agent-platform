import type { Artifact, Attachment, CreditConsumption, ExpertStage } from "./api/client";

export interface ConversationActivityItem {
  id: string | number;
  label: string;
  detail?: string;
}

export interface ConversationActivityGroup {
  id: string | number;
  label: string;
  detail?: string;
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
  stages?: ExpertStage[];
  creditConsumption?: CreditConsumption;
  artifacts?: Artifact[];
  attachments?: Attachment[];
  skills?: Array<{ id: string; name: string }>;
  meta?: { label: string; title?: string };
  retryable?: boolean;
}
