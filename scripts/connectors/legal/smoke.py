#!/usr/bin/env python3
"""Verify real MCP discovery and upload preparation without sending a contract."""
import json
from pathlib import Path
from urllib import error, parse, request

ROOT = Path(__file__).resolve().parent


def rpc(url, method, params, identifier=1):
    message = {"jsonrpc": "2.0", "id": identifier, "method": method, "params": params}
    headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
    with request.urlopen(request.Request(url, json.dumps(message).encode(), headers), timeout=60) as response:
        raw = response.read().decode()
        if "text/event-stream" in response.headers.get("Content-Type", ""):
            replies = [json.loads(line[6:]) for line in raw.splitlines() if line.startswith("data: ")]
            reply = next(item for item in replies if item.get("id") == identifier)
        else:
            reply = json.loads(raw)
    if reply.get("id") != identifier or "error" in reply:
        raise RuntimeError("MCP protocol response failed")
    return reply["result"]


def main():
    pkulaw = json.loads((ROOT / "pkulaw/mcp.json").read_text())
    try:
        rpc(pkulaw["url"], "tools/list", {})
    except error.HTTPError as exc:
        if exc.code != 401:
            raise RuntimeError("PKULaw unauthenticated boundary changed") from None
    else:
        raise RuntimeError("PKULaw unexpectedly accepted an unauthenticated request")
    mindbye = json.loads((ROOT / "mindbye/mcp.json").read_text())
    initialized = rpc(mindbye["url"], "initialize", {"protocolVersion": "2025-03-26", "capabilities": {},
                         "clientInfo": {"name": "agent-workspace-legal-smoke", "version": "1.0.0"}})
    tools = rpc(mindbye["url"], "tools/list", {})
    expected = {"contract_review_prepare_upload", "contract_review_submit", "contract_review_result"}
    if {tool["name"] for tool in tools["tools"]} != expected or tools.get("nextCursor"):
        raise RuntimeError("Mindbye tool set changed; review the package")
    prepared = rpc(mindbye["url"], "tools/call", {"name": "contract_review_prepare_upload", "arguments": {
        "fileName": "agent-workspace-synthetic-smoke.txt", "fileType": "txt", "mimeType": "text/plain", "sizeBytes": 100}})
    if prepared.get("isError"):
        raise RuntimeError("Mindbye upload preparation failed")
    data = json.loads(next(item["text"] for item in prepared["content"] if item["type"] == "text"))
    upload = data["data"]
    url = parse.urlparse(upload["uploadUrl"])
    if data.get("success") is not True or url.scheme != "https" or url.hostname not in mindbye["egress_hosts"]:
        raise RuntimeError("Mindbye upload target is outside reviewed Egress")
    print(json.dumps({"pkulaw": {"unauthenticated_http": 401, "authorized_queries": "not_run"},
                      "mindbye": {"server": initialized["serverInfo"], "tools": sorted(expected),
                                  "upload_preparation": "passed", "upload_host": url.hostname,
                                  "contract_submissions": 0, "completed_reviews": "not_run"},
                      "runtime_conformance": "not_run"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
