import { afterEach, describe, expect, it, vi } from "vitest";
import { getCurrentUser } from "./client";

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
    expect((request as RequestInit).headers).toMatchObject({
      Accept: "application/json",
      Authorization: "Bearer private-token",
    });
  });

  it("returns a safe status error without including the token", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response('{"error":"invalid_authentication"}', { status: 401 })));

    await expect(getCurrentUser("private-token")).rejects.toThrow("401");
    await expect(getCurrentUser("private-token")).rejects.not.toThrow("private-token");
  });
});
