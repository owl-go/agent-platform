import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, createPlatformApi, getCurrentUser } from "./client";

afterEach(() => vi.unstubAllGlobals());

describe("getCurrentUser", () => {
  it("sends the access token only in the Authorization header", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({
      user_id: "user-1",
      organization: { id: "org-1", slug: "acme", name: "Acme" },
      role_grants: [],
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const user = await getCurrentUser("private-token");

    expect(user.user_id).toBe("user-1");
    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, request] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/me");
    expect(JSON.stringify(url)).not.toContain("private-token");
    const headers = new Headers((request as RequestInit).headers);
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("Authorization")).toBe("Bearer private-token");
  });

  it("returns a safe status error without including the token", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response('{"error":"invalid_authentication"}', { status: 401 })));

    await expect(getCurrentUser("private-token")).rejects.toMatchObject({ status: 401, kind: "unauthenticated" });
    await expect(getCurrentUser("private-token")).rejects.not.toThrow("private-token");
  });
});

describe("PlatformApi", () => {
  it("sends pagination, bearer auth, and safe content negotiation", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ items: [], next_page_token: "next" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const page = await createPlatformApi(() => "access-token").listRuntimeImages("cursor", 12);

    expect(page.nextPageToken).toBe("next");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/runtime-images?page_size=12&page_token=cursor");
    const headers = new Headers(init?.headers);
    expect(headers.get("Authorization")).toBe("Bearer access-token");
    expect(headers.get("Accept")).toBe("application/json");
  });

  it("uses one supplied idempotency key and the current quoted Version", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ id: "image-1", version: 3 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.changeRuntimeImageStatus("image-1", { status: "blocked", blocked_reason: "CVE" }, 2, "intent-1");

    const headers = new Headers(fetchMock.mock.calls[0]![1]?.headers);
    expect(headers.get("Idempotency-Key")).toBe("intent-1");
    expect(headers.get("If-Match")).toBe('"2"');
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("normalizes conflicts and keeps the request correlation ID", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: "version_conflict" }), {
      status: 412, headers: { "X-Request-ID": "request-1" },
    })));

    const error = await createPlatformApi(() => "access-token").changeRuntimeImageStatus("image-1", { status: "production" }, 1, "intent-1").catch((reason) => reason);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ kind: "conflict", status: 412, code: "version_conflict", requestID: "request-1" });
  });

  it("keeps Credential Profile references out of URLs and adds write controls", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ id: "credential-1", enabled: true, version: 1 }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.registerCredentialProfile({ name: "primary", kind: "model", secret_ref: "vault://platform/model" }, "credential-intent");

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/credential-profiles");
    expect(String(url)).not.toContain("vault://platform/model");
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe("credential-intent");
    expect(init?.body).toContain("vault://platform/model");
  });

  it("scopes Repository Binding reads and protects updates with Version and intent", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ id: "binding-1", version: 3 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.listRepositoryBindings("team-1");
    await api.updateRepositoryBinding("binding-1", { team_id: "team-1", name: "repository" }, 2, "binding-intent");

    expect(fetchMock.mock.calls[0]![0]).toBe("/api/v1/repository-bindings?team_id=team-1");
    const [url, init] = fetchMock.mock.calls[1]!;
    expect(url).toBe("/api/v1/repository-bindings/binding-1");
    expect(JSON.parse(String(init?.body))).toEqual({ binding: { team_id: "team-1", name: "repository" } });
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe("binding-intent");
    expect(headers.get("If-Match")).toBe('"2"');
  });

  it("scopes Agent Draft writes and sends Version plus stable intent headers", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ id: "draft-1", version: 3 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");
    const input = { release_risk: "low", configuration: { instructions: "Ship safely", repository_binding_id: "binding-1", runtime_image_id: "runtime-1", configured_model_id: "model-1" } };

    await api.updateAgentDraft("agent-1", "draft-1", "team-1", input, 2, "draft-intent");

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/agents/agent-1/drafts/draft-1");
    expect(JSON.parse(String(init?.body))).toMatchObject({ team_id: "team-1", release_risk: "low" });
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe("draft-intent");
    expect(headers.get("If-Match")).toBe('"2"');
  });

  it("creates a Team-scoped Coding Task with the supplied idempotency intent", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ task: { id: "task-1" }, session: { id: "session-1" }, run_id: "run-1" }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.createCodingTask("team-1", { agent_release_id: "release-1", title: "Fix parser", request_text: "Handle empty input" }, "task-intent");

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/coding-tasks");
    expect(JSON.parse(String(init?.body))).toMatchObject({ team_id: "team-1", agent_release_id: "release-1" });
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe("task-intent");
    expect(headers.get("Authorization")).toBe("Bearer access-token");
  });

  it("binds Run Approval decisions and controls to one intent and quoted Version", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async (input) => new Response(JSON.stringify(
      String(input).includes("approvals") ? { id: "approval-1", state: "approved", version: 2 } : { id: "run-1", state: "interrupting", version: 4 },
    ), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.decideRunApproval("approval-1", true, "reviewed", 1, "approval-intent");
    await api.controlRun("run-1", "interrupt", 3, "control-intent");

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(["/api/v1/approvals/approval-1/decision", "/api/v1/runs/run-1/interrupt"]);
    const approvalHeaders = new Headers(fetchMock.mock.calls[0]![1]?.headers);
    expect(approvalHeaders.get("Idempotency-Key")).toBe("approval-intent");
    expect(approvalHeaders.get("If-Match")).toBe('"1"');
    const controlHeaders = new Headers(fetchMock.mock.calls[1]![1]?.headers);
    expect(controlHeaders.get("Idempotency-Key")).toBe("control-intent");
    expect(controlHeaders.get("If-Match")).toBe('"3"');
  });

  it("loads the server-authorized Team launch catalog and prerequisite", async () => {
	const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({
	  items: [{ agent_release_id: "release-1", repository_binding_id: "binding-1" }], prerequisite: "",
	}), { status: 200 }));
	vi.stubGlobal("fetch", fetchMock);

	const catalog = await createPlatformApi(() => "access-token").listCodingTaskLaunchOptions("team-1");

	expect(fetchMock.mock.calls[0]![0]).toBe("/api/v1/coding-task-launch-options?team_id=team-1");
	expect(catalog).toEqual({ items: [{ agent_release_id: "release-1", repository_binding_id: "binding-1" }], prerequisite: "" });
  });

  it("streams Run Events with bearer auth and cursor while keeping credentials out of the URL", async () => {
    const frames = [
      'id: 5\r\nevent: command.started\r\ndata: {"run_id":"run-1","sequence":5,"event_type":"command.started","payload":{"command":"test"},"created_at":"2026-08-23T08:00:00Z"}\r\n\r\n',
      'id: 6\nevent: run.completed\ndata: {"run_id":"run-1","sequence":6,"event_type":"run.completed","payload":{"result":"ok"},"created_at":"2026-08-23T08:01:00Z"}\n\n',
    ];
    const stream = new ReadableStream<Uint8Array>({
      start(controller) { for (const frame of frames) controller.enqueue(new TextEncoder().encode(frame)); controller.close(); },
    });
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } }));
    vi.stubGlobal("fetch", fetchMock);
    const received: number[] = [];

    const result = await createPlatformApi(() => "private-token").streamRunEvents("run-1", 4, (event) => received.push(event.sequence));

    expect(result).toEqual({ cursor: 6, terminal: true });
    expect(received).toEqual([5, 6]);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/v1/runs/run-1/events");
    expect(String(url)).not.toContain("private-token");
    const headers = new Headers(init?.headers);
    expect(headers.get("Authorization")).toBe("Bearer private-token");
    expect(headers.get("Last-Event-ID")).toBe("4");
    expect(headers.get("Accept")).toBe("text/event-stream");
  });

  it("deduplicates the reconnect cursor and fails closed on a Run Event gap", async () => {
    const data = [
      'event: run.running\ndata: {"run_id":"run-1","sequence":4,"event_type":"run.running","payload":{},"created_at":"2026-08-23T08:00:00Z"}\n\n',
      'event: run.completed\ndata: {"run_id":"run-1","sequence":6,"event_type":"run.completed","payload":{},"created_at":"2026-08-23T08:01:00Z"}\n\n',
    ].join("");
    vi.stubGlobal("fetch", vi.fn(async () => new Response(data, { status: 200 })));
    const received = vi.fn();

    await expect(createPlatformApi(() => "token").streamRunEvents("run-1", 4, received)).rejects.toMatchObject({ code: "event_contract_invalid" });
    expect(received).not.toHaveBeenCalled();
  });

  it("rejects a mismatched Run even when its Sequence equals the reconnect cursor", async () => {
    const data = 'event: run.running\ndata: {"run_id":"other-run","sequence":4,"event_type":"run.running","payload":{},"created_at":"2026-08-23T08:00:00Z"}\n\n';
    vi.stubGlobal("fetch", vi.fn(async () => new Response(data, { status: 200 })));

    await expect(createPlatformApi(() => "token").streamRunEvents("run-1", 4, vi.fn())).rejects.toMatchObject({ code: "event_contract_invalid" });
  });

  it("makes authorization loss and server stream errors explicit", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response('{"error":"invalid_authentication"}', { status: 401 })));
    await expect(createPlatformApi(() => "expired").streamRunEvents("run-1", 0, vi.fn())).rejects.toMatchObject({ kind: "unauthenticated", status: 401 });

    vi.stubGlobal("fetch", vi.fn(async () => new Response('event: stream_error\ndata: {"error":"event_stream_failed"}\n\n', { status: 200 })));
    await expect(createPlatformApi(() => "token").streamRunEvents("run-1", 0, vi.fn())).rejects.toMatchObject({ code: "event_stream_failed" });

    vi.stubGlobal("fetch", vi.fn(async () => new Response('event: stream_error\ndata: {"error":"invalid_authentication"}\n\n', { status: 200 })));
    await expect(createPlatformApi(() => "expired").streamRunEvents("run-1", 0, vi.fn())).rejects.toMatchObject({ kind: "unauthenticated", code: "invalid_authentication" });
  });

  it("lists safe Artifact metadata and resolves a short-lived download through the API", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [{ id: "artifact-1", run_id: "run-1", kind: "diff", size_bytes: "42", sha256: "abc" }] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ url: "https://objects.example/download?signature=short", expires_at: "2026-08-23T08:05:00Z" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "token");

    const artifacts = await api.listRunArtifacts("run-1");
    const download = await api.getArtifactDownload("artifact-1");

    expect(artifacts[0]).not.toHaveProperty("object_key");
    expect(download.url).toContain("signature=short");
    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual(["/api/v1/runs/run-1/artifacts", "/api/v1/artifacts/artifact-1/download"]);
  });

  it("keeps Release Approval distinct and protects Release status writes", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => new Response(JSON.stringify({ id: "release-1", version: 3 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPlatformApi(() => "access-token");

    await api.requestAgentDraftApproval("agent-1", "draft-1", "team-1", "Runtime-native Subagents", "approval-intent");
    await api.blockAgentRelease("agent-1", "release-1", "team-1", "Emergency policy response", 2, "block-intent");

    expect(fetchMock.mock.calls[0]![0]).toBe("/api/v1/agents/agent-1/drafts/draft-1/approval");
    expect(JSON.parse(String(fetchMock.mock.calls[0]![1]?.body))).toEqual({ team_id: "team-1", risk_reason: "Runtime-native Subagents" });
    const [url, init] = fetchMock.mock.calls[1]!;
    expect(url).toBe("/api/v1/agents/agent-1/releases/release-1/block");
    expect(JSON.parse(String(init?.body))).toEqual({ team_id: "team-1", reason: "Emergency policy response" });
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toBe("block-intent");
    expect(headers.get("If-Match")).toBe('"2"');
  });

  it("fails before fetch when the authenticated token is unavailable", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    await expect(createPlatformApi(() => undefined).listRuntimeImages()).rejects.toMatchObject({ kind: "unauthenticated" });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
