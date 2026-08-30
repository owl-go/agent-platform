import { afterEach, describe, expect, it, vi } from "vitest";
import { createPlatformApi } from "./client";

describe("Agent Workspace API client", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("projects Runtime availability from the authenticated API", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ items: [{ name: "codex", available: true, native_resume: false, cli_version: "0.147.0" }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const items = await createPlatformApi(() => "token").listRuntimeEngines();

    expect(items).toEqual([{ name: "codex", available: true, native_resume: false, cli_version: "0.147.0" }]);
    expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("Authorization")).toBe("Bearer token");
  });

  it("normalizes protobuf Timestamp objects without changing ordinary JSON", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ items: [{ id: "session-1", title: "Fresh", archived: false, created_at: { seconds: 1787627045, nanos: 0 }, updated_at: { seconds: "1787627045", nanos: 120000000 }, version: 1, metadata: { seconds: 7 } }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const items = await createPlatformApi(() => "token").listSessions();

    expect(items[0]?.created_at).toBe("2026-08-25T03:04:05.000Z");
    expect(items[0]?.updated_at).toBe("2026-08-25T03:04:05.120Z");
    expect((items[0] as unknown as { metadata: unknown }).metadata).toEqual({ seconds: 7 });
  });

  it("sends Settings version only as the optimistic concurrency field", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => new Response(init?.body, { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await createPlatformApi(() => "token").updateSettings({ personality: "direct_efficient", personality_instructions: "", runtime_model_defaults: [{ runtime_engine: "codex", provider_model_id: "model-1" }], default_runtime_engine: "codex", language: "zh-CN", timezone: "Asia/Shanghai", version: 3 });

    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ personality: "direct_efficient", personality_instructions: "", runtime_model_defaults: [{ runtime_engine: "codex", provider_model_id: "model-1" }], default_runtime_engine: "codex", language: "zh-CN", timezone: "Asia/Shanghai", expected_version: 3 });
  });

  it("normalizes omitted provider model collections from protobuf JSON", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{
      id: "connection-1", name: "Primary", provider_type: "openai", endpoint: "https://api.openai.com/v1",
      api_key_configured: true, verification_status: "verified", custom_endpoint: false, created_at: "2026-08-25T00:00:00Z", updated_at: "2026-08-25T00:00:00Z", version: 1,
    }] }), { status: 200, headers: { "Content-Type": "application/json" } })));

    const result = await createPlatformApi(() => "token").listModelProviderConnections();

    expect(result[0]?.protocols).toEqual([]);
    expect(result[0]?.models).toEqual([]);
  });

  it("streams a Workspace upload as binary instead of Base64 JSON", async () => {
    const fetchMock = vi.fn(async (_path: string, init?: RequestInit) => {
      expect(init?.body).toBeInstanceOf(Blob);
      expect(new Headers(init?.headers).get("Content-Type")).toBe("application/octet-stream");
      return new Response(JSON.stringify({ path: "docs/a.txt", name: "a.txt", directory: false, size: 3, modified_at: "2026-08-25T00:00:00Z" }), { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);

    const entry = await createPlatformApi(() => "token").uploadWorkspaceFile("workflow-1", "docs/a.txt", new Blob(["abc"]));

    expect(entry.path).toBe("docs/a.txt");
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/workflows/workflow-1/workspace/upload?path=docs%2Fa.txt");
  });

  it("normalizes Workspace byte counters from protobuf JSON", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{ path: "verification.txt", name: "verification.txt", directory: false, size: 22 }], usedBytes: "22", limitBytes: "1073741824" }), { status: 200, headers: { "Content-Type": "application/json" } })));

    const result = await createPlatformApi(() => "token").listWorkspace("workflow-1");

    expect(result.items[0]?.name).toBe("verification.txt");
    expect(result.used_bytes).toBe(22);
    expect(result.limit_bytes).toBe(1073741824);
  });

  it("filters legacy final-result records from file Artifacts", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{ id: "artifact-1", run_id: "run-1", kind: "result", name: "Final result", path: "", text_preview: "done", expired: false, created_at: "2026-08-25T00:00:00Z" }, { id: "artifact-2", run_id: "run-1", kind: "file", name: "report.md", path: "report.md", expired: false, created_at: "2026-08-25T00:00:00Z" }] }), { status: 200, headers: { "Content-Type": "application/json" } })));

    const result = await createPlatformApi(() => "token").listArtifacts("workflow-1");

    expect(result).toHaveLength(1);
    expect(result[0]?.name).toBe("report.md");
    expect(result[0]?.sha256).toBeUndefined();
    expect(result[0]?.size).toBe(0);
  });

  it("normalizes an omitted queued Run duration to zero", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{ id: "run-1", workflow_id: "workflow-1", workflow_name: "Build", trigger: "manual", state: "queued", queued_at: "2026-08-25T00:00:00Z" }] }), { status: 200, headers: { "Content-Type": "application/json" } })));

    const result = await createPlatformApi(() => "token").listRuns("workflow-1");

    expect(result[0]?.elapsed_ms).toBe(0);
  });

  it("submits a follow-up to the selected Run Conversation", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => new Response(JSON.stringify({ id: "run-2", conversation_id: "run-1", turn_number: 2, workflow_id: "workflow-1", workflow_name: "Build", trigger: "manual", state: "queued", text_input: "继续分析", queued_at: "2026-08-25T00:01:00Z" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await createPlatformApi(() => "token").continueRunConversation("workflow-1", "run-1", "继续分析");

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/workflows/workflow-1/runs/run-1/turns");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ content: "继续分析" });
    expect(result.turn_number).toBe(2);
  });

  it("parses replayed and live SSE events in order", async () => {
    const body = new ReadableStream({ start(controller) { controller.enqueue(new TextEncoder().encode("id: 1\nevent: run.started\ndata: {}\n\nid: 2\nevent: run.succeeded\ndata: {\"message\":\"done\"}\n\n")); controller.close(); } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } })));
    const events: string[] = [];

    await createPlatformApi(() => "token").streamRunEvents("workflow-1", "run-1", (event) => events.push(`${event.sequence}:${event.type}:${String(event.payload.message ?? "")}`));

    expect(events).toEqual(["1:run.started:", "2:run.succeeded:done"]);
  });

  it("streams Session message snapshots with progress and partial content", async () => {
    const body = new ReadableStream({ start(controller) {
      controller.enqueue(new TextEncoder().encode("id: 1\nevent: message.snapshot\ndata: {\"state\":\"generating\",\"content\":\"\",\"progress_stage\":\"thinking\",\"elapsed_ms\":0}\n\n"));
      controller.enqueue(new TextEncoder().encode("id: 2\nevent: message.snapshot\ndata: {\"state\":\"generating\",\"content\":\"你好\",\"progress_stage\":\"responding\",\"elapsed_ms\":0}\n\nid: 3\nevent: message.snapshot\ndata: {\"state\":\"completed\",\"content\":\"你好！\",\"elapsed_ms\":1200}\n\n"));
      controller.close();
    } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } })));
    const snapshots: string[] = [];

    await createPlatformApi(() => "token").streamSessionMessage("session-1", 2, (snapshot) => snapshots.push(`${snapshot.progress_stage ?? "done"}:${snapshot.content}`));

    expect(snapshots).toEqual(["thinking:", "responding:你好", "done:你好！"]);
  });

  it("requests backend cancellation for the active Session response", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ id: 2, role: "assistant", state: "generating", content: "", elapsed_ms: 0, created_at: "2026-08-25T00:00:00Z" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await createPlatformApi(() => "token").cancelSessionMessage("session-1", 2);

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/sessions/session-1/messages/2/cancellation");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });

  it("loads every Session message page in ascending order", async () => {
    const first = Array.from({ length: 200 }, (_, index) => ({ id: index + 1, role: "user", state: "completed", content: String(index + 1), elapsed_ms: 0, created_at: "2026-08-25T00:00:00Z" }));
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input);
      const items = path.includes("after=200") ? [{ ...first[0], id: 201, content: "201" }] : first;
      return new Response(JSON.stringify({ items }), { status: 200, headers: { "Content-Type": "application/json" } });
    });
    vi.stubGlobal("fetch", fetchMock);

    const messages = await createPlatformApi(() => "token").listSessionMessages("session-1");

    expect(messages).toHaveLength(201);
    expect(messages.at(-1)?.id).toBe(201);
    expect(fetchMock.mock.calls.map((call) => String(call[0]))).toEqual([
      "/api/v1/sessions/session-1/messages?after=0&limit=200",
      "/api/v1/sessions/session-1/messages?after=200&limit=200",
    ]);
  });
});
