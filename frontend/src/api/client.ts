import type { components } from "./generated";

export type Health = components["schemas"]["v1HealthResponse"];
export type CurrentUser = components["schemas"]["v1CurrentUser"];

export async function getHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch("/api/healthz", {
    headers: { Accept: "application/json" },
    signal,
  });

  if (!response.ok) {
    throw new Error(`Health check failed with status ${response.status}`);
  }

  return (await response.json()) as Health;
}

export async function getCurrentUser(accessToken: string, signal?: AbortSignal): Promise<CurrentUser> {
  const response = await fetch("/api/v1/me", {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    signal,
  });

  if (!response.ok) {
    throw new Error(`Current User request failed with status ${response.status}`);
  }

  return (await response.json()) as CurrentUser;
}
