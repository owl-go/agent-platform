#!/usr/bin/env python3
"""Build both reviewed legal MCP packages and validate their final ZIPs."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent
SOURCES = ("pkulaw", "mindbye")


def build(source):
    if source not in SOURCES:
        raise ValueError("unreviewed legal connector")
    directory = ROOT / source
    files = {name: (directory / name).read_bytes() for name in ("connector-meta.json", "mcp.json", "icon.svg")}
    files[f"skills/{source}/SKILL.md"] = (directory / "SKILL.md").read_bytes()
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as archive:
        for name, body in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.compress_type, entry.external_attr = zipfile.ZIP_DEFLATED, 0o100644 << 16
            archive.writestr(entry, body)
    return output.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-directory", type=Path, required=True)
    args = parser.parse_args()
    args.output_directory.mkdir(parents=True, exist_ok=True)
    for source in SOURCES:
        output = args.output_directory / f"{source}-1.0.0.zip"
        data = build(source)
        output.write_bytes(data)
        result = subprocess.run(["go", "-C", str(ROOT.parents[2] / "backend"), "run",
                                 "./cmd/connector-package-validate", str(output.resolve())],
                                check=True, capture_output=True, text=True)
        print(result.stdout, end="")
        receipt = {"archive_sha256": hashlib.sha256(data).hexdigest(),
                   "normalized_sha256": re.search(r" sha256=([a-f0-9]{64})", result.stdout).group(1)}
        output.with_suffix(".receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")


if __name__ == "__main__":
    main()
