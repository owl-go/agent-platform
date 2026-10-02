#!/usr/bin/env python3
"""Assemble the pinned AI-Hive MCP package and validate its final ZIP."""
import argparse
import base64
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parent
VERSION = "0.3.0"
PACKAGE = "@infimind-next/ai-hive-mcp"
INTEGRITY = "YtEmILqposMHnku0S5WgYXtuT+XnxS+tzyT5U8u8U+yb8xXiAIrg6EbG1XuKekdkrP/0CaUDYbOVL2ZAYrIRTg=="
ICON_SHA256 = "2119e4774cc43d80dcfb9f424bf203303119d343a064cde4afb84188c7705e72"


def build(upstream):
    if base64.b64encode(hashlib.sha512(upstream).digest()).decode() != INTEGRITY:
        raise ValueError("upstream archive differs from reviewed npm 0.3.0")
    with tarfile.open(fileobj=io.BytesIO(upstream), mode="r:gz") as archive:
        meta = json.load(archive.extractfile("package/package.json"))
        if meta["name"] != PACKAGE or meta["version"] != VERSION:
            raise ValueError("unexpected upstream package identity")
    icon = (ROOT / "icon.svg").read_bytes()
    if hashlib.sha256(icon).hexdigest() != ICON_SHA256:
        raise ValueError("brand asset differs from reviewed official icon")
    metadata = {
        "source": "ai-hive", "version": VERSION, "type": "mcp", "name": "AI-Hive",
        "description": "使用蜂巢 AI 查询模型、进行文本对话，预检并生成图片和视频，查询生成任务。",
        "examples_zh": ["查询可用图片模型并预检生成费用", "查询 AI-Hive 视频生成任务进度"],
        "examples_en": ["List image models and preview generation cost", "Check an AI-Hive video generation task"],
        "minPlatformVersion": "1.0.0", "auth_mode": "cli",
    }
    manifest = {
        "transport": "stdio", "runner": "npx", "package": PACKAGE,
        "package_version": VERSION, "arguments": ["--timeout-ms", "120000"],
        "environment": [{"name": "AI_HIVE_MCP_KEY", "value": "${MCP_BEARER_TOKEN}"}],
        "egress_hosts": ["registry.npmjs.org", "ai-hive.iclip.cn"], "timeout_seconds": 180,
        "resource_limits": {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 180,
                            "concurrency": 1, "child_processes": 64},
    }
    files = {"connector-meta.json": json.dumps(metadata, ensure_ascii=False, sort_keys=True).encode(),
             "mcp.json": json.dumps(manifest, sort_keys=True).encode(), "icon.svg": icon,
             "skills/ai-hive/SKILL.md": (ROOT / "SKILL.md").read_bytes()}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, content in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type, info.external_attr = zipfile.ZIP_DEFLATED, 0o100644 << 16
            archive.writestr(info, content)
    return output.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--npm-tgz", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    data = build(args.npm_tgz.read_bytes())
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(data)
    subprocess.run(["go", "-C", str(ROOT.parents[2] / "backend"), "run",
                    "./cmd/connector-package-validate", str(args.output.resolve())], check=True, timeout=120)
    print(json.dumps({"output": str(args.output.resolve()), "archive_sha256": hashlib.sha256(data).hexdigest(),
                      "installation": "not_installed", "publication": "not_published"}))


if __name__ == "__main__":
    main()
