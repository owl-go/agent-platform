#!/usr/bin/env python3
"""Build the local, reviewed Modao CLI bridge; never records Conformance or publishes."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parent
VERSION = "0.1.0"
RUNTIME_VERSION = "24.15.0"
SOURCE_SHA256 = "51f698e47cd81dd9ef79c24b4547f33a48e88cc234e01f35313bf31d1b6a1aaf"
ICON_SHA256 = "d8603337f91c3ef6b5c0587d82f8fb43b381df2e0719837776ca339a84287f77"


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def reviewed_source():
    source = (ROOT / "modao.mjs").read_bytes()
    if sha256(source) != SOURCE_SHA256:
        raise ValueError("CLI source differs from the reviewed revision")
    icon = (ROOT / "icon.svg").read_bytes()
    if sha256(icon) != ICON_SHA256:
        raise ValueError("brand asset differs from the reviewed revision")
    result = subprocess.run(["node", str(ROOT / "modao.mjs"), "--help"], check=True,
                            capture_output=True, text=True, timeout=10, env={"PATH": os.environ["PATH"]})
    help_data = json.loads(result.stdout)["data"]
    if help_data["version"] != VERSION:
        raise ValueError("CLI version differs from package version")
    return source, icon, help_data["operations"]


def bundle(source):
    output = io.BytesIO()
    with gzip.GzipFile(fileobj=output, mode="wb", mtime=0, filename="") as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            info = tarfile.TarInfo("node_modules/.bin/modao")
            info.size, info.mode, info.mtime = len(source), 0o755, 0
            archive.addfile(info, io.BytesIO(source))
    return output.getvalue()


def build(runtime_image, runtime_version):
    match = re.fullmatch(r"[a-zA-Z0-9.:/-]+@sha256:([a-f0-9]{64})", runtime_image)
    if not match or runtime_version != RUNTIME_VERSION:
        raise ValueError("require a Registry RepoDigest and the reviewed Node 24.15.0 Runtime")
    source, icon, operations = reviewed_source()
    capabilities = []
    for command, policy in operations.items():
        capabilities.append({"id": "modao_" + command.replace(" ", "_"), "argv_prefix": command.split(),
                             "risk": policy["risk"], "identities": ["user"], "scopes": [],
                             "egress_hosts": ["modao.cc"], "timeout_seconds": 180 if policy["risk"] == "high" else 60})
    for command in ["tools", "schema", "version"]:
        capabilities.append({"id": "modao_" + command, "argv_prefix": [command], "risk": "low",
                             "identities": ["user"], "scopes": [], "egress_hosts": ["modao.cc"], "timeout_seconds": 60})
    meta = {"source": "modao", "version": VERSION, "type": "cli", "name": "墨刀",
            "description": "墨刀 AI MCP 的 CLI 桥接器：生成原型、React 和 PRD，查询任务并导入 HTML；使用个人空间令牌连接。",
            "examples_zh": ["查询墨刀账号权益", "生成 HTML 原型并导入墨刀"],
            "examples_en": ["Check Modao account benefits", "Generate and import an HTML prototype"],
            "minPlatformVersion": "1.0.0", "auth_mode": "cli"}
    manifest = {"runtime": {"kind": "node", "version": runtime_version, "digest": "sha256:" + match.group(1)},
                "executable": "modao", "bundle_path": "node_modules/.bin/modao", "authentication_driver": "connector_package",
                "commands": {"init": {"argv": ["init"]}, "auth": {"argv": ["auth"], "timeout_seconds": 60},
                             "status": {"argv": ["status"], "timeout_seconds": 60}, "unAuth": {"argv": ["unauth"]}},
                "status_match": {"json_path": "$.data.authenticated", "equals": True},
                "capabilities": capabilities, "auth_url_domains": ["modao.cc"], "egress_hosts": ["modao.cc"],
                "timeout_seconds": 180,
                "resource_limits": {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 180, "concurrency": 1, "child_processes": 8}}
    files = {"connector-meta.json": json.dumps(meta, ensure_ascii=False, sort_keys=True).encode(),
             "cli.json": json.dumps(manifest, ensure_ascii=False, sort_keys=True).encode(),
             "icon.svg": icon, "cli-bundle.tgz": bundle(source), "skills/modao/SKILL.md": (ROOT / "SKILL.md").read_bytes(),
             "skills/modao/capabilities.json": json.dumps(capabilities, sort_keys=True).encode()}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for name, content in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, content)
    return output.getvalue(), sha256(files["cli-bundle.tgz"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-image", required=True, help="Configured repository@sha256:<digest>")
    parser.add_argument("--runtime-version", default=RUNTIME_VERSION)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data, bundle_sha = build(args.runtime_image, args.runtime_version)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(data)
    print(json.dumps({"output": str(args.output), "sha256": sha256(data), "bundle_sha256": bundle_sha,
                      "conformance": "not_run", "installation": "not_installed", "publication": "not_published"}))


if __name__ == "__main__":
    main()
