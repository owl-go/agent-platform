#!/usr/bin/env python3
"""Build an MCP package from an owner-exported official Haokuaiji configuration."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import urlsplit
import zipfile

ROOT = Path(__file__).resolve().parent
VERSION = "0.1.0"
HOST = "mcphub.chanapp.chanjet.com"
ICON_SHA256 = "abfb82dd7671d7374d6f99dbf0c8d7e05640bf3ef24fe2ca9979973ef62b4a2c"
PRODUCT = re.compile(r"好会计|haokuaiji|(?:^|[^a-z])hkj(?:[^a-z]|$)", re.I)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("configuration contains duplicate JSON keys")
        result[key] = value
    return result


def read_config(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 64 * 1024:
        raise ValueError("configuration must be a regular JSON file no larger than 64 KiB")
    try:
        return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=unique_object)
    except (UnicodeError, json.JSONDecodeError):
        # JSON decoding errors can contain credential-bearing input.
        raise ValueError("configuration must contain valid UTF-8 JSON") from None


def manifest_from_config(config, server_name=None):
    if not isinstance(config, dict) or set(config) != {"mcpServers"}:
        raise ValueError("expected the official mcp-json export containing mcpServers")
    servers = config["mcpServers"]
    if not isinstance(servers, dict) or not servers:
        raise ValueError("mcpServers must contain a named server")
    if server_name is None:
        names = [name for name in servers if PRODUCT.search(name)]
        if len(names) != 1:
            raise ValueError("export must identify exactly one Haokuaiji server; use --server to select it")
        server_name = names[0]
    if server_name not in servers or not PRODUCT.search(server_name):
        raise ValueError("selected server must identify Haokuaiji, not another Chanjet product")
    server = servers[server_name]
    if not isinstance(server, dict) or set(server) - {"type", "transport", "url", "headers"}:
        raise ValueError("server contains unsupported configuration; do not discard routing or auth fields")
    for key in ("type", "transport"):
        if key in server and server[key] not in ("http", "streamable-http", "streamable_http"):
            raise ValueError("only HTTPS Streamable HTTP exports are supported")
    address = server.get("url")
    if not isinstance(address, str) or any(ord(c) < 33 or ord(c) == 127 for c in address):
        raise ValueError("server URL is missing or unsafe")
    parsed = urlsplit(address)
    # Never infer a Haokuaiji service ID from the documented HSY/HYC examples.
    # Its actual service ID must come from the owner's export.
    if (parsed.scheme != "https" or parsed.netloc != HOST
            or not re.fullmatch(r"/[1-9][0-9]*/mcp", parsed.path)
            or parsed.query or parsed.fragment):
        raise ValueError("export must use the official MCP Hub HTTPS endpoint without URL credentials")
    headers = server.get("headers")
    if not isinstance(headers, dict) or not headers:
        raise ValueError("an owner-authorized export with a Bearer header is required")
    normalized = {}
    for key, value in headers.items():
        if not isinstance(key, str) or not isinstance(value, str) or any(ord(c) < 32 or ord(c) == 127 for c in value):
            raise ValueError("export contains unsafe HTTP headers")
        lower = key.lower()
        if lower in normalized:
            raise ValueError("export contains duplicate HTTP headers")
        normalized[lower] = value
    if set(normalized) - {"authorization", "accept", "content-type"}:
        raise ValueError("platform currently materializes only Bearer auth; additional headers need an adapter")
    for key, expected in {"accept": "application/json, text/event-stream", "content-type": "application/json"}.items():
        if key in normalized and normalized[key] != expected:
            raise ValueError("export contains unsupported protocol headers")
    authorization = normalized.get("authorization", "")
    if not re.fullmatch(r"Bearer [A-Za-z0-9._~+/=-]+", authorization, re.I):
        raise ValueError("platform currently requires a Bearer API Key; other authentication needs an adapter")
    return {"transport": "streamable_http", "url": address,
            "egress_hosts": [HOST], "timeout_seconds": 60}


def build(config, server_name=None):
    manifest = manifest_from_config(config, server_name)
    icon = (ROOT / "icon.svg").read_bytes()
    if hashlib.sha256(icon).hexdigest() != ICON_SHA256:
        raise ValueError("brand asset differs from the reviewed source")
    metadata = {"source": "haokuaiji", "version": VERSION, "type": "mcp",
                "name": "用友好会计", "description": "通过畅捷通官方 MCP 连接好会计；可用财税操作以企业授权后的工具目录为准。",
                "examples_zh": ["查看好会计连接器的可用工具", "使用已授权只读工具查询好会计账套数据"],
                "examples_en": ["List available Haokuaiji tools", "Query Haokuaiji data with an authorized read-only tool"],
                "minPlatformVersion": "1.0.0", "auth_mode": "cli"}
    files = {"connector-meta.json": json.dumps(metadata, ensure_ascii=False, sort_keys=True).encode(),
             "mcp.json": json.dumps(manifest, sort_keys=True).encode(), "icon.svg": icon,
             "skills/haokuaiji/SKILL.md": (ROOT / "SKILL.md").read_bytes()}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, body in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, body)
    return output.getvalue()


def validate_package(content):
    with tempfile.TemporaryDirectory(prefix="haokuaiji-validate-") as directory:
        path = Path(directory) / "package.zip"
        path.write_bytes(content)
        result = subprocess.run(["go", "-C", str(ROOT.parents[2] / "backend"), "run",
                                 "./cmd/connector-package-validate", str(path)],
                                capture_output=True, text=True, timeout=120)
        if result.returncode:
            raise ValueError("the current connectorpackage.Parse rejected the package or could not run")
        return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True, type=Path, help="Local official mcp-json export; never committed")
    parser.add_argument("--server", help="Exact Haokuaiji server name in a multi-server export")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    try:
        if args.output.resolve() == args.config.resolve():
            raise ValueError("output must not overwrite the credential-bearing configuration")
        package = build(read_config(args.config), args.server)
        validation = validate_package(package)
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_bytes(package)
        print(json.dumps({"output": str(args.output), "sha256": hashlib.sha256(package).hexdigest(),
                          "validation": validation, "handshake": "not_run", "business_operations": "not_verified",
                          "installation": "not_installed", "publication": "not_published"}, ensure_ascii=False))
    except (ValueError, OSError, subprocess.SubprocessError):
        # Paths, upstream input, and subprocess diagnostics may contain secrets.
        parser.exit(1, "Build failed: check the official Haokuaiji export and README compatibility requirements.\n")


if __name__ == "__main__":
    main()
