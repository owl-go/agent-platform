#!/usr/bin/env python3
"""Build matching CLI source ZIP and Connector Package from pinned ntn binaries."""

import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
from urllib.request import urlopen
import zipfile


ROOT = Path(__file__).resolve().parent
VERSION = "0.23.13"
UPSTREAM_URL = f"https://registry.npmjs.org/ntn/-/ntn-{VERSION}.tgz"
UPSTREAM_SHA512 = "d987d6463b582496b778847504e982c04b1c96a6e247adb8ebc33bb77d167b77811664eca0f00388f94b1b65a36754e256ff0e3522775725546ae0b39775f854"
UPSTREAM_FILES = {
    "package/dist/ntn-linux-arm64/ntn": "ntn/dist/ntn-linux-arm64/ntn",
    "package/dist/ntn-linux-x64/ntn": "ntn/dist/ntn-linux-x64/ntn",
    "package/LICENSE.md": "LICENSE.md",
}


def member_bytes(archive, name):
    member = archive.getmember(name)
    if not member.isfile():
        raise ValueError(f"upstream member is not a regular file: {name}")
    return archive.extractfile(member).read()


def build_source(upstream, policy):
    content = {"bin/ntn": (ROOT / "bridge.cjs").read_bytes()}
    with tarfile.open(fileobj=io.BytesIO(upstream), mode="r:gz") as archive:
        for source, target in UPSTREAM_FILES.items():
            content[target] = member_bytes(archive, source)
        upstream_manifest = json.loads(member_bytes(archive, "package/package.json"))
    if upstream_manifest.get("name") != "ntn" or upstream_manifest.get("version") != VERSION:
        raise ValueError("upstream npm package identity changed")
    manifest = {
        "name": "agent-workspace-notion-cli",
        "version": VERSION,
        "license": "MIT",
        "bin": {"ntn": "bin/ntn"},
        "agentWorkspace": {
            "executable": "ntn",
            "authenticationDriver": "connector_package",
            "resourceLimits": {
                "cpuMillis": policy["resource_limits"]["cpu_millis"],
                "memoryMiB": policy["resource_limits"]["memory_mib"],
                "childProcesses": policy["resource_limits"]["child_processes"],
            },
            "supportedArchitectures": ["linux-amd64", "linux-arm64"],
            "capabilities": [
                {
                    "id": item["id"],
                    "argvPrefix": item["argv_prefix"],
                    "risk": item["risk"],
                    "identities": item["identities"],
                    "scopes": item.get("scopes", []),
                    "egressHosts": item["egress_hosts"],
                    "timeoutSeconds": item["timeout_seconds"],
                }
                for item in policy["capabilities"]
            ],
        },
    }
    content["package.json"] = (json.dumps(manifest, sort_keys=True, separators=(",", ":")) + "\n").encode()
    return content


def add_zip_file(archive, name, body):
    item = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
    item.compress_type = zipfile.ZIP_DEFLATED
    item.external_attr = (0o100755 if name.endswith("/ntn") else 0o100644) << 16
    archive.writestr(item, body)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-image", required=True, help="actual Runtime registry RepoDigest")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--source-output", type=Path, help="matching source ZIP for the platform CLI Builder")
    parser.add_argument("--tarball", type=Path, help="verified local copy of ntn npm tarball")
    args = parser.parse_args()
    match = re.fullmatch(r"[^\s@]+@sha256:([0-9a-f]{64})", args.runtime_image)
    if not match:
        parser.error("--runtime-image must be a registry RepoDigest")

    if args.tarball:
        upstream = args.tarball.read_bytes()
    else:
        with urlopen(UPSTREAM_URL, timeout=60) as response:
            upstream = response.read(50 << 20)
    if hashlib.sha512(upstream).hexdigest() != UPSTREAM_SHA512:
        raise ValueError("ntn npm tarball SHA-512 does not match the pinned release")

    policy = json.loads((ROOT / "cli-policy.json").read_text())
    policy["runtime"] = {"kind": "node", "version": "24.15.0", "digest": "sha256:" + match.group(1)}
    source_files = build_source(upstream, policy)
    with tempfile.TemporaryDirectory(prefix="notion-connector-build-") as directory:
        source_path = args.source_output.resolve() if args.source_output else Path(directory) / "source.zip"
        source_path.parent.mkdir(parents=True, exist_ok=True)
        with zipfile.ZipFile(source_path, "w") as source_archive:
            for name, body in sorted(source_files.items()):
                add_zip_file(source_archive, name, body)
        bundle_path = Path(directory) / "bundle.tgz"
        subprocess.run(
            ["go", "-C", str(ROOT.parent.parent / "backend"), "run", "./cmd/connector-zip-build", str(source_path), str(bundle_path)],
            check=True,
        )
        bundle = bundle_path.read_bytes()
        if args.source_output:
            print(f"source={source_path}")
            print(f"source_sha256={hashlib.sha256(source_path.read_bytes()).hexdigest()}")
    files = {
        "connector-meta.json": (ROOT / "connector-meta.json").read_bytes(),
        "icon.svg": (ROOT / "icon.svg").read_bytes(),
        "cli.json": (json.dumps(policy, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode(),
        "cli-bundle.tgz": bundle,
        "skills/notion/SKILL.md": (ROOT / "SKILL.md").read_bytes(),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, "w") as archive:
        for name, body in sorted(files.items()):
            add_zip_file(archive, name, body)
    print(f"package={args.output}")
    print(f"package_sha256={hashlib.sha256(args.output.read_bytes()).hexdigest()}")
    print(f"bundle_sha256={hashlib.sha256(bundle).hexdigest()}")
    print(f"upstream_npm_sha512={UPSTREAM_SHA512}")


if __name__ == "__main__":
    main()
