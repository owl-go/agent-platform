import { ApiError } from "./client";

export interface PublicAssistantProfile {
  name: string; introduction: string; faqs: { id: string; question: string }[];
  free_text_enabled: boolean; width: string; height: number;
}
export type PublicAssistantEvent =
  | { type: "thinking"; turn_id: string; conversation_id: string }
  | { type: "delta"; text: string }
  | { type: "done"; answer: string; state: "completed" | "cancelled" | "failed"; conversation_id?: string; failure_code?: string }
  | { type: "error"; code: string };
export interface PublicAssistantApi {
  profile(signal?: AbortSignal): Promise<PublicAssistantProfile>;
  iconURL: string;
  stream(question: string, conversationID: string, faqID: string | undefined, onEvent: (event: PublicAssistantEvent) => void, signal: AbortSignal): Promise<void>;
}

export function createPublicAssistantApi(base: string): PublicAssistantApi {
  return {
    iconURL: `${base}/icon`,
    async profile(signal) {
      const response = await fetch(base, { signal, credentials: "same-origin" });
      if (!response.ok) throw new ApiError("unavailable", response.status, "share_unavailable");
      return await response.json() as PublicAssistantProfile;
    },
    async stream(question, conversationID, faqID, onEvent, signal) {
      const response = await fetch(`${base}/turns`, {
        method: "POST", signal, credentials: "same-origin",
        headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
        body: JSON.stringify({ question, conversation_id: conversationID || undefined, faq_id: faqID }),
      });
      if (!response.ok) {
        const value = await response.json().catch(() => ({})) as { error?: string };
        throw new ApiError("unknown", response.status, value.error ?? "public_answer_failed");
      }
      if (!response.headers.get("Content-Type")?.includes("text/event-stream")) {
        const value = await response.json() as { answer?: string };
        if (!value.answer) throw new ApiError("unknown", 502, "assistant_stream_invalid");
        onEvent({ type: "done", answer: value.answer, state: "completed" });
        return;
      }
      if (!response.body) throw new ApiError("unknown", 502, "assistant_stream_invalid");
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
      let pending = "";
      let completed = false;
      const handle = (block: string) => {
        const event = /^event: (.+)$/m.exec(block)?.[1];
        const payload = /^data: (.+)$/m.exec(block)?.[1];
        if (!event || !payload) return;
        const value = JSON.parse(payload) as Record<string, string>;
        if (event === "thinking") onEvent({ type: "thinking", turn_id: value.turn_id, conversation_id: value.conversation_id });
        if (event === "delta") onEvent({ type: "delta", text: value.text });
        if (event === "error") onEvent({ type: "error", code: value.code });
        if (event === "done") {
          if (!["completed", "cancelled", "failed"].includes(value.state)) throw new ApiError("unknown", 502, "assistant_stream_invalid");
          completed = true;
          onEvent({ type: "done", answer: value.answer, state: value.state as "completed" | "cancelled" | "failed", conversation_id: value.conversation_id, failure_code: value.failure_code });
        }
      };
      try {
        while (true) {
          const { value, done } = await reader.read();
          pending += value ?? "";
          let boundary = pending.indexOf("\n\n");
          while (boundary >= 0) {
            handle(pending.slice(0, boundary));
            pending = pending.slice(boundary + 2);
            boundary = pending.indexOf("\n\n");
          }
          if (done) {
            if (pending.trim()) handle(pending);
            if (!completed) throw new ApiError("unknown", 502, "assistant_stream_interrupted");
            return;
          }
        }
      } finally {
        await reader.cancel().catch(() => undefined);
        reader.releaseLock();
      }
    },
  };
}
