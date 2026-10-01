#!/usr/bin/env python3
"""Discover the pinned MCP tools without making any upstream business call."""
import asyncio
import json
import os
from pathlib import Path
import tempfile

TOOLS = {"get_user_info", "list_models", "get_model", "upload_media_from_path",
         "chat_text", "generate_image", "generate_video", "get_generation_task"}


async def smoke():
    with tempfile.TemporaryDirectory(prefix="ai-hive-smoke-") as directory:
        env = {"PATH": os.environ["PATH"], "HOME": directory,
               "AI_HIVE_MCP_KEY": "protocol-only-canary",
               "npm_config_ignore_scripts": "true", "npm_config_cache": str(Path(directory) / "npm-cache")}
        with (Path(directory) / "stderr").open("wb") as stderr:
            process = await asyncio.create_subprocess_exec(
                "npx", "--yes", "@infimind-next/ai-hive-mcp@0.3.0",
                "--endpoint", "http://127.0.0.1:9/api", env=env,
                stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE, stderr=stderr)
            async def send(message):
                process.stdin.write((json.dumps({"jsonrpc": "2.0", **message}) + "\n").encode())
                await process.stdin.drain()

            async def receive(identifier):
                while True:
                    line = await asyncio.wait_for(process.stdout.readline(), timeout=90)
                    if not line:
                        raise RuntimeError("MCP server exited before its protocol response")
                    response = json.loads(line)
                    if response.get("id") == identifier:
                        if "error" in response:
                            raise RuntimeError("MCP protocol request failed")
                        return response["result"]
            try:
                await send({"id": 1, "method": "initialize", "params": {
                    "protocolVersion": "2024-11-05", "capabilities": {},
                    "clientInfo": {"name": "agent-workspace-connector-smoke", "version": "1.0.0"}}})
                initialized = await receive(1)
                assert initialized["serverInfo"] == {"name": "ai-hive", "version": "0.3.0"}
                await send({"method": "notifications/initialized"})
                await send({"id": 2, "method": "tools/list", "params": {}})
                discovered = await receive(2)
                assert {tool["name"] for tool in discovered["tools"]} == TOOLS
                for tool in discovered["tools"]:
                    if tool["name"] in {"generate_image", "generate_video"}:
                        properties = tool["inputSchema"]["properties"]
                        assert {"dryRun", "clientRequestId", "pricingSnapshot", "params"} <= properties.keys()
                print(json.dumps({"handshake": "passed", "server": initialized["serverInfo"],
                                  "tools": sorted(TOOLS), "business_calls": 0,
                                  "account_authorization": "not_tested"}))
            finally:
                process.stdin.close()
                try:
                    await asyncio.wait_for(process.wait(), timeout=5)
                except TimeoutError:
                    process.terminate()
                    await asyncio.wait_for(process.wait(), timeout=5)


if __name__ == "__main__":
    asyncio.run(smoke())
