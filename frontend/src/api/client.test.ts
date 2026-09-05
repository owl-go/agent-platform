import { afterEach, describe, expect, it, vi } from "vitest";
import { createPlatformApi, type SessionMessageSnapshot } from "./client";

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

  it("allowlists fields when updating a Model Provider Connection", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => new Response(init?.body, { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const editForm = {
      name: "OpenAI",
      provider_type: "openai",
      endpoint: "https://models.example.test/openai/v1",
      protocols: ["openai_responses", "openai_chat"],
      api_key: "",
    };

    await createPlatformApi(() => "token").updateModelProviderConnection("connection-1", editForm, 12);

    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      name: "OpenAI",
      endpoint: "https://models.example.test/openai/v1",
      protocols: ["openai_responses", "openai_chat"],
      expected_version: 12,
    });
  });

  it("configures Git separately from the read-only Workspace", async () => {
    const fetchMock = vi.fn(async (_path: string, init?: RequestInit) => {
      expect(JSON.parse(String(init?.body))).toEqual({ url: "https://git.example.com/team/project.git", branch: "main", authentication: "basic", username: "developer", password: "secret", config: [{ key: "user.name", value: "Agent" }] });
      return new Response(JSON.stringify({ id: "workflow-1" }), { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);

    await createPlatformApi(() => "token").configureWorkflowGitSource("workflow-1", { url: "https://git.example.com/team/project.git", branch: "main", authentication: "basic", username: "developer", password: "secret", config: [{ key: "user.name", value: "Agent" }] });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/workflows/workflow-1/git-source");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PUT");
  });

  it("uploads a message attachment with its original media type", async () => {
    const fetchMock = vi.fn(async (_path: string, init?: RequestInit) => {
      expect(init?.body).toBeInstanceOf(File);
      expect(new Headers(init?.headers).get("Content-Type")).toBe("image/png");
      return new Response(JSON.stringify({ id: "attachment-1", name: "diagram.png", content_type: "image/png", size: 3, sha256: "abc", image: true }), { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);

    const attachment = await createPlatformApi(() => "token").uploadAttachment(new File(["png"], "diagram.png", { type: "image/png" }));

    expect(attachment.image).toBe(true);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/attachments/upload?name=diagram.png");
  });

  it("downloads attachment content through the authenticated API", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(new Uint8Array([1, 2, 3]), { status: 200, headers: { "Content-Type": "image/png" } }));
    vi.stubGlobal("fetch", fetchMock);

    const content = await createPlatformApi(() => "token").getAttachmentDownload("attachment-1");

    expect(content).toBeInstanceOf(Blob);
    expect(content.type).toBe("image/png");
    expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("Authorization")).toBe("Bearer token");
  });

  it("loads Session Artifact metadata and downloads authenticated content", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => String(input).endsWith("/download")
      ? new Response("generated report", { status: 200, headers: { "Content-Type": "application/octet-stream" } })
      : new Response(JSON.stringify({ items: [{ id: 2, role: "assistant", state: "completed", content: "done", elapsed_ms: 1, created_at: "2026-08-25T00:00:00Z", artifacts: [{ id: "artifact-1", message_id: "2", kind: "file", name: "report.md", path: "report.md", size: "1536", expired: false, created_at: "2026-08-25T00:00:00Z" }] }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "token");

    const messages = await api.listSessionMessages("session-1");
    const download = await api.getSessionArtifactDownload("session-1", "artifact-1");

    expect(messages[0]?.artifacts?.[0]?.size).toBe(1536);
    expect(download).toBeInstanceOf(Blob);
    expect(download.size).toBeGreaterThan(0);
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/sessions/session-1/artifacts/artifact-1/download");
    expect(new Headers(fetchMock.mock.calls[1]?.[1]?.headers).get("Authorization")).toBe("Bearer token");
  });

  it("downloads Workflow Artifact content through the authenticated API", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response("workflow report", { status: 200, headers: { "Content-Type": "application/octet-stream" } }));
    vi.stubGlobal("fetch", fetchMock);

    const download = await createPlatformApi(() => "token").getArtifactDownload("workflow-1", "artifact-1");

    expect(download).toBeInstanceOf(Blob);
    expect(download.size).toBeGreaterThan(0);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/workflows/workflow-1/artifacts/artifact-1/download");
    expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("Authorization")).toBe("Bearer token");
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

  it("normalizes a non-numeric Run duration to zero", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{ id: "run-1", workflow_id: "workflow-1", workflow_name: "Build", trigger: "manual", state: "running", queued_at: "2026-08-25T00:00:00Z", elapsed_ms: "invalid" }] }), { status: 200, headers: { "Content-Type": "application/json" } })));

    const result = await createPlatformApi(() => "token").listRuns("workflow-1");

    expect(result[0]?.elapsed_ms).toBe(0);
  });

  it("submits a follow-up to the selected Run Conversation", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => new Response(JSON.stringify({ id: "run-2", conversation_id: "run-1", turn_number: 2, workflow_id: "workflow-1", workflow_name: "Build", trigger: "manual", state: "queued", text_input: "继续分析", queued_at: "2026-08-25T00:01:00Z" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await createPlatformApi(() => "token").continueRunConversation("workflow-1", "run-1", "继续分析");

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/workflows/workflow-1/runs/run-1/turns");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ content: "继续分析", attachment_ids: [] });
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

  it("parses the final Session snapshot when the stream closes without a blank-line delimiter", async () => {
    const body = new ReadableStream({ start(controller) {
      controller.enqueue(new TextEncoder().encode("id: 1\nevent: message.snapshot\ndata: {\"state\":\"completed\",\"content\":\"完成\",\"elapsed_ms\":1200}"));
      controller.close();
    } });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } })));
    const snapshots: SessionMessageSnapshot[] = [];

    await createPlatformApi(() => "token").streamSessionMessage("session-1", 2, (snapshot) => snapshots.push(snapshot));

    expect(snapshots).toEqual([{ state: "completed", content: "完成", elapsed_ms: 1200 }]);
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

  it("uses safe administrator Redemption Code status endpoints", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => new Response(JSON.stringify(String(input).endsWith("/void") ? { id: "code-1", state: "void" } : { items: [{ id: "code-1", identifier: "safe-id", state: "available" }], next_cursor: "code-1" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "token");

    const page = await api.listRedemptionCodes("cursor-1");
    await api.voidRedemptionCode("code-1");

    expect(page.items[0]?.identifier).toBe("safe-id");
    expect(fetchMock.mock.calls.map((call) => String(call[0]))).toEqual([
      "/api/v1/admin/redemption-codes?limit=50&cursor=cursor-1",
      "/api/v1/admin/redemption-codes/code-1/void",
    ]);
  });
});
