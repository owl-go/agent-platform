import type { ExecutionActivity, RunEvent } from "./api/client";

export interface CLIAuthorizationRequest {
  connectorID: string;
  capabilityID: string;
}

export function cliAuthorizationRequestFromCommand(command: string): CLIAuthorizationRequest | undefined {
  const marker = /(?:^|[\s'"])agent-cli\s/.exec(command);
  if (!marker) return undefined;
  const invocation = command.slice(marker.index + marker[0].indexOf("agent-cli"));
  const connectorID = optionValue(invocation, "connector");
  const capabilityID = optionValue(invocation, "capability");
  const identity = optionValue(invocation, "identity");
  if (!connectorID || !capabilityID || (identity !== "user" && identity !== "me")) return undefined;
  return { connectorID, capabilityID };
}

export function cliAuthorizationRequestFromActivities(activities?: ExecutionActivity[]): CLIAuthorizationRequest | undefined {
  for (const activity of [...(activities ?? [])].reverse()) {
    if (activity.type !== "command.requested") continue;
    const request = cliAuthorizationRequestFromCommand(activity.detail);
    if (request) return request;
  }
  return undefined;
}

export function cliAuthorizationRequestFromEvents(events: RunEvent[]): CLIAuthorizationRequest | undefined {
  for (const event of [...events].reverse()) {
    if (event.type !== "command.requested") continue;
    const detail = typeof event.payload.command === "string" ? event.payload.command : typeof event.payload.tool === "string" ? event.payload.tool : "";
    const request = cliAuthorizationRequestFromCommand(detail);
    if (request) return request;
  }
  return undefined;
}

function optionValue(command: string, name: string): string {
  const match = new RegExp(`(?:^|\\s)--${name}\\s+["']?([A-Za-z0-9][A-Za-z0-9._-]*)`).exec(command);
  return match?.[1] ?? "";
}
