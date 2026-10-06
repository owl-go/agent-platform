import importlib.util
import json
import os
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("registration", Path(__file__).with_name("configure-registration-identity.py"))
registration = importlib.util.module_from_spec(spec)
spec.loader.exec_module(registration)


class RegistrationConfigurationTest(unittest.TestCase):
    def test_preserves_profile_requirements_and_grants_only_narrow_roles(self):
        original = {"attributes": [{"name": "email", "required": {"roles": ["user"]}, "validations": {"email": {}}}, {"name": "firstName", "required": {"roles": ["user"]}}], "unmanagedAttributePolicy": "DISABLED", "groups": [{"name": "personal"}]}
        profile = json.loads(json.dumps(original))
        grants = []
        changes = []
        def request(url, method="GET", body=None, token=None, content_type=None):
            nonlocal profile
            if url.endswith("/token"):
                return {"access_token": "fixture"}
            if url.endswith("/users/profile"):
                if method == "PUT":
                    profile = json.loads(body)
                    changes.append("profile")
                    return None
                return profile
            if url.endswith("/clients?clientId=service"):
                return [{"id": "service-id", "clientId": "service", "serviceAccountsEnabled": True}]
            if url.endswith("/clients?clientId=realm-management"):
                return [{"id": "management-id"}]
            if url.endswith("/service-account-user"):
                return {"id": "service-user"}
            if url.endswith("/role-mappings/clients/management-id"):
                if method == "POST":
                    grants.extend(json.loads(body))
                    changes.append("roles")
                    return None
                return grants
            if url.endswith("/roles"):
                return [{"id": name, "name": name} for name in registration.ROLES + ("manage-realm",)]
            self.fail("Unexpected identity request")
        environment = {"VITE_OIDC_AUTHORITY": "https://identity.test/realms/workspace", "KEYCLOAK_ADMIN_USER": "admin", "KEYCLOAK_ADMIN_PASSWORD": "fixture", "KEYCLOAK_SERVICE_CLIENT_ID": "service"}
        with patch.dict(os.environ, environment), patch.object(registration.identity, "request", request):
            registration.configure("apply")
            registration.configure("check")
            registration.configure("apply")
        self.assertEqual(changes, ["profile", "roles"])
        self.assertNotIn("required", profile["attributes"][0])
        self.assertEqual(profile["attributes"][0]["validations"], original["attributes"][0]["validations"])
        self.assertEqual(profile["attributes"][1], original["attributes"][1])
        self.assertEqual(profile["unmanagedAttributePolicy"], "DISABLED")
        self.assertEqual(profile["groups"], original["groups"])
        self.assertNotIn("manage-realm", {item["name"] for item in grants})
        for attribute in profile["attributes"][2:]:
            self.assertEqual(attribute["permissions"], {"view": ["admin"], "edit": ["admin"]})


if __name__ == "__main__":
    unittest.main()
