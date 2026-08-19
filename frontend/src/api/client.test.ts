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

  it("fails before fetch when the authenticated token is unavailable", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    await expect(createPlatformApi(() => undefined).listRuntimeImages()).rejects.toMatchObject({ kind: "unauthenticated" });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
