#!/usr/bin/env python3
"""Declare admin-only scan identity attributes and narrow service-account roles.

Run once against an existing realm before enabling scan registration. Uses the
same protected environment as configure-identity-theme.py. Does not change
password registration, existing users, or sessions. Email becomes optional
for scan identities; manual account creation still requires it in the product.
"""
from __future__ import annotations
import argparse
import importlib.util
import os
from pathlib import Path
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, quote

spec = importlib.util.spec_from_file_location("identity_theme", Path(__file__).with_name("configure-identity-theme.py"))
identity = importlib.util.module_from_spec(spec)
spec.loader.exec_module(identity)
ATTRIBUTES = ("aw_registration_subject", "aw_registration_provider")
ROLES = ("manage-users", "view-users", "manage-identity-providers")


def desired_profile(profile):
    result = dict(profile)
    attributes = [dict(item) for item in profile.get("attributes", [])]
    for attribute in attributes:
        if attribute.get("name") == "email":
            attribute.pop("required", None)
    for name in ATTRIBUTES:
        found = next((item for item in attributes if item.get("name") == name), None)
        if found is None:
            found = {"name": name, "displayName": name, "multivalued": False}
            attributes.append(found)
        found["permissions"] = {"view": ["admin"], "edit": ["admin"]}
    result["attributes"] = attributes
    return result


def configure(mode):
    token_url, realm_url = identity.endpoints(os.environ["VITE_OIDC_AUTHORITY"])
    credentials = urlencode({"grant_type": "password", "client_id": "admin-cli", "username": os.environ["KEYCLOAK_ADMIN_USER"], "password": os.environ["KEYCLOAK_ADMIN_PASSWORD"]}).encode()
    token = identity.request(token_url, "POST", credentials, content_type="application/x-www-form-urlencoded")["access_token"]
    request = lambda path, method="GET", body=None: identity.request(realm_url + path, method, body, token)
    import json
    profile = request("/users/profile")
    desired = desired_profile(profile)
    client_name = os.environ["KEYCLOAK_SERVICE_CLIENT_ID"]
    clients = request("/clients?clientId=" + quote(client_name, safe=""))
    if len(clients) != 1 or clients[0].get("clientId") != client_name or not clients[0].get("serviceAccountsEnabled"):
        raise ValueError("Expected one existing service-account client")
    admin_clients = request("/clients?clientId=realm-management")
    if len(admin_clients) != 1:
        raise ValueError("Realm management client missing")
    management_id = admin_clients[0]["id"]
    account = request("/clients/" + clients[0]["id"] + "/service-account-user")
    role_path = "/users/" + account["id"] + "/role-mappings/clients/" + management_id
    current = request(role_path)
    missing = [role for role in ROLES if role not in {item["name"] for item in current}]
    if mode == "apply":
        if desired != profile:
            request("/users/profile", "PUT", json.dumps(desired).encode())
        if missing:
            all_roles = request("/clients/" + management_id + "/roles")
            grants = [item for item in all_roles if item["name"] in missing]
            if len(grants) != len(missing):
                raise ValueError("Required narrow roles missing")
            request(role_path, "POST", json.dumps(grants).encode())
    actual = request("/users/profile")
    if desired_profile(actual) != actual:
        raise ValueError("Scan identity markers must be admin-only")
    if not set(ROLES).issubset({item["name"] for item in request(role_path)}):
        raise ValueError("Required service-account roles missing")
    print("Registration identity profile and narrow service roles verified")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["check", "apply"])
    args = parser.parse_args()
    try:
        configure(args.mode)
    except HTTPError as error:
        sys.exit("Registration identity configuration failed: HTTP " + str(error.code))
    except (URLError, ValueError, KeyError, OSError):
        sys.exit("Registration identity configuration failed; check protected configuration and connectivity")
