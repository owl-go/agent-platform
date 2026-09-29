#!/usr/bin/env python3
"""Assemble the reviewed WeCom Connector from the pinned CLI Builder output."""

import argparse
import base64
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import tarfile
import zipfile


VERSION = "1.3.4"
NPM_INTEGRITY = "sha512-vz47EsT/BKkHBONVt94xhZtUWhvMR4uF9Y4TTNUwF7PnLETqyFyrNNHwG3aTLmhMNC/ecqRWUUqvNgP90nreaQ=="
NATIVE_SHA256 = {
    "arm64": "6275ed033c054946c00805040cb1b016a45681e1bf11cad855d24427db897f71",
    "x64": "9d976b5c717b4f67667c96cd6115e4441d7ef6ff3a8b114b53806f7070bb1d92",
}
HERE = Path(__file__).resolve().parent
EGRESS = ["qyapi.weixin.qq.com"]


def capability(name, argv, risk, identity):
    return {"id": name, "argv_prefix": argv, "risk": risk, "identities": [identity], "egress_hosts": EGRESS, "timeout_seconds": 60}


CAPABILITIES = [
    capability("identity_whoami", ["identity", "whoami"], "low", "user"),
    capability("contact_users_search", ["contact", "users", "search"], "low", "user"),
    capability("doc_search", ["doc", "search"], "low", "user"),
    capability("doc_contents_get", ["doc", "contents", "get"], "low", "user"),
    capability("todo_create", ["todo", "create"], "high", "user"),
    capability("message_sessions_list", ["message", "aibot", "sessions", "list"], "low", "bot"),
    capability("message_send", ["message", "aibot", "send"], "high", "bot"),
]


def reviewed_bundle(source: bytes, arch: str) -> bytes:
    entries = {}
    with tarfile.open(fileobj=io.BytesIO(source), mode="r:gz") as archive:
        for member in archive:
            name = member.name.removeprefix("./")
            if not name or name == ".":
                continue
            if name.startswith("/") or ".." in Path(name).parts or name in entries:
                raise ValueError("unsafe or duplicate upstream bundle member")
            data = archive.extractfile(member).read() if member.isfile() else b""
            entries[name] = (member, data)
    manifest = json.loads(entries["node_modules/@wecom/cli/package.json"][1])
    native = json.loads(entries[f"node_modules/@wecom/cli-linux-{arch}/package.json"][1])
    if manifest.get("version") != VERSION or native.get("version") != VERSION:
        raise ValueError("upstream package version changed")
    native_path = f"node_modules/@wecom/cli-linux-{arch}/bin/wecom-cli"
    if native_path not in entries:
        raise ValueError("missing pinned Linux executable")
    native_member, native_binary = entries[native_path]
    if native_member.mode & 0o111 == 0 or hashlib.sha256(native_binary).hexdigest() != NATIVE_SHA256[arch]:
        raise ValueError("native executable differs from the reviewed release")
    wrapper = (HERE / "wecom-workspace.mjs").read_bytes()
    redactor = (HERE / "redact.mjs").read_bytes()
    result = io.BytesIO()
    with gzip.GzipFile(fileobj=result, mode="wb", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w") as output:
            for name in sorted(entries):
                member, data = entries[name]
                record = tarfile.TarInfo(name)
                record.type = member.type
                record.linkname = member.linkname
                record.mode = member.mode
                record.size = len(data) if member.isfile() else 0
                output.addfile(record, io.BytesIO(data) if member.isfile() else None)
            record = tarfile.TarInfo("node_modules/.bin/wecom-workspace")
            record.mode = 0o755
            record.size = len(wrapper)
            output.addfile(record, io.BytesIO(wrapper))
            record = tarfile.TarInfo("node_modules/.bin/redact.mjs")
            record.mode = 0o644
            record.size = len(redactor)
            output.addfile(record, io.BytesIO(redactor))
    return result.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--builder-output", required=True, type=Path)
    parser.add_argument("--runtime-digest", required=True)
    parser.add_argument("--runtime-version", default="24.15.0")
    parser.add_argument("--arch", choices=["x64", "arm64"], required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", args.runtime_digest):
        parser.error("runtime digest must be an exact sha256 RepoDigest")
    builder = args.builder_output
    if (builder / "integrity.txt").read_text().strip() != NPM_INTEGRITY:
        raise ValueError("upstream npm integrity differs from the reviewed release")
    actual_integrity = "sha512-" + base64.b64encode(hashlib.sha512((builder / "package.tgz").read_bytes()).digest()).decode()
    if actual_integrity != NPM_INTEGRITY:
        raise ValueError("upstream npm tarball differs from the reviewed release")
    bundle = reviewed_bundle((builder / "bundle.tgz").read_bytes(), args.arch)
    metadata = {
        "source": "wecom", "version": VERSION, "type": "cli", "name": "企业微信",
        "description": "使用经审核的企业微信 CLI 命令访问当前 User 已授权的资源。",
        "examples_zh": ["搜索企业微信文档", "创建企业微信待办"],
        "examples_en": ["Search WeCom documents", "Create a WeCom task"],
        "minPlatformVersion": "1.0.0", "auth_mode": "cli",
    }
    manifest = {
        "runtime": {"kind": "node", "version": args.runtime_version, "digest": args.runtime_digest},
        "executable": "wecom-workspace", "bundle_path": "node_modules/.bin/wecom-workspace",
        "authentication_driver": "connector_package",
        "commands": {
            "init": {"argv": ["platform", "status"]},
            "auth": {"argv": ["platform", "authorize"]},
            "status": {"argv": ["platform", "status"]},
            "unAuth": {"argv": ["platform", "revoke"]},
        },
        "status_match": {"json_path": "$.credentials_present", "equals": True},
        "capabilities": CAPABILITIES,
        "egress_hosts": EGRESS,
        "timeout_seconds": 60,
        "resource_limits": {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 900, "concurrency": 1, "child_processes": 8},
    }
    files = {
        "connector-meta.json": json.dumps(metadata, ensure_ascii=False, separators=(",", ":")).encode(),
        "cli.json": json.dumps(manifest, ensure_ascii=False, separators=(",", ":")).encode(),
        "icon.svg": (HERE / "icon.svg").read_bytes(),
        "cli-bundle.tgz": bundle,
        "skills/wecom/SKILL.md": (HERE / "SKILL.md").read_bytes(),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for name, body in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.compress_type = zipfile.ZIP_DEFLATED
            entry.external_attr = 0o644 << 16
            archive.writestr(entry, body)
    print(json.dumps({"package": str(args.output), "sha256": hashlib.sha256(args.output.read_bytes()).hexdigest(), "bundle_sha256": hashlib.sha256(bundle).hexdigest()}))


if __name__ == "__main__":
    main()
