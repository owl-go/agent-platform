#!/usr/bin/env python3
"""Build a deterministic Caoliao public MCP Connector Package ZIP."""

import argparse
import hashlib
import json
from pathlib import Path
import zipfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    source = Path(__file__).with_name("package")
    names = ["connector-meta.json", "icon.svg", "mcp.json", "skills/caoliao/SKILL.md"]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
        for name in sorted(names):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            path = source / name
            if name == "icon.svg":
                path = Path(__file__).resolve().parents[3] / "backend/internal/connectorpackage/icons/caoliao.svg"
            archive.writestr(info, path.read_bytes())
    content = args.output.read_bytes()
    print(json.dumps({"output": str(args.output), "bytes": len(content),
                      "sha256": hashlib.sha256(content).hexdigest()}, ensure_ascii=False))


if __name__ == "__main__":
    main()
