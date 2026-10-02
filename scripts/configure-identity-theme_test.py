import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("theme", Path(__file__).with_name("configure-identity-theme.py"))
theme = importlib.util.module_from_spec(spec)
spec.loader.exec_module(theme)


class AppearanceConfigurationTests(unittest.TestCase):
    def test_rejects_non_https_or_ambiguous_issuers(self):
        for authority in ("http://identity.test/realms/test", "https://user:secret@identity.test/realms/test",
                          "https://identity.test/realms/test?redirect=elsewhere", "https://identity.test/realms/test/other"):
            with self.subTest(authority=authority), self.assertRaises(ValueError):
                theme.endpoints(authority)

    def test_backup_apply_and_restore_preserve_unrelated_realm_settings(self):
        original = {"loginTheme": "keycloak", "internationalizationEnabled": False,
                    "supportedLocales": ["en"], "defaultLocale": "en"}
        realm = {**original, "enabled": True, "users": [{"private": "account"}], "ssoSessionMaxLifespan": 259200}
        updates = []

        def transport(url, method="GET", body=None, token=None, content_type="application/json"):
            if url.endswith("/token"):
                self.assertIsNone(token)
                self.assertIn(b"password=fixture", body)
                return {"access_token": "private-token"}
            self.assertEqual(token, "private-token")
            if method == "PUT":
                update = json.loads(body)
                self.assertEqual(set(update), set(theme.APPEARANCE))
                updates.append(update)
                realm.update(update)
                return None
            result = realm.copy()
            result["supportedLocales"] = list(reversed(result["supportedLocales"]))
            return result

        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {
            "VITE_OIDC_AUTHORITY": "https://identity.test/identity/realms/test",
            "KEYCLOAK_ADMIN_USER": "fixture-admin", "KEYCLOAK_ADMIN_PASSWORD": "fixture",
        }), patch.object(theme, "request", side_effect=transport):
            backup = Path(directory) / "appearance.json"
            theme.configure("backup", backup)
            self.assertEqual(backup.stat().st_mode & 0o777, 0o600)
            self.assertEqual(json.loads(backup.read_text()), original)
            with self.assertRaises(FileExistsError):
                theme.configure("backup", backup)
            theme.configure("apply", None)
            theme.configure("restore", backup)
        self.assertEqual(updates, [theme.APPEARANCE, original])
        self.assertEqual(realm["ssoSessionMaxLifespan"], 259200)
        self.assertTrue(realm["enabled"])


if __name__ == "__main__":
    unittest.main()
