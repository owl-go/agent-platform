import type { components } from "./generated";

export type Health = components["schemas"]["v1HealthResponse"];

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
