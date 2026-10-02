#!/usr/bin/env python3
"""Publish the exact reviewed legal MCP ZIPs without creating duplicate entries."""
import argparse
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
from urllib import parse

import build

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("platform_publication", ROOT.parent / "teambition/publish.py")
platform = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform)


def publish(base, token, source, archive, normalized_sha256):
    listing = platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
    items = [item for item in listing if item["revision"]["source"] == source]
    target = next((item["revision"] for item in items if item["revision"]["sha256"] == normalized_sha256), None)
    if target is None:
        target = platform.api(base, token, "POST", "/api/v1/admin/connectors/packages",
                              {"archive": base64.b64encode(archive).decode()})
    if target["source"] != source or target["sha256"] != normalized_sha256 or target["mode"] != "mcp":
        raise RuntimeError("staged revision differs from parser-verified package")
    current = next((item["publication"] for item in items if item.get("publication")), None)
    if not current or current["active_revision_id"] != target["id"] or current["state"] != "available":
        current = platform.api(base, token, "POST", "/api/v1/admin/connectors/publications/"
                               + parse.quote(target["id"]) + "/publish",
                               {"expected_version": current["version"] if current else 0})
    installation = platform.api(base, token, "POST", "/api/v1/connectors/catalog/" + source + "/install", {})
    if installation["active_revision_id"] != target["id"]:
        installation = platform.api(base, token, "POST", "/api/v1/connectors/" + parse.quote(installation["id"])
                                    + "/upgrade", {"expected_version": installation["version"]})
    user = [item for item in platform.api(base, token, "GET", "/api/v1/connectors/catalog").get("items", [])
            if item["source"] == source]
    admin = [item for item in platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
             if (item.get("publication") or {}).get("source") == source]
    installed = [item for item in platform.api(base, token, "GET", "/api/v1/connectors").get("items", [])
                 if item["source"] == source]
    if len(user) != 1 or len(admin) != 1 or len(installed) != 1:
        raise RuntimeError("catalogs must each contain exactly one official entry")
    if current["state"] != "available" or user[0]["active_revision_id"] != target["id"] or installation["state"] != "active":
        raise RuntimeError("publication or installation did not activate")
    if not user[0]["revision"]["icon"].startswith("data:image/png;base64,"):
        raise RuntimeError("official brand projection is not deployed")
    expected_driver = "connector_package" if source == "pkulaw" else "none"
    if installation["authentication_driver"] != expected_driver or source == "mindbye" and not installation.get("authorized"):
        raise RuntimeError("installation authorization policy differs from reviewed service")
    return {"revision": target, "publication": current, "installation": installation,
            "user_catalog": user, "administrator_catalog": admin}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--package-directory", type=Path, required=True)
    parser.add_argument("--evidence-directory", type=Path, required=True)
    args = parser.parse_args()
    config = platform.read_config(args.config)
    base = config["VITE_OIDC_AUTHORITY"].split("/identity/realms/")[0]
    origin = parse.urlparse(base)
    if origin.scheme != "https" or not origin.netloc or origin.username or origin.path or origin.query or origin.fragment:
        raise RuntimeError("platform requires an HTTPS origin")
    token = platform.administrator_token(config)
    args.evidence_directory.mkdir(parents=True, exist_ok=True)
    for source in build.SOURCES:
        path = args.package_directory / f"{source}-1.0.0.zip"
        archive = path.read_bytes()
        receipt = json.loads(path.with_suffix(".receipt.json").read_text())
        if archive != build.build(source) or hashlib.sha256(archive).hexdigest() != receipt["archive_sha256"]:
            raise RuntimeError("package differs from reviewed source or validation receipt")
        outcome = publish(base, token, source, archive, receipt["normalized_sha256"])
        (args.evidence_directory / f"{source}.json").write_text(json.dumps(outcome, ensure_ascii=False, indent=2) + "\n")
        print(json.dumps({"source": source, "revision_id": outcome["revision"]["id"],
                          "installation_id": outcome["installation"]["id"],
                          "publication": "available", "installation": "active",
                          "authorized": outcome["installation"].get("authorized", False)}))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("publication_failed", str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__)
        raise SystemExit(1)
