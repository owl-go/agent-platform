#!/usr/bin/env python3
"""Back up, apply, or restore only the existing realm's appearance settings.

Credentials are read from process environment and sent in an HTTPS request body.
No users, clients, sessions, authentication policies, or realm imports are changed.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import Request

APPEARANCE = {
    "loginTheme": "agent-workspace",
    "internationalizationEnabled": True,
    "supportedLocales": ["zh-Hans", "en"],
    "defaultLocale": "zh-Hans",
}


def endpoints(authority: str) -> tuple[str, str]:
    parsed = urlsplit(authority)
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username
            or parsed.password or parsed.query or parsed.fragment):
        raise ValueError("Identity authority must be an HTTPS issuer without credentials or query")
    prefix, separator, realm = authority.rstrip("/").rpartition("/realms/")
    if not separator or not realm or "/" in realm or not all(c.isalnum() or c in "_-" for c in realm):
        raise ValueError("Identity authority must identify one realm")
    return prefix + "/realms/master/protocol/openid-connect/token", prefix + "/admin/realms/" + realm


def request(url: str, method: str = "GET", body: bytes | None = None,
            token: str | None = None, content_type: str = "application/json"):
    headers = {"Content-Type": content_type}
    if token:
        headers["Authorization"] = "Bearer " + token
    # Do not follow a redirect carrying the administrative Authorization header.
    from urllib.request import build_opener, HTTPRedirectHandler
    class NoRedirect(HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    with build_opener(NoRedirect).open(Request(url, body, headers, method=method), timeout=30) as response:
        data = response.read()
        return json.loads(data) if data else None


def configure(mode: str, backup: Path | None) -> None:
    token_url, realm_url = endpoints(os.environ["VITE_OIDC_AUTHORITY"])
    credentials = urlencode({
        "grant_type": "password", "client_id": "admin-cli",
        "username": os.environ["KEYCLOAK_ADMIN_USER"],
        "password": os.environ["KEYCLOAK_ADMIN_PASSWORD"],
    }).encode()
    token = request(token_url, "POST", credentials, content_type="application/x-www-form-urlencoded")["access_token"]
    realm = request(realm_url, token=token)
    if mode == "backup":
        if backup is None:
            raise ValueError("backup requires a private output path")
        fd = os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as output:
            json.dump({key: realm.get(key) for key in APPEARANCE}, output, indent=2)
            output.write("\n")
        return
    desired = APPEARANCE if mode == "apply" else json.loads(backup.read_text())
    if set(desired) != set(APPEARANCE):
        raise ValueError("Restore file must contain only appearance settings")
    request(realm_url, "PUT", json.dumps(desired).encode(), token)
    actual = request(realm_url, token=token)
    if any((set(actual.get(key) or []) != set(value or []) if key == "supportedLocales"
            else actual.get(key) != value) for key, value in desired.items()):
        raise ValueError("Identity appearance verification failed")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["backup", "apply", "restore"])
    parser.add_argument("backup", nargs="?", type=Path)
    args = parser.parse_args()
    if args.mode in ("backup", "restore") and args.backup is None:
        parser.error("backup and restore require a backup path")
    try:
        configure(args.mode, args.backup)
    except HTTPError as error:
        sys.exit("Identity configuration failed: HTTP " + str(error.code))
    except (URLError, ValueError, KeyError, OSError):
        sys.exit("Identity configuration failed; check protected configuration and connectivity")
    print("Identity appearance " + args.mode + " verified")
