#!/usr/bin/env python3
"""Check the public MCP with the official public demo code; print no business data."""

import json
from pathlib import Path
import urllib.request


def main():
    manifest = json.loads(Path(__file__).with_name("package").joinpath("mcp.json").read_text())
    headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
    sequence = 0

    def rpc(method, params, notification=False):
        nonlocal sequence
        body = {"jsonrpc": "2.0", "method": method, "params": params}
        if not notification:
            sequence += 1
            body["id"] = sequence
        request = urllib.request.Request(manifest["url"], json.dumps(body).encode(), headers)
        with urllib.request.urlopen(request, timeout=manifest["timeout_seconds"]) as response:
            if response.headers.get("Mcp-Session-Id"):
                headers["Mcp-Session-Id"] = response.headers["Mcp-Session-Id"]
            raw = response.read().decode()
            content_type = response.headers.get("Content-Type", "")
        if notification:
            return None
        if "text/event-stream" in content_type:
            messages = [json.loads(line[6:]) for line in raw.splitlines() if line.startswith("data: ")]
            matches = [message for message in messages if message.get("id") == body["id"]]
            if len(matches) != 1:
                raise RuntimeError("missing or duplicate MCP response")
            message = matches[0]
        else:
            message = json.loads(raw)
        if message.get("id") != body["id"] or "error" in message:
            raise RuntimeError("MCP request failed: " + method)
        return message["result"]

    initialized = rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                     "clientInfo": {"name": "agent-workspace-caoliao-smoke", "version": "1.0.0"}})
    headers["MCP-Protocol-Version"] = initialized["protocolVersion"]
    rpc("notifications/initialized", {}, notification=True)
    expected = {"entity_get", "entity_records_list", "entity_actions_list", "entity_action_get", "entity_bindings_list"}
    discovery = rpc("tools/list", {})
    if {tool["name"] for tool in discovery["tools"]} != expected or discovery.get("nextCursor"):
        raise RuntimeError("public MCP tool set changed; review the package before use")
    code = "https://qr71.cn/okDISU/qssTnoy"

    def call(name, arguments):
        result = rpc("tools/call", {"name": name, "arguments": {"code": code, **arguments}})
        if result.get("isError"):
            raise RuntimeError("MCP tool failed: " + name)
        if "structuredContent" in result:
            data = result["structuredContent"]
        else:
            data = json.loads(next(item["text"] for item in result["content"] if item["type"] == "text"))
        return data

    card = call("entity_get", {})["json"]
    rules = card["contract"]["rules"]
    if rules.get("readOnly") is not True or rules.get("publicOnly") is not True or rules.get("actionsExecutable") is not False:
        raise RuntimeError("public demo contract changed")
    actions = call("entity_actions_list", {})["actions"]
    fields = call("entity_action_get", {"action": actions[0]["action"]})["fields"]
    records = call("entity_records_list", {"size": 20})
    bindings = call("entity_bindings_list", {})["bindings"]
    if not fields or not bindings or not isinstance(records["records"], list):
        raise RuntimeError("public demo result shape changed")
    print(json.dumps({"server": initialized["serverInfo"], "protocol": initialized["protocolVersion"],
                      "tools": sorted(expected), "demo_calls": 5, "result": "passed",
                      "runtime_conformance": "not_run"}))


if __name__ == "__main__":
    main()
