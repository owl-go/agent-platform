#!/usr/bin/env python3
"""Build the local, reviewed Picset AI CLI bridge; never records Conformance or publishes."""
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
POLICY_SHA256 = "f2ff04a865e74ff8e9a682cbffcde897541b909591606913a727023fa1ffba84"
SOURCE_SHA256 = "8f9287dadf4775de6af06cb7a119937fb6a75e298e40a246b46f7b76b47caddb"
ICON_SHA256 = "046ea6d52c5ddcc82659a2b10eff73e6173fff3bf4596508e1038dd03e688716"


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def reviewed_source():
    source = (ROOT / "picset-ai.mjs").read_bytes()
    if sha256(source) != SOURCE_SHA256:
        raise ValueError("CLI source differs from the reviewed revision")
    policy = (ROOT / "operations.json").read_bytes()
    if sha256(policy) != POLICY_SHA256:
        raise ValueError("API policy differs from reviewed revision")
    icon = (ROOT / "icon.svg").read_bytes()
    if sha256(icon) != ICON_SHA256:
        raise ValueError("brand asset differs from the reviewed revision")
    result = subprocess.run(["node", str(ROOT / "picset-ai.mjs"), "--help"], check=True,
                            capture_output=True, text=True, timeout=10, env={"PATH": os.environ["PATH"]})
    help_data = json.loads(result.stdout)["data"]
    if help_data["version"] != VERSION:
        raise ValueError("CLI version differs from package version")
    return source, icon, help_data["operations"]


def source_zip(source, capabilities):
    metadata = {"name": "@agent-platform/picset-ai-connector", "version": VERSION, "type": "module", "bin": {"picset-ai": "picset-ai.mjs"},
                "agentWorkspace": {"executable": "picset-ai", "authenticationDriver": "connector_package",
                                   "supportedArchitectures": ["linux-amd64", "linux-arm64"],
                                   "resourceLimits": {"cpuMillis": 1000, "memoryMiB": 512, "childProcesses": 64},
                                   "capabilities": [{"id": c["id"], "argvPrefix": c["argv_prefix"], "risk": c["risk"],
                                                     "identities": c["identities"], "scopes": c["scopes"],
                                                     "egressHosts": c["egress_hosts"], "timeoutSeconds": c["timeout_seconds"]} for c in capabilities]}}
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, (body, mode) in {"picset-ai.mjs": (source, 0o755), "operations.json": ((ROOT / "operations.json").read_bytes(), 0o644), "package.json": (json.dumps(metadata, sort_keys=True).encode(), 0o644)}.items():
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type, info.external_attr = zipfile.ZIP_DEFLATED, (0o100000 | mode) << 16
            archive.writestr(info, body)
    return output.getvalue()


def bundle(source):
    with tempfile.TemporaryDirectory(prefix="picset-ai-build-") as directory:
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
        capabilities.append({"id": "picset_" + command.replace("-", "_"), "argv_prefix": command.split(),
                             "risk": policy["risk"], "identities": ["user"], "scopes": [],
                             "egress_hosts": ["picsetai.cn"], "timeout_seconds": 180 if policy["risk"] == "high" else 60})
    for command in ["operations", "schema", "version"]:
        capabilities.append({"id": "picset_" + command, "argv_prefix": [command], "risk": "low",
                             "identities": ["user"], "scopes": [], "egress_hosts": ["picsetai.cn"], "timeout_seconds": 60})
    meta = {"source": "picset-ai", "version": VERSION, "type": "cli", "name": "Picset AI",
            "description": "Picset AI 图片 API：商品图、风格复刻、SKU 替换、精修、翻译、生成、分层与任务查询；使用个人 API Key 连接。",
            "examples_zh": ["用文字生成商品图片", "查询 Picset AI 图片任务进度"],
            "examples_en": ["Generate an image from a prompt", "Query an image generation request"],
            "minPlatformVersion": "1.0.0", "auth_mode": "cli"}
    manifest = {"runtime": {"kind": "node", "version": runtime_version, "digest": "sha256:" + match.group(1)},
                "executable": "picset-ai", "bundle_path": "node_modules/.bin/picset-ai", "authentication_driver": "connector_package",
                "commands": {"init": {"argv": ["init"]}, "auth": {"argv": ["auth"], "timeout_seconds": 60},
                             "status": {"argv": ["status"], "timeout_seconds": 60}, "unAuth": {"argv": ["unauth"]}},
                "status_match": {"json_path": "$.data.authenticated", "equals": True},
                "capabilities": capabilities, "auth_url_domains": ["picsetai.cn"], "egress_hosts": ["picsetai.cn"],
                "timeout_seconds": 180,
                "resource_limits": {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 180, "concurrency": 1, "child_processes": 64}}
    source_archive = source_zip(source, capabilities)
    files = {"connector-meta.json": json.dumps(meta, ensure_ascii=False, sort_keys=True).encode(),
             "cli.json": json.dumps(manifest, ensure_ascii=False, sort_keys=True).encode(),
             "icon.svg": icon, "cli-bundle.tgz": bundle(source_archive), "skills/picset-ai/SKILL.md": (ROOT / "SKILL.md").read_bytes(),
             "skills/picset-ai/capabilities.json": json.dumps(capabilities, sort_keys=True).encode(), "skills/picset-ai/api-reference.md": (ROOT / "api-reference.md").read_bytes()}
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
