#!/usr/bin/env python3
"""Assemble a deterministic Notion CLI Connector Package from a pinned npm tarball."""

import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import tarfile
from urllib.request import urlopen
import zipfile


ROOT = Path(__file__).resolve().parent
VERSION = "0.23.13"
UPSTREAM_URL = f"https://registry.npmjs.org/ntn/-/ntn-{VERSION}.tgz"
UPSTREAM_SHA512 = "d987d6463b582496b778847504e982c04b1c96a6e247adb8ebc33bb77d167b77811664eca0f00388f94b1b65a36754e256ff0e3522775725546ae0b39775f854"
UPSTREAM_FILES = {
    "package/dist/ntn-linux-arm64/ntn": "node_modules/ntn/dist/ntn-linux-arm64/ntn",
    "package/dist/ntn-linux-x64/ntn": "node_modules/ntn/dist/ntn-linux-x64/ntn",
    "package/package.json": "node_modules/ntn/package.json",
    "package/LICENSE.md": "node_modules/ntn/LICENSE.md",
}


def member_bytes(archive, name):
    member = archive.getmember(name)
    if not member.isfile():
        raise ValueError(f"upstream member is not a regular file: {name}")
    return archive.extractfile(member).read()


def build_bundle(upstream):
    content = {"node_modules/.bin/ntn": (ROOT / "bridge.cjs").read_bytes()}
    with tarfile.open(fileobj=io.BytesIO(upstream), mode="r:gz") as archive:
        for source, target in UPSTREAM_FILES.items():
            content[target] = member_bytes(archive, source)
    manifest = json.loads(content["node_modules/ntn/package.json"])
    if manifest.get("name") != "ntn" or manifest.get("version") != VERSION:
        raise ValueError("upstream npm package identity changed")
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, mode="wb", mtime=0) as stream:
        with tarfile.open(fileobj=stream, mode="w") as archive:
            for name, body in sorted(content.items()):
                entry = tarfile.TarInfo(name)
                entry.size = len(body)
                entry.mode = 0o755 if name.endswith("/ntn") else 0o644
                entry.mtime = 0
                archive.addfile(entry, io.BytesIO(body))
    return compressed.getvalue()


def add_zip_file(archive, name, body):
    item = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
    item.compress_type = zipfile.ZIP_DEFLATED
    item.external_attr = 0o100644 << 16
    archive.writestr(item, body)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime-image", required=True, help="actual Runtime registry RepoDigest")
    parser.add_argument("--output", required=True, type=Path)
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

    bundle = build_bundle(upstream)
    policy = json.loads((ROOT / "cli-policy.json").read_text())
    policy["runtime"] = {"kind": "node", "version": "24.15.0", "digest": "sha256:" + match.group(1)}
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
