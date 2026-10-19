"""Add the checksum-pinned ARM64 npm artifact to the reviewed x64 bundle.

Run with python3 -m connectors.wecom.dual_arch --arm64-package <npm.tgz>
--output <package.zip>. No npm scripts or third-party code run during assembly.
"""
import argparse
import base64
import copy
import gzip
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import tarfile
import zipfile

from connectors.wecom import build

ARM64_INTEGRITY = "sha512-BUvkJCMiM6aImK1MGjA6TuNslfay9eg/cOrmmvg3RLAlB9xTQUfqdq9YQxFog04wEpnlZSsgNXq6bWQaIvwX9w=="


def dual_bundle(existing: bytes, arm64: bytes) -> bytes:
    integrity = "sha512-" + base64.b64encode(hashlib.sha512(arm64).digest()).decode()
    if integrity != ARM64_INTEGRITY:
        raise ValueError("ARM64 npm artifact integrity mismatch")
    entries = {}
    with tarfile.open(fileobj=io.BytesIO(existing), mode="r:gz") as archive:
        for member in archive:
            name = member.name.removeprefix("./").rstrip("/")
            if not name:
                continue
            if name.startswith("/") or ".." in PurePosixPath(name).parts or name in entries:
                raise ValueError("unsafe or duplicate base bundle path")
            entries[name] = (copy.copy(member), archive.extractfile(member).read() if member.isfile() else b"")
    with tarfile.open(fileobj=io.BytesIO(arm64), mode="r:gz") as archive:
        for member in archive:
            if not member.isfile() or not member.name.startswith("package/"):
                raise ValueError("unexpected ARM64 npm artifact entry")
            relative = member.name.removeprefix("package/")
            if relative.startswith("/") or ".." in PurePosixPath(relative).parts:
                raise ValueError("unsafe ARM64 npm artifact path")
            name = "node_modules/@wecom/cli-linux-arm64/" + relative
            body = archive.extractfile(member).read()
            if name in entries and entries[name][1] != body:
                raise ValueError("ARM64 artifact conflicts with existing bundle")
            entries[name] = (copy.copy(member), body)
    for arch in ("arm64", "x64"):
        prefix = f"node_modules/@wecom/cli-linux-{arch}/"
        manifest = json.loads(entries[prefix + "package.json"][1])
        if manifest["version"] != build.VERSION or manifest["name"] != f"@wecom/cli-linux-{arch}":
            raise ValueError("native package identity differs from reviewed version")
        member, body = entries[prefix + "bin/wecom-cli"]
        if not member.mode & 0o111 or hashlib.sha256(body).hexdigest() != build.NATIVE_SHA256[arch]:
            raise ValueError("native binary differs from reviewed checksum")
    result = io.BytesIO()
    with gzip.GzipFile(fileobj=result, mode="wb", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w") as archive:
            for name, (member, body) in sorted(entries.items()):
                member.name, member.uid, member.gid = name, 0, 0
                member.uname, member.gname, member.mtime = "", "", 0
                archive.addfile(member, io.BytesIO(body) if member.isfile() else None)
    return result.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arm64-package", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    package = build.HERE / "package"
    bundle = dual_bundle((package / "cli-bundle.tgz").read_bytes(), args.arm64_package.read_bytes())
    (package / "cli-bundle.tgz").write_bytes(bundle)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for file in sorted(package.rglob("*")):
            if file.is_file():
                info = zipfile.ZipInfo(file.relative_to(package).as_posix(), (1980, 1, 1, 0, 0, 0))
                info.external_attr = 0o644 << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                archive.writestr(info, file.read_bytes())
    print(f"WeCom dual-architecture bundle sha256={hashlib.sha256(bundle).hexdigest()}")


if __name__ == "__main__":
    main()
