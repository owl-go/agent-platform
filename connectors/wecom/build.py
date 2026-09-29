#!/usr/bin/env python3
"""Assemble the reviewed WeCom Connector from the pinned CLI Builder output."""

import argparse
import base64
import gzip
import hashlib
import io
import json
from pathlib import Path
import posixpath
import re
import tarfile
import zipfile


VERSION = "1.3.4"  # Reviewed upstream CLI release.
PACKAGE_VERSION = "1.4.1"  # Connector policy revision; independent of the CLI.
# runsc needs room for its sandbox processes before Node and the CLI can start.
RESOURCE_LIMITS = {"cpu_millis": 1000, "memory_mib": 512, "timeout_seconds": 900, "concurrency": 1, "child_processes": 128}
NPM_INTEGRITY = "sha512-vz47EsT/BKkHBONVt94xhZtUWhvMR4uF9Y4TTNUwF7PnLETqyFyrNNHwG3aTLmhMNC/ecqRWUUqvNgP90nreaQ=="
NATIVE_SHA256 = {
    "arm64": "6275ed033c054946c00805040cb1b016a45681e1bf11cad855d24427db897f71",
    "x64": "9d976b5c717b4f67667c96cd6115e4441d7ef6ff3a8b114b53806f7070bb1d92",
}
UPSTREAM_SHA256 = "db63f990446b0a41d03fdc1bca6efafd674d0b930addae27128941168d3ae7e5"
UPSTREAM_SKILLS = {
    "wecomcli-calendar", "wecomcli-contact", "wecomcli-disk", "wecomcli-doc-manage",
    "wecomcli-doc", "wecomcli-email", "wecomcli-media", "wecomcli-meeting",
    "wecomcli-message", "wecomcli-pptx", "wecomcli-shared", "wecomcli-sheet",
    "wecomcli-smartpage", "wecomcli-smartsheet", "wecomcli-todo",
}
HERE = Path(__file__).resolve().parent
EGRESS = ["qyapi.weixin.qq.com"]


def capability(name, argv, risk, identity):
    return {"id": name, "argv_prefix": argv, "risk": risk, "identities": [identity], "egress_hosts": EGRESS, "timeout_seconds": 180}


def reviewed_capabilities() -> list[dict]:
    policies = json.loads((HERE / "capabilities.json").read_text())
    capabilities = []
    seen = set()
    for policy in policies:
        command = policy["command"].split(" ")
        name = "_".join(command)
        if not all(re.fullmatch(r"[a-z][a-z0-9]*", part) for part in command):
            raise ValueError(f"invalid reviewed command: {command}")
        if name in seen or policy["identity"] not in ("user", "bot") or policy["risk"] not in ("low", "high"):
            raise ValueError(f"duplicate or invalid reviewed capability: {name}")
        seen.add(name)
        capabilities.append(capability(name, command, policy["risk"], policy["identity"]))
    return capabilities


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
    policy_module = (HERE / "policy.mjs").read_bytes()
    redactor = (HERE / "redact.mjs").read_bytes()
    policy = (HERE / "capabilities.json").read_bytes()
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
            record = tarfile.TarInfo("node_modules/.bin/policy.mjs")
            record.mode = 0o644
            record.size = len(policy_module)
            output.addfile(record, io.BytesIO(policy_module))
            record = tarfile.TarInfo("node_modules/.bin/capabilities.json")
            record.mode = 0o644
            record.size = len(policy)
            output.addfile(record, io.BytesIO(policy))
    return result.getvalue()


def upstream_resources() -> dict[str, bytes]:
    archive_body = (HERE / "upstream-v1.3.4.tar.gz").read_bytes()
    if hashlib.sha256(archive_body).hexdigest() != UPSTREAM_SHA256:
        raise ValueError("upstream Skill archive differs from the reviewed source")
    resources = {}
    with tarfile.open(fileobj=io.BytesIO(archive_body), mode="r:gz") as archive:
        for member in archive:
            if member.isdir():
                continue
            name = member.name.removeprefix("./")
            if not member.isfile() or name.startswith("/") or ".." in Path(name).parts or name in resources:
                raise ValueError("unsafe upstream Skill resource")
            resources[name] = archive.extractfile(member).read()
    skills = {Path(name).parent.name for name in resources if name.startswith("skills/") and name.endswith("/SKILL.md")}
    if skills != UPSTREAM_SKILLS or len(resources) != 125 or "docs/cli-reference.md" not in resources or "LICENSE" not in resources:
        raise ValueError("upstream Skill archive is incomplete")
    for name, body in resources.items():
        if not name.endswith(".md"):
            continue
        for raw in re.findall(rb"\[[^\]]*\]\(([^)]+)\)", body):
            target = raw.decode("utf-8").split("#", 1)[0].strip()
            if not target.startswith(("references/", "scripts/", "assets/", "../", "./")):
                continue
            resolved = posixpath.normpath(posixpath.join(posixpath.dirname(name), target))
            if resolved not in resources and not any(path.startswith(resolved.rstrip("/") + "/") for path in resources):
                raise ValueError(f"upstream Skill reference is missing: {name} -> {target}")
    return {"skills/wecom/references/upstream/" + name: body for name, body in resources.items()}


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
        "source": "wecom", "version": PACKAGE_VERSION, "type": "cli", "name": "企业微信",
        "description": "通过企业微信 CLI 处理消息、邮件、文档、表格、待办、日程、会议、微盘和通讯录。",
        "examples_zh": ["创建企业微信文档", "发送企业微信消息", "查询日程"],
        "examples_en": ["Create a WeCom document", "Send a WeCom message", "Find a schedule"],
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
        "capabilities": reviewed_capabilities(),
        "egress_hosts": EGRESS,
        "timeout_seconds": 180,
        "resource_limits": RESOURCE_LIMITS,
    }
    files = {
        "connector-meta.json": json.dumps(metadata, ensure_ascii=False, separators=(",", ":")).encode(),
        "cli.json": json.dumps(manifest, ensure_ascii=False, separators=(",", ":")).encode(),
        "icon.svg": (HERE / "icon.svg").read_bytes(),
        "cli-bundle.tgz": bundle,
        "skills/wecom/SKILL.md": (HERE / "SKILL.md").read_bytes(),
        "skills/wecom/UPSTREAM.md": (HERE / "UPSTREAM.md").read_bytes(),
        "skills/wecom/capabilities.json": (HERE / "capabilities.json").read_bytes(),
    }
    files.update(upstream_resources())
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
