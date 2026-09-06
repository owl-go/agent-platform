import type { Attachment, ConversationFile, ConversationSelection, ConversationScope, ConversationInput } from "./api/client";
export interface ComposerSubmission { content: string; attachmentIDs: string[]; input: ConversationInput }

export type DraftPart = { kind: "text"; text: string } | { kind: "skill"; id: string; name: string } | { kind: "file"; file: ConversationFile };
export interface ConversationDraft { parts: DraftPart[]; selection?: ConversationSelection; attachments: Attachment[]; pendingFileNames: string[] }
export function conversationDraftKey(owner: string, scope: ConversationScope): string {
  return `agent-workspace:conversation-draft:v1:${owner}:${scope.session_id ? `session:${scope.session_id}` : `run:${scope.workflow_id}:${scope.run_id}`}`;
}
export function loadConversationDraft(key: string): ConversationDraft | undefined {
  try { const value = JSON.parse(localStorage.getItem(key) ?? "null") as ConversationDraft | null; return value && Array.isArray(value.parts) && Array.isArray(value.attachments) && Array.isArray(value.pendingFileNames) ? value : undefined; } catch { return undefined; }
}
export function saveConversationDraft(key: string, draft: ConversationDraft): void {
  try { localStorage.setItem(key, JSON.stringify(draft)); } catch { /* Storage can be disabled; keep the live draft editable. */ }
}
export function draftText(parts: DraftPart[]): string {
  return parts.map((part) => part.kind === "text" ? part.text : part.kind === "skill" ? `[${part.name}]` : `@${part.file.name}`).join("").trim();
}
