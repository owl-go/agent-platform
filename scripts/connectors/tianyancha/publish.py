#!/usr/bin/env python3
"""Publish the exact reviewed Tianyancha MCP package and verify the User catalog."""
import argparse
import base64
import importlib.util
import json
from pathlib import Path
import re
from urllib import error, parse, request
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("platform_publication", ROOT.parent / "teambition/publish.py")
platform = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform)

class NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--allow-region-blocked", action="store_true", help="Publish a disclosed region-blocked catalog entry; does not claim OAuth passed")
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--package", type=Path, required=True)
    parser.add_argument("--normalized-sha256", required=True, help="SHA-256 returned by the current connectorpackage.Parse")
    parser.add_argument("--evidence-directory", type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch("[a-f0-9]{64}", args.normalized_sha256):
        parser.error("normalized SHA-256 must use 64 lowercase hex digits")
    config = platform.read_config(args.config)
    base = config["VITE_OIDC_AUTHORITY"].split("/identity/realms/")[0]
    origin = parse.urlparse(base)
    if origin.scheme != "https" or not origin.netloc or origin.username or origin.path or origin.query or origin.fragment:
        raise RuntimeError("platform requires an HTTPS origin")
    archive = args.package.read_bytes()
    with zipfile.ZipFile(args.package) as package:
        meta = json.loads(package.read("connector-meta.json"))
        mcp = json.loads(package.read("mcp.json"))
    if meta["source"] != "tianyancha" or meta["version"] != "1.0.0" or meta["type"] != "mcp" or meta["auth_mode"] != "oauth" or mcp != {
        "transport":"streamable_http", "url":"https://mcp.tianyancha.com/mcp", "egress_hosts":["mcp.tianyancha.com"], "timeout_seconds":60
    }:
        raise RuntimeError("package differs from reviewed Tianyancha policy")
    redirect = base + "/api/v1/connectors/tianyancha/oauth/callback"
    opener = request.build_opener(NoRedirect())
    try:
        opener.open(redirect, timeout=20).close()
        raise RuntimeError("unexpected callback response")
    except error.HTTPError as exc:
        if exc.code != 400 or exc.headers.get("Cache-Control") != "no-store" or exc.headers.get("Referrer-Policy") != "no-referrer":
            raise RuntimeError("Tianyancha browser OAuth adapter is not deployed") from None
    registration = {"client_name":"Agent Workspace Tianyancha", "redirect_uris":[redirect], "grant_types":["authorization_code","refresh_token"], "response_types":["code"], "token_endpoint_auth_method":"none"}
    callback_registration = "passed"
    try:
        with opener.open(request.Request("https://capi.tianyancha.com/oauth/register", data=json.dumps(registration).encode(), headers={"Content-Type":"application/json","Accept":"application/json"}), timeout=20) as response:
            registered = json.load(response)
    except error.HTTPError as exc:
        blocked = json.loads(exc.read(4096)) if exc.code == 419 else {}
        if not args.allow_region_blocked or blocked.get("errorCode") != 301000 or blocked.get("message") != "bannedLocation" or "授权暂受阻" not in meta["description"]:
            raise RuntimeError(f"Tianyancha rejected the actual platform callback (HTTP {exc.code})") from None
        registered = None
        callback_registration = "blocked_region"
    if registered is not None and (not registered.get("client_id") or registered.get("redirect_uris") != [redirect] or registered.get("token_endpoint_auth_method") != "none"):
        raise RuntimeError("Tianyancha public client registration mismatch")
    token = platform.administrator_token(config)
    listing = platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
    items = [item for item in listing if item["revision"]["source"] == "tianyancha"]
    target = next((item["revision"] for item in items if item["revision"]["sha256"] == args.normalized_sha256 and item["revision"]["package_version"] == meta["version"]), None)
    if target is None:
        target = platform.api(base, token, "POST", "/api/v1/admin/connectors/packages", {"archive":base64.b64encode(archive).decode()})
    if target["sha256"] != args.normalized_sha256 or target["source"] != "tianyancha" or target["mode"] != "mcp":
        raise RuntimeError("staged revision differs from verified package")
    current = next((item["publication"] for item in items if item.get("publication")), None)
    if not current or current["active_revision_id"] != target["id"] or current["state"] != "available":
        current = platform.api(base, token, "POST", "/api/v1/admin/connectors/publications/" + parse.quote(target["id"]) + "/publish", {"expected_version":current["version"] if current else 0})
    if current["active_revision_id"] != target["id"] or current["state"] != "available":
        raise RuntimeError("publication did not activate expected revision")
    catalog = platform.api(base, token, "GET", "/api/v1/connectors/catalog").get("items", [])
    matches = [item for item in catalog if item["source"] == "tianyancha"]
    if len(matches) != 1 or matches[0]["active_revision_id"] != target["id"] or not matches[0]["revision"]["icon"].startswith("data:image/png;base64,"):
        raise RuntimeError("User catalog or brand image verification failed")
    admin = platform.api(base, token, "GET", "/api/v1/admin/connectors/publications").get("items", [])
    active = [item for item in admin if (item.get("publication") or {}).get("source") == "tianyancha"]
    if len(active) != 1:
        raise RuntimeError("administrator catalog has duplicate publications")
    args.evidence_directory.mkdir(parents=True, exist_ok=True)
    for filename, value in [("stage-response.json",target),("publication-response.json",current),("user-catalog.json",matches),("administrator-catalog.json",active)]:
        (args.evidence_directory / filename).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"source":"tianyancha", "revision_id":target["id"], "sha256":target["sha256"], "state":"available", "callback_registration":callback_registration, "account_verification":"not_run"}))

if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("publication_failed", str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__)
        raise SystemExit(1)
