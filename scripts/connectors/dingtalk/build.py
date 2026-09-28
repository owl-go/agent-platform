#!/usr/bin/env python3
"""Build a reviewable DingTalk CLI Connector Package from pinned upstream artifacts.

This does not record Runtime Conformance or publish the package.
"""

import argparse
import base64
import gzip
import hashlib
import io
import json
import re
import tarfile
import zipfile
from pathlib import Path


VERSION = "1.0.64"
NPM_SHA512 = "j4B+Daqil+mvVSKHPeBdBeGxNPuSWoXjgT5fu/R3WQUeAQHw0cWmMuqtLNjjwgW7eZfT98JN2yJ+j3TF5cEsBg=="
ASSET_SHA256 = {
    "dws-linux-amd64.tar.gz": "6198a86570ea52f24d88a58dfe65514540793c4dd64410cb133a8ff7b3b008a8",
    "dws-linux-arm64.tar.gz": "7b015a3cf5104e4477786661fe42e97616961aa43ef6075bc5954b815d09a853",
    "dws-skills.zip": "0d347947118f67cee8bc84ec896777acfe8888db39d1ff5ee22d103dd5c433df",
}
BUSINESS_EXCLUSIONS = {"audit", "dev", "devapp", "event", "mcp", "pat"}
EGRESS_HOSTS = [
    "aflow.dingtalk.com", "aihub.dingtalk.com", "alidocs.dingtalk.com",
    "alidocs2.oss-cn-zhangjiakou.aliyuncs.com", "alimail-cn.aliyuncs.com",
    "alimail-personal.aliyuncs.com",
    "api.dingtalk.com", "api.dingtalk.io", "docs.dingtalk.com",
    "down.dingtalk.com", "img.alicdn.com",
    "login.dingtalk.com", "login.dingtalk.io", "mcp-gw.dingtalk.com",
    "mcp.dingtalk.com", "mcp.dingtalk.io", "oapi.dingtalk.com",
    "open.dingtalk.com", "open-dev.dingtalk.com", "open-dev.dingtalk.io",
    "qr.dingtalk.com", "shanji.dingtalk.com", "www.dingtalk.com",
]
SKILL = """---
name: dingtalk
display_name: 钉钉 CLI
description: 使用已审核的 DingTalk Workspace CLI 命令操作钉钉业务资源。
version: 1.0.64
author: DingTalk-Real-AI / Agent Workspace
---

# 钉钉 CLI

本包固定 DingTalk Workspace CLI v1.0.62。先确认 Connector Installation 已启用、账号授权有效、当前 Runtime Digest 已通过该 bundle 的 Conformance。未满足任一条件时停止，不尝试在会话中重新安装 CLI 或绕过授权。

按产品读取同包的[钉钉官方 MultiSkill 入口](reference/dingtalk-shared/SKILL.md)及对应产品 Skill。官方技能是使用说明；最终可调用命令以本修订 [capabilities.json](capabilities.json) 的能力白名单为准。先查目标命令的精确 `dws <path> --help`；参数或风险不明确时查 `dws schema --cli-path "<path>" --compact --format json`。不要猜命令或参数，尤其不要沿用其他版本的指南。

所有业务命令使用当前用户身份和 `--format json`。查找接收人、群、文档、待办等目标时先读取真实 ID；零命中或多候选时请用户消歧。写入前核对组织、账号、对象和内容。高风险 capability 的每一次 `agent-cli` 调用（包括 `--help` 和 `--dry-run`）都必须在命令分隔符 `--` 之前提供非空、可展示且不含 Secret 的具体 `--target`，格式为 `agent-cli --connector <id> --capability <capability-id> --identity user --target "<具体目标>" -- <DWS 子命令和参数>`；例如建群目标可写群名，不得写“当前操作”等泛化值。低风险 capability 不需要虚构 target。

高风险命令遵守平台一次性批准。平台批准卡就是该次命令的显式用户确认；上游 DWS 写命令需要 `--yes` 时，应将它放进同一份待平台批准的 argv，平台会先阻断，批准后才启动 CLI，不要先执行一遍无 `--yes` 的命令。若返回 `user_action_unavailable`，先检查高风险调用是否遗漏 `--target` 并用完整参数重试；不得把 `user_action_unavailable` 解释为钉钉确认功能缺失。写后读取结果或对象验证，超时或结果不明时先对账，不盲目重发。

平台会在每条业务命令执行前检查 Installation 授权，并在缺失或过期时由会话输入区提供钉钉授权入口。不要调用 `dws auth status` 或 `dws profile list` 判断平台授权：它们只检查 CLI 本地 Profile，会把平台注入的短期令牌误报为未登录。平台在单次隔离进程环境中提供短期 Access Token，Refresh Token 保留在平台加密存储中；不得把凭证写入命令参数、Skill 或工作区文件。

官方资料：[DWS CLI](https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli)、[用户指南](https://open.dingtalk.com/document/development/dingtalk-cli-performing-tasks-within)、[应用管理指南](https://open.dingtalk.com/document/development/dev-cli-app-management-guide)。
"""
LAUNCHER = b"""#!/bin/sh
case "$(uname -m)" in
  x86_64) binary="${0%/*}/dws-linux-amd64" ;;
  aarch64) binary="${0%/*}/dws-linux-arm64" ;;
  *) echo 'unsupported DingTalk CLI architecture' >&2; exit 1 ;;
esac
exec "$binary" "$@"
"""


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def npm_assets(package_bytes):
    actual = base64.b64encode(hashlib.sha512(package_bytes).digest()).decode()
    if actual != NPM_SHA512:
        raise ValueError("npm package integrity differs from pinned v1.0.62")
    wanted = {"package/assets/" + name for name in ASSET_SHA256}
    found = {}
    with tarfile.open(fileobj=io.BytesIO(package_bytes), mode="r:gz") as archive:
        for member in archive:
            if member.name not in wanted:
                continue
            if not member.isfile() or member.name in found:
                raise ValueError("duplicate or non-file npm asset")
            found[member.name] = archive.extractfile(member).read()
    if set(found) != wanted:
        raise ValueError("pinned npm package is missing a required asset")
    assets = {name: found["package/assets/" + name] for name in ASSET_SHA256}
    for name, expected in ASSET_SHA256.items():
        if sha256(assets[name]) != expected:
            raise ValueError("asset checksum mismatch: " + name)
    return assets


def linux_binary(asset):
    with tarfile.open(fileobj=io.BytesIO(asset), mode="r:gz") as archive:
        members = [item for item in archive if item.name.lstrip("./") == "dws"]
        if len(members) != 1 or not members[0].isfile():
            raise ValueError("release asset has no unique dws executable")
        binary = archive.extractfile(members[0]).read()
    if not binary.startswith(b"\x7fELF"):
        raise ValueError("release asset is not an ELF executable")
    return binary


def reviewed_capabilities(schema, include_admin):
    if schema.get("tool_count") != 1435 or schema.get("catalog_hash") != "sha256:857592291504d44aae4c4d94f9dad462dc472bafbd8d8235ffa3b68bd4433bd5":
        raise ValueError("schema does not match reviewed CLI v1.0.62")
    tools = [tool for product in schema["products"] for tool in product["tools"]]
    if len(tools) != schema["tool_count"]:
        raise ValueError("schema tool count changed")
    paths = set()
    capabilities = []
    for tool in tools:
        if tool["availability"] != "available":
            continue
        path = tool.get("cli_path", "").split()
        if len(path) < 2 or path[0] in BUSINESS_EXCLUSIONS and not include_admin:
            continue
        if tuple(path) in paths:
            raise ValueError("duplicate CLI path in schema")
        paths.add(tuple(path))
        is_read = tool["effect"] == "read" and tool["risk"] == "low" and tool["confirmation"] == "not_required"
        capabilities.append({
            "id": "dws_" + sha256(" ".join(path).encode())[:16],
            "argv_prefix": path,
            "risk": "low" if is_read else "high",
            "identities": ["user"],
            "scopes": [],
            "egress_hosts": EGRESS_HOSTS,
            "timeout_seconds": 120,
        })
    for path in paths:
        if any(len(other) > len(path) and other[:len(path)] == path for other in paths):
            raise ValueError("overlapping CLI paths would weaken the capability policy")
    if not capabilities:
        raise ValueError("no reviewed CLI capabilities")
    capabilities.extend([
        {"id": "dws_version", "argv_prefix": ["version"], "risk": "low", "identities": ["user"], "scopes": [], "egress_hosts": EGRESS_HOSTS, "timeout_seconds": 30},
        {"id": "dws_schema", "argv_prefix": ["schema"], "risk": "low", "identities": ["user"], "scopes": [], "egress_hosts": EGRESS_HOSTS, "timeout_seconds": 60},
    ])
    return sorted(capabilities, key=lambda item: item["argv_prefix"])


def skills(assets):
    result = {"skills/dingtalk/SKILL.md": SKILL.encode()}
    with zipfile.ZipFile(io.BytesIO(assets["dws-skills.zip"])) as archive:
        for legal in ("LICENSE", "NOTICE"):
            result["skills/dingtalk/reference/" + legal] = archive.read(legal)
        for member in archive.infolist():
            if not member.filename.startswith("multi/") or member.is_dir():
                continue
            relative = member.filename.removeprefix("multi/")
            if relative.startswith("/") or ".." in Path(relative).parts or "\\" in relative:
                raise ValueError("unsafe official Skill path")
            result["skills/dingtalk/reference/" + relative] = archive.read(member)
    if "skills/dingtalk/reference/dingtalk-shared/SKILL.md" not in result:
        raise ValueError("official MultiSkill entry is missing")
    return result


def verified_platform_binary(binary, machine):
    if not binary.startswith(b"\x7fELF") or int.from_bytes(binary[18:20], "little") != machine:
        raise ValueError("patched DingTalk executable has the wrong ELF architecture")
    if b"AGENT_PLATFORM_DWS_ACCESS_TOKEN" not in binary:
        raise ValueError("DingTalk executable lacks the reviewed environment token bridge")
    return binary


def bundle(amd64_binary, arm64_binary):
    files = {
        "node_modules/.bin/dws": LAUNCHER,
        "node_modules/.bin/dws-linux-amd64": verified_platform_binary(amd64_binary, 62),
        "node_modules/.bin/dws-linux-arm64": verified_platform_binary(arm64_binary, 183),
    }
    output = io.BytesIO()
    with gzip.GzipFile(fileobj=output, mode="wb", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w") as archive:
            for name, content in sorted(files.items()):
                info = tarfile.TarInfo(name)
                info.size = len(content)
                info.mode = 0o755
                info.mtime = 0
                archive.addfile(info, io.BytesIO(content))
    return output.getvalue()


def build(package_bytes, schema, runtime_image, runtime_version, include_admin, amd64_binary, arm64_binary):
    match = re.fullmatch(r"[^\s@]+(?:/[^\s@]+)*@sha256:([a-f0-9]{64})", runtime_image)
    if not match or not re.fullmatch(r"\d+\.\d+\.\d+", runtime_version):
        raise ValueError("require a digest-pinned Runtime image reference and exact Runtime version")
    assets = npm_assets(package_bytes)
    reviewed = reviewed_capabilities(schema, include_admin)
    icon = Path(__file__).with_name("dingtalk.png").read_bytes()
    if sha256(icon) != "fbffbab13dd378fa25cb0ccb7540c0d57a4b7328058beb677b344cea8662124d":
        raise ValueError("DingTalk icon differs from the reviewed Open Platform asset")
    icon_svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><image width="128" height="128" href="data:image/png;base64,' + base64.b64encode(icon).decode() + '"/></svg>'
    meta = {
        "source": "dingtalk", "version": VERSION, "type": "cli", "name": "钉钉",
        "description": "DingTalk Workspace CLI 业务操作；安装后通过钉钉设备授权连接账号。",
        "examples_zh": ["查询钉钉待办", "搜索钉钉文档"],
        "examples_en": ["List DingTalk tasks", "Search DingTalk documents"],
        "minPlatformVersion": "1.0.0", "auth_mode": "cli",
    }
    manifest = {
        "runtime": {"kind": "node", "version": runtime_version, "digest": "sha256:" + match.group(1)},
        "executable": "dws", "bundle_path": "node_modules/.bin/dws",
        "authentication_driver": "dingtalk",
        "commands": {
            "init": {"argv": ["version"]},
            "auth": {"argv": ["auth", "login", "--device", "--format", "json"]},
            "status": {"argv": ["auth", "status", "--format", "json"]},
            "unAuth": {"argv": ["auth", "logout", "--yes", "--format", "json"]},
        },
        "status_match": {"json_path": "$.authenticated", "equals": True},
        "capabilities": reviewed,
        "auth_url_domains": ["login.dingtalk.com", "login.dingtalk.io", "open.dingtalk.com"],
        "egress_hosts": EGRESS_HOSTS, "timeout_seconds": 120,
        "resource_limits": {"cpu_millis": 1000, "memory_mib": 1024, "timeout_seconds": 900, "concurrency": 1, "child_processes": 64},
    }
    files = skills(assets)
    files["skills/dingtalk/capabilities.json"] = json.dumps(reviewed, ensure_ascii=False, sort_keys=True).encode()
    files.update({
        "connector-meta.json": json.dumps(meta, ensure_ascii=False, sort_keys=True).encode(),
        "icon.svg": icon_svg.encode(),
        "cli.json": json.dumps(manifest, ensure_ascii=False, sort_keys=True).encode(),
        "cli-bundle.tgz": bundle(amd64_binary, arm64_binary),
    })
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for name, content in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, content)
    return output.getvalue(), len(reviewed), sha256(files["cli-bundle.tgz"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--npm-tgz", type=Path, required=True)
    parser.add_argument("--schema", type=Path, required=True, help="dws v1.0.62 schema --all --format json")
    parser.add_argument("--runtime-image", required=True, help="Runtime repository@sha256:<64 lowercase hex>; publication requires a Registry RepoDigest")
    parser.add_argument("--runtime-version", required=True)
    parser.add_argument("--include-admin", action="store_true", help="also allow development, PAT, audit and event commands")
    parser.add_argument("--amd64-binary", type=Path, required=True, help="pinned-source DWS binary with the reviewed environment token bridge")
    parser.add_argument("--arm64-binary", type=Path, required=True, help="pinned-source DWS binary with the reviewed environment token bridge")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    archive, count, bundle_sha = build(args.npm_tgz.read_bytes(), json.loads(args.schema.read_text()), args.runtime_image, args.runtime_version, args.include_admin, args.amd64_binary.read_bytes(), args.arm64_binary.read_bytes())
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(archive)
    print(json.dumps({"output": str(args.output), "bytes": len(archive), "sha256": sha256(archive), "bundle_sha256": bundle_sha, "capabilities": count, "conformance": "not_run", "authorization": "dingtalk_device_flow"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
