import { describe, expect, it } from "vitest";
import { cliAuthorizationRequestFromActivities, cliAuthorizationRequestFromCommand, cliAuthorizationRequestFromEvents } from "./cliAuthorization";

describe("conversation CLI authorization requests", () => {
  it("recognizes an attempted user operation inside a shell wrapper", () => {
    expect(cliAuthorizationRequestFromCommand("/bin/sh -lc 'agent-cli --connector feishu-1 --capability im_messages_send --identity user -- im +messages-send'")).toEqual({ connectorID: "feishu-1", capabilityID: "im_messages_send" });
  });

  it("ignores connector selection and bot operations", () => {
    expect(cliAuthorizationRequestFromCommand("agent-cli --connector feishu-1 --capability im_messages_send --identity bot -- im +messages-send")).toBeUndefined();
    expect(cliAuthorizationRequestFromActivities([{ type: "command.requested", detail: "git status" }])).toBeUndefined();
  });

  it("uses the latest matching command from Session activities and Run events", () => {
    expect(cliAuthorizationRequestFromActivities([
      { type: "command.requested", detail: "agent-cli --connector first --capability search --identity user -- im search" },
      { type: "command.requested", detail: "agent-cli --connector second --capability send --identity me -- im send" },
    ])).toEqual({ connectorID: "second", capabilityID: "send" });
    expect(cliAuthorizationRequestFromEvents([{ sequence: 1, type: "command.requested", payload: { command: "agent-cli --connector feishu --capability calendar --identity user -- calendar list" }, raw: "{}" }])).toEqual({ connectorID: "feishu", capabilityID: "calendar" });
  });
});
