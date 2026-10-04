#!/usr/bin/env python3
"""Build a deterministic Xiaoe MCP package and validate it with the repository parser."""
import argparse
import hashlib
import io
import json
import re
from pathlib import Path
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent


def build():
    files = {name: (ROOT / name).read_bytes() for name in ["connector-meta.json", "mcp.json", "icon.svg"]}
    files["skills/xiaoe/SKILL.md"] = (ROOT / "SKILL.md").read_bytes()
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as archive:
        for name, body in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.compress_type, entry.external_attr = zipfile.ZIP_DEFLATED, 0o100644 << 16
            archive.writestr(entry, body)
    return output.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    data = build()
    args.output.write_bytes(data)
    result = subprocess.run(["go", "-C", str(ROOT.parents[2] / "backend"), "run", "./cmd/connector-package-validate", str(args.output.resolve())], check=True, capture_output=True, text=True)
    print(result.stdout, end="")
    receipt = {"archive_sha256": hashlib.sha256(data).hexdigest(), "normalized_sha256": re.search(r" sha256=([a-f0-9]{64})", result.stdout).group(1)}
    args.output.with_suffix(".receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")


if __name__ == "__main__":
    main()
