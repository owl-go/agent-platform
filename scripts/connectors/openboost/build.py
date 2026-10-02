#!/usr/bin/env python3
"""Build the local, reviewed OpenBoost CLI bridge; never records Conformance or publishes."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent
VERSION = "0.1.0"
RUNTIME_VERSION = "24.15.0"
SOURCE_SHA256 = "610ae66b2941737452d4941477c5570b09e03a6adbda71d685e0a9650c54cb4e"
ICON_SHA256 = "23968e48e1f54d9f3e9e8814bac3bd84f3204e98371319ddb13b0475f35958d1"


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def reviewed_source():
    source = (ROOT / "openboost.mjs").read_bytes()
    if sha256(source) != SOURCE_SHA256:
        raise ValueError("CLI source differs from the reviewed revision")
    icon = (ROOT / "icon.svg").read_bytes()
    if sha256(icon) != ICON_SHA256:
        raise ValueError("brand asset differs from the reviewed revision")
    result = subprocess.run(["node", str(ROOT / "openboost.mjs"), "--help"], check=True,
                            capture_output=True, text=True, timeout=10, env={"PATH": os.environ["PATH"]})
    help_data = json.loads(result.stdout)["data"]
    if help_data["version"] != VERSION:
        raise ValueError("CLI version differs from package version")
    return source, icon, help_data["operations"]


def source_zip(source, capabilities):
    metadata = {"name": "@agent-platform/openboost-connector", "version": VERSION, "type": "module", "bin": {"openboost": "openboost.mjs"},
                "agentWorkspace": {"executable": "openboost", "authenticationDriver": "connector_package",
                                   "supportedArchitectures": ["linux-amd64", "linux-arm64"],
                                   "resourceLimits": {"cpuMillis": 1000, "memoryMiB": 512, "childProcesses": 64},
                                   "capabilities": [{"id": c["id"], "argvPrefix": c["argv_prefix"], "risk": c["risk"],
                                                     "identities": c["identities"], "scopes": c["scopes"],
                                                     "egressHosts": c["egress_hosts"], "timeoutSeconds": c["timeout_seconds"]} for c in capabilities]}}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, (body, mode) in {"openboost.mjs": (source, 0o755), "package.json": (json.dumps(metadata, sort_keys=True).encode(), 0o644)}.items():
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type, info.external_attr = zipfile.ZIP_DEFLATED, (0o100000 | mode) << 16
            archive.writestr(info, body)
    return output.getvalue()


def bundle(source):
    with tempfile.TemporaryDirectory(prefix="openboost-build-") as directory:
        root = Path(directory)
        (root / "source.zip").write_bytes(source)
        subprocess.run(["go", "-C", str(ROOT.parents[2] / "backend"), "run", "./cmd/cli-connector-bundle",
                        str(root / "source.zip"), str(root / "bundle.tgz")], check=True, timeout=90)
        return (root / "bundle.tgz").read_bytes()


def build(runtime_image, runtime_version):
    match = re.fullmatch(r"[a-zA-Z0-9.:/-]+@sha256:([a-f0-9]{64})", runtime_image)
    if not match or runtime_version != RUNTIME_VERSION:
        raise ValueError("require a Registry RepoDigest and the reviewed Node 24.15.0 Runtime")
    source, icon, operations = reviewed_source()
    capabilities = []
    for command, policy in operations.items():
        capabilities.append({"id": "openboost_" + command.replace(" ", "_"), "argv_prefix": command.split(),
                             "risk": policy["risk"], "identities": ["user"], "scopes": [],
                             "egress_hosts": ["mcp.microdata-inc.com"], "timeout_seconds": 180})
    for command in ["tools", "schema", "version"]:
        capabilities.append({"id": "openboost_" + command, "argv_prefix": [command], "risk": "low",
                             "identities": ["user"], "scopes": [], "egress_hosts": ["mcp.microdata-inc.com"], "timeout_seconds": 60})
    meta = {"source": "openboost", "version": VERSION, "type": "cli", "name": "OpenBoost",
            "description": "连接 OpenBoost 官方统一 MCP，查询 Amazon、TikTok、专利及社媒跨境数据；使用 Secret Key 连接。",
            "examples_zh": ["分析美国 Amazon 家居类目竞争与价格分布", "查询 TikTok 商品、达人和评论数据", "检索产品相关专利"],
            "examples_en": ["Analyze Amazon category competition and prices", "Research TikTok products and creators", "Search relevant patents"],
            "minPlatformVersion": "1.0.0", "auth_mode": "cli"}
    manifest = {"runtime": {"kind": "node", "version": runtime_version, "digest": "sha256:" + match.group(1)},
                "executable": "openboost", "bundle_path": "node_modules/.bin/openboost", "authentication_driver": "connector_package",
                "commands": {"init": {"argv": ["init"]}, "auth": {"argv": ["auth"], "timeout_seconds": 60},
                             "status": {"argv": ["status"], "timeout_seconds": 60}, "unAuth": {"argv": ["unauth"]}},
                "status_match": {"json_path": "$.data.authenticated", "equals": True},
                "capabilities": capabilities, "auth_url_domains": ["mcp.microdata-inc.com"], "egress_hosts": ["mcp.microdata-inc.com"],
                "timeout_seconds": 180,
                "resource_limits": {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 180, "concurrency": 1, "child_processes": 64}}
    source_archive = source_zip(source, capabilities)
    files = {"connector-meta.json": json.dumps(meta, ensure_ascii=False, sort_keys=True).encode(),
             "cli.json": json.dumps(manifest, ensure_ascii=False, sort_keys=True).encode(),
             "icon.svg": icon, "cli-bundle.tgz": bundle(source_archive), "skills/openboost/SKILL.md": (ROOT / "SKILL.md").read_bytes(),
             "skills/openboost/capabilities.json": json.dumps(capabilities, sort_keys=True).encode(), "skills/openboost/tools.json": (ROOT / "tools.json").read_bytes()}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for name, content in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, content)
    return output.getvalue(), sha256(files["cli-bundle.tgz"]), source_archive


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-image", required=True, help="Configured repository@sha256:<digest>")
    parser.add_argument("--runtime-version", default=RUNTIME_VERSION)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data, bundle_sha, source = build(args.runtime_image, args.runtime_version)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(data)
    args.output.with_suffix(".source.zip").write_bytes(source)
    print(json.dumps({"output": str(args.output), "sha256": sha256(data), "bundle_sha256": bundle_sha,
                      "conformance": "not_run", "installation": "not_installed", "publication": "not_published"}))


if __name__ == "__main__":
    main()
