import type { InjectionKey } from "vue";
import type { components } from "./generated";

export type Health = components["schemas"]["v1HealthResponse"];
export type CurrentUser = components["schemas"]["v1CurrentUser"];
export type RuntimeImage = components["schemas"]["v1RuntimeImage"];
export type RegisterRuntimeImageInput = components["schemas"]["v1RegisterRuntimeImageRequest"];
export type RuntimeImageStatusInput = components["schemas"]["RuntimeCatalogServiceChangeRuntimeImageStatusBody"];

export type ApiErrorKind = "unauthenticated" | "forbidden" | "not_found" | "conflict" | "validation" | "rate_limited" | "unavailable" | "unknown";

export class ApiError extends Error {
  constructor(
    public readonly kind: ApiErrorKind,
    public readonly status: number,
    public readonly code: string,
    public readonly requestID: string,
  ) {
    super(code || `request_failed_${status}`);
    this.name = "ApiError";
  }
}

export interface RuntimeImagePage {
  items: RuntimeImage[];
  nextPageToken: string;
}

export interface PlatformApi {
  listRuntimeImages(pageToken?: string, pageSize?: number, signal?: AbortSignal): Promise<RuntimeImagePage>;
  getRuntimeImage(id: string, signal?: AbortSignal): Promise<RuntimeImage>;
  registerRuntimeImage(input: RegisterRuntimeImageInput, idempotencyKey: string, signal?: AbortSignal): Promise<RuntimeImage>;
  changeRuntimeImageStatus(id: string, input: RuntimeImageStatusInput, version: number, idempotencyKey: string, signal?: AbortSignal): Promise<RuntimeImage>;
}

export const platformApiKey: InjectionKey<PlatformApi> = Symbol("agent-platform-api");

export async function getHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch("/api/healthz", { headers: { Accept: "application/json" }, signal });
  if (!response.ok) throw new Error(`Health check failed with status ${response.status}`);
  return (await response.json()) as Health;
}

export async function getCurrentUser(accessToken: string, signal?: AbortSignal): Promise<CurrentUser> {
  return request<CurrentUser>(accessToken, "/api/v1/me", { signal });
}

export function createPlatformApi(getAccessToken: () => string | undefined): PlatformApi {
  const authorizedRequest = <T>(path: string, init: RequestInit = {}) => {
    const token = getAccessToken();
    if (!token) throw new ApiError("unauthenticated", 401, "invalid_authentication", "");
    return request<T>(token, path, init);
  };

  return {
    async listRuntimeImages(pageToken = "", pageSize = 20, signal) {
      const query = new URLSearchParams({ page_size: String(pageSize) });
      if (pageToken) query.set("page_token", pageToken);
      const body = await authorizedRequest<components["schemas"]["v1ListRuntimeImagesResponse"]>(`/api/v1/runtime-images?${query}`, { signal });
      return { items: body.items ?? [], nextPageToken: body.next_page_token ?? "" };
    },
    getRuntimeImage(id, signal) {
      return authorizedRequest<RuntimeImage>(`/api/v1/runtime-images/${encodeURIComponent(id)}`, { signal });
    },
    registerRuntimeImage(input, idempotencyKey, signal) {
      return authorizedRequest<RuntimeImage>("/api/v1/runtime-images", {
        method: "POST", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      });
    },
    changeRuntimeImageStatus(id, input, version, idempotencyKey, signal) {
      return authorizedRequest<RuntimeImage>(`/api/v1/runtime-images/${encodeURIComponent(id)}/status`, {
        method: "PATCH", body: JSON.stringify(input), signal,
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, "If-Match": `"${version}"` },
      });
    },
  };
}

async function request<T>(accessToken: string, path: string, init: RequestInit): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(path, { ...init, headers });
  if (!response.ok) throw await normalizeError(response);
  return (await response.json()) as T;
}

async function normalizeError(response: Response): Promise<ApiError> {
  let code = "";
  try {
    const body = await response.json() as { error?: string; reason?: string; message?: string };
    code = body.error ?? body.reason ?? body.message ?? "";
  } catch {
    // A malformed upstream response is still represented by its safe status class.
  }
  const kind: ApiErrorKind = response.status === 401 ? "unauthenticated"
    : response.status === 403 ? "forbidden"
      : response.status === 404 ? "not_found"
        : response.status === 409 || response.status === 412 ? "conflict"
          : response.status === 400 || response.status === 422 || response.status === 428 ? "validation"
            : response.status === 429 ? "rate_limited"
              : response.status >= 500 ? "unavailable" : "unknown";
  return new ApiError(kind, response.status, code, response.headers.get("X-Request-ID") ?? "");
}
