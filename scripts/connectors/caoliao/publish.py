#!/usr/bin/env python3
"""Stage and publish the reviewed Caoliao public MCP package via platform APIs."""

import argparse
import base64
import importlib.util
import json
from pathlib import Path
import re
from urllib import parse
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("platform_publication", ROOT.parent / "teambition/publish.py")
platform = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--package", type=Path, required=True)
    parser.add_argument("--normalized-sha256", required=True)
    parser.add_argument("--evidence-directory", type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch("[a-f0-9]{64}", args.normalized_sha256):
        parser.error("normalized SHA-256 must come from connectorpackage.Parse")
    config = platform.read_config(args.config)
    base = config["VITE_OIDC_AUTHORITY"].split("/identity/realms/")[0]
    origin = parse.urlparse(base)
    if origin.scheme != "https" or not origin.netloc or origin.username or origin.path or origin.query or origin.fragment:
        raise RuntimeError("platform requires an HTTPS origin")
    archive = args.package.read_bytes()
    with zipfile.ZipFile(args.package) as package:
        meta = json.loads(package.read("connector-meta.json"))
        manifest = json.loads(package.read("mcp.json"))
    if (meta["source"], meta["version"], meta["type"], meta["auth_mode"]) != ("caoliao", "1.0.0", "mcp", "none") or manifest != {
        "transport": "streamable_http", "url": "https://mcp.objqr.com/mcp",
        "egress_hosts": ["mcp.objqr.com"], "timeout_seconds": 30,
    }:
        raise RuntimeError("package differs from reviewed public MCP policy")
    token = platform.administrator_token(config)
    listing = platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
    items = [item for item in listing if item["revision"]["source"] == "caoliao"]
    target = next((item["revision"] for item in items if item["revision"]["sha256"] == args.normalized_sha256
                   and item["revision"]["package_version"] == meta["version"]), None)
    if target is None:
        target = platform.api(base, token, "POST", "/api/v1/admin/connectors/packages",
                              {"archive": base64.b64encode(archive).decode()})
    if target["sha256"] != args.normalized_sha256 or target["source"] != "caoliao" or target["mode"] != "mcp":
        raise RuntimeError("staged revision differs from verified package")
    current = next((item["publication"] for item in items if item.get("publication")), None)
    if not current or current["active_revision_id"] != target["id"] or current["state"] != "available":
        current = platform.api(base, token, "POST", "/api/v1/admin/connectors/publications/"
                               + parse.quote(target["id"]) + "/publish",
                               {"expected_version": current["version"] if current else 0})
    installation = platform.api(base, token, "POST", "/api/v1/connectors/catalog/caoliao/install", {})
    if installation["active_revision_id"] != target["id"]:
        installation = platform.api(base, token, "POST", "/api/v1/connectors/" + parse.quote(installation["id"])
                                    + "/upgrade", {"expected_version": installation["version"]})
    user = [item for item in platform.api(base, token, "GET", "/api/v1/connectors/catalog").get("items", [])
            if item["source"] == "caoliao"]
    admin = [item for item in platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
             if (item.get("publication") or {}).get("source") == "caoliao"]
    installed = [item for item in platform.api(base, token, "GET", "/api/v1/connectors").get("items", [])
                 if item["source"] == "caoliao"]
    if len(user) != 1 or len(admin) != 1 or len(installed) != 1:
        raise RuntimeError("catalogs must each contain one official Caoliao entry")
    if (current["state"] != "available" or user[0]["active_revision_id"] != target["id"]
            or installation["state"] != "active" or installation["active_revision_id"] != target["id"]
            or not installation.get("authorized")):
        raise RuntimeError("publication or no-auth installation did not activate expected revision")
    if not user[0]["revision"]["icon"].startswith("data:image/svg+xml;base64,"):
        raise RuntimeError("Caoliao brand projection is not deployed")
    args.evidence_directory.mkdir(parents=True, exist_ok=True)
    for filename, value in [("revision.json", target), ("publication.json", current), ("installation.json", installation),
                            ("user-catalog.json", user), ("administrator-catalog.json", admin)]:
        (args.evidence_directory / filename).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"source": "caoliao", "revision_id": target["id"], "installation_id": installation["id"],
                      "publication": "available", "installation": "active", "authorized": True}))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("publication_failed", str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__)
        raise SystemExit(1)
