import io
import json
import unittest
import zipfile

import build
import publish


class PackageTest(unittest.TestCase):
    def test_reviewed_service_boundaries(self):
        for source in build.SOURCES:
            with self.subTest(source=source):
                archive = build.build(source)
                self.assertEqual(archive, build.build(source))
                with zipfile.ZipFile(io.BytesIO(archive)) as package:
                    self.assertEqual(len(package.namelist()), 4)
                    self.assertNotIn("cli.json", package.namelist())
                    meta = json.loads(package.read("connector-meta.json"))
                    mcp = json.loads(package.read("mcp.json"))
                    self.assertEqual(meta["source"], source)
                    self.assertTrue(mcp["url"].startswith("https://"))
                    self.assertEqual(meta["auth_mode"], "oauth" if source == "pkulaw" else "none")
                    skill = package.read(f"skills/{source}/SKILL.md").decode()
                    if source == "pkulaw":
                        self.assertEqual(mcp["headers"], {"Authorization": "${MCP_BEARER_TOKEN}"})
                        self.assertEqual(mcp["url"], "https://apim-gateway.pkulaw.com/mcp-law-search-service")
                    else:
                        self.assertNotIn("headers", mcp)
                        self.assertIn("contract-review-mcp.oss-cn-hangzhou.aliyuncs.com", mcp["egress_hosts"])
                        self.assertIn("contract_review_submit", skill)

    def test_publication_reuses_exact_revision(self):
        for source in build.SOURCES:
            with self.subTest(source=source):
                revision = {"id": source + "-revision", "source": source, "sha256": "a" * 64, "mode": "mcp", "icon": "data:image/png;base64,asset"}
                publication = {"source": source, "active_revision_id": revision["id"], "state": "available", "version": 1, "revision": revision}
                installation = {"id": source + "-installation", "source": source, "active_revision_id": revision["id"], "state": "active", "authentication_driver": "connector_package" if source == "pkulaw" else "none", "authorized": source == "mindbye"}
                calls = []

                def api(base, token, method, path, body=None):
                    calls.append((method, path))
                    if path == "/api/v1/admin/connectors/publications":
                        return {"items": [{"revision": revision, "publication": publication}]}
                    if path.endswith("/install"):
                        return installation
                    if path == "/api/v1/connectors/catalog":
                        return {"items": [publication]}
                    if path == "/api/v1/connectors":
                        return {"items": [installation]}
                    raise AssertionError("unexpected mutation: " + path)

                original = publish.platform.api
                publish.platform.api = api
                try:
                    outcome = publish.publish("https://platform.example.test", "fixture", source, b"zip", "a" * 64)
                    self.assertEqual(outcome["revision"], revision)
                    self.assertEqual([p for m, p in calls if m == "POST"], [f"/api/v1/connectors/catalog/{source}/install"])
                finally:
                    publish.platform.api = original


if __name__ == "__main__":
    unittest.main()
