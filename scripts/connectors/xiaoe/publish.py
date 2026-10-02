#!/usr/bin/env python3
"""Stage and publish the reviewed Xiaoe MCP package, preserving existing installations."""
import argparse
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import zipfile

spec = importlib.util.spec_from_file_location("platform_publish", Path(__file__).resolve().parents[1] / "teambition/publish.py")
platform = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform)


def publish(api, archive, normalized_sha):
    with zipfile.ZipFile(archive) as package:
        meta = json.loads(package.read("connector-meta.json"))
        mcp = json.loads(package.read("mcp.json"))
    if meta["source"] != "xiaoe" or meta["type"] != "mcp" or meta["auth_mode"] != "oauth" or mcp["url"] != "https://agent.xiaoe-tech.com/mcp":
        raise RuntimeError("unexpected package identity")
    listing = api("GET", "/api/v1/admin/connectors/publications").get("items", [])
    items = [v for v in listing if v["revision"]["source"] == "xiaoe"]
    current = next((v["publication"] for v in items if v.get("publication")), None)
    sha = normalized_sha
    target = next((v["revision"] for v in items if v["revision"]["sha256"] == sha), None)
    if target is None:
        target = api("POST", "/api/v1/admin/connectors/packages", {"archive": base64.b64encode(archive.read_bytes()).decode()})
    if target["sha256"] != sha: raise RuntimeError("staged package differs from parsed package")
    if not current or current["active_revision_id"] != target["id"] or current["state"] != "available":
        current = api("POST", "/api/v1/admin/connectors/publications/" + target["id"] + "/publish", {"expected_version": current["version"] if current else 0})
    if current["state"] != "available" or current["active_revision_id"] != target["id"]:
        raise RuntimeError("publication did not activate expected revision")
    catalog = api("GET", "/api/v1/connectors/publications").get("items", [])
    matches = [v for v in catalog if v["source"] == "xiaoe"]
    if len(matches) != 1 or matches[0]["revision"]["id"] != target["id"] or matches[0]["revision"]["icon"] != "xiaoe":
        raise RuntimeError("User catalog or brand projection differs from publication")
    return {"publication": current, "revision": target, "user_catalog": matches,
            "account_authorization": "blocked_by_upstream_callback_domain_policy", "business_api_verification": "not_run"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--package", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    config = platform.read_config(args.config)
    token = platform.administrator_token(config)
    origin = config["VITE_OIDC_AUTHORITY"].split("/identity/realms/")[0]
    receipt = json.loads(args.package.with_suffix(".receipt.json").read_text())
    if hashlib.sha256(args.package.read_bytes()).hexdigest() != receipt["archive_sha256"]: raise RuntimeError("archive changed after Parse")
    result = publish(lambda method, path, body=None: platform.api(origin, token, method, path, body), args.package, receipt["normalized_sha256"])
    args.evidence.parent.mkdir(parents=True, exist_ok=True)
    args.evidence.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"source": "xiaoe", "revision_id": result["revision"]["id"], "state": result["publication"]["state"], "authorization": result["account_authorization"]}))


if __name__ == "__main__":
    main()
