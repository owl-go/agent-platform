import hashlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import zipfile

import build
import publish


class PackageTests(unittest.TestCase):
    def test_deterministic_mcp_only_archive_and_actual_parse(self):
        data = build.build()
        self.assertEqual(data, build.build())
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            self.assertEqual(set(archive.namelist()), {"connector-meta.json", "icon.svg", "mcp.json", "skills/xiaoe/SKILL.md"})
            self.assertEqual(archive.read("icon.svg"), (build.ROOT.parents[2] / "frontend/src/assets/xiaoe.svg").read_bytes())
            self.assertEqual(json.loads(archive.read("mcp.json"))["egress_hosts"], ["agent.xiaoe-tech.com"])
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "xiaoe.zip"
            path.write_bytes(data)
            result = subprocess.run(["go", "-C", str(build.ROOT.parents[2] / "backend"), "run", "./cmd/connector-package-validate", str(path)], capture_output=True, text=True, check=True)
            self.assertIn("source=xiaoe version=0.1.0", result.stdout)
            self.assertIn("skills=1 capabilities=0", result.stdout)

    def test_publication_reuses_exact_revision_and_creates_no_staging_definition(self):
        revision = {"id": "revision", "source": "xiaoe", "sha256": "a" * 64, "icon": "xiaoe"}
        publication = {"source": "xiaoe", "active_revision_id": "revision", "state": "available", "version": 1}
        calls = []

        def api(method, path, body=None):
            calls.append((method, path))
            if path == "/api/v1/admin/connectors/publications":
                return {"items": [{"revision": revision, "publication": publication}]}
            if path == "/api/v1/connectors/publications":
                return {"items": [{"source": "xiaoe", "revision": revision}]}
            self.fail("unexpected platform mutation")

        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "package.zip"
            path.write_bytes(build.build())
            result = publish.publish(api, path, "a" * 64)
        self.assertEqual(result["publication"]["state"], "available")
        self.assertEqual(result["business_api_verification"], "not_run")
        self.assertTrue(all(method == "GET" for method, _ in calls))

    def test_first_publication_stages_then_activates_with_catalog_verification(self):
        revision = {"id": "revision", "source": "xiaoe", "sha256": "a" * 64, "icon": "xiaoe"}
        calls = []

        def api(method, path, body=None):
            calls.append((method, path, body))
            if path == "/api/v1/admin/connectors/publications": return {"items": []}
            if path == "/api/v1/admin/connectors/packages": return revision
            if path.endswith("/revision/publish"):
                self.assertEqual(body, {"expected_version": 0})
                return {"state": "available", "active_revision_id": "revision"}
            if path == "/api/v1/connectors/publications": return {"items": [{"source": "xiaoe", "revision": revision}]}
            self.fail("unexpected API")

        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "package.zip"
            path.write_bytes(build.build())
            publish.publish(api, path, "a" * 64)
        self.assertEqual([path for method, path, _ in calls if method == "POST"], ["/api/v1/admin/connectors/packages", "/api/v1/admin/connectors/publications/revision/publish"])


if __name__ == "__main__":
    unittest.main()
