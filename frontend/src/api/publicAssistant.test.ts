import { afterEach, describe, expect, it, vi } from "vitest";
import { createPublicAssistantApi, type PublicAssistantEvent } from "./publicAssistant";

afterEach(() => vi.unstubAllGlobals());
describe("Public Assistant API", () => {
  it("delivers a delta before the stream closes without authenticated credentials", async () => {
    let producer!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({ start(controller) { producer = controller; } });
    const fetchMock = vi.fn(async () => new Response(body, { headers: { "Content-Type": "text/event-stream" } }));
    vi.stubGlobal("fetch", fetchMock);
    const events: PublicAssistantEvent[] = [];
    let firstDelta!: () => void;
    const deltaArrived = new Promise<void>((resolve) => { firstDelta = resolve; });
    const streaming = createPublicAssistantApi("/api/v1/public/assistants/test-token").stream("问题", "", undefined, (event) => { events.push(event); if (event.type === "delta") firstDelta(); }, new AbortController().signal);
    const encoder = new TextEncoder();
    producer.enqueue(encoder.encode('event: delta\ndata: {"text":"第一段"}\n\n'));
    await deltaArrived;
    expect(events).toEqual([{ type: "delta", text: "第一段" }]);
    producer.enqueue(encoder.encode('event: done\ndata: {"answer":"第一段完成","state":"completed","conversation_id":"visitor"}\n\n'));
    producer.close();
    await streaming;
    expect(events[1]).toMatchObject({ type: "done", answer: "第一段完成" });
    const init = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(new Headers(init[1].headers).has("Authorization")).toBe(false);
  });

  it("fails an interrupted stream instead of marking partial text complete", async () => {
    const body = new ReadableStream({ start(controller) { controller.enqueue(new TextEncoder().encode('event: delta\ndata: {"text":"部分"}\n\n')); controller.close(); } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { headers: { "Content-Type": "text/event-stream" } })));
    await expect(createPublicAssistantApi("/public").stream("问题", "", undefined, () => {}, new AbortController().signal)).rejects.toMatchObject({ code: "assistant_stream_interrupted" });
  });
});
