#!/usr/bin/env python3
"""Build the login theme with the workspace's authoritative design tokens."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil


def build(destination: Path) -> None:
    root = Path(__file__).resolve().parents[1]
    source = root / "deploy/platform/themes/agent-workspace"
    if destination.exists():
        raise ValueError("Theme output already exists; use a new release directory")
    if any(path.is_symlink() for path in source.rglob("*")):
        raise ValueError("Theme source must not contain symbolic links")
    theme = destination / "agent-workspace"
    shutil.copytree(source, theme)
    shutil.copyfile(root / "frontend/src/design-tokens.css", theme / "login/resources/css/design-tokens.css")
    checksums = {str(path.relative_to(destination)): hashlib.sha256(path.read_bytes()).hexdigest()
                 for path in sorted(destination.rglob("*")) if path.is_file()}
    (destination / "manifest.json").write_text(json.dumps(checksums, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path)
    build(parser.parse_args().destination)
