import importlib.util
import unittest
from pathlib import Path


spec = importlib.util.spec_from_file_location("dingtalk_build", Path(__file__).with_name("build.py"))
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


class DingTalkBuilderTest(unittest.TestCase):
    def schema(self, tools):
        return {
            "tool_count": 1435,
            "catalog_hash": "sha256:857592291504d44aae4c4d94f9dad462dc472bafbd8d8235ffa3b68bd4433bd5",
            "products": [{"tools": tools + [{"availability": "unavailable"}] * (1435 - len(tools))}],
        }

    def tool(self, path, effect="read", risk="low", confirmation="not_required"):
        return {"availability": "available", "cli_path": path, "effect": effect, "risk": risk, "confirmation": confirmation}

    def test_all_scope_keeps_management_and_requires_approval_for_writes(self):
        schema = self.schema([
            self.tool("doc search"),
            self.tool("doc create", effect="write", risk="medium"),
            self.tool("pat token revoke", effect="destructive", risk="high", confirmation="user_required"),
        ])
        capabilities = builder.reviewed_capabilities(schema, include_admin=True)
        by_path = {tuple(item["argv_prefix"]): item for item in capabilities}
        self.assertEqual(by_path[("doc", "search")]["risk"], "low")
        self.assertEqual(by_path[("doc", "create")]["risk"], "high")
        self.assertEqual(by_path[("pat", "token", "revoke")]["risk"], "high")
        self.assertEqual(by_path[("schema",)]["risk"], "low")

    def test_business_scope_omits_management(self):
        schema = self.schema([self.tool("doc search"), self.tool("pat token revoke", effect="destructive")])
        paths = {tuple(item["argv_prefix"]) for item in builder.reviewed_capabilities(schema, include_admin=False)}
        self.assertIn(("doc", "search"), paths)
        self.assertNotIn(("pat", "token", "revoke"), paths)

    def test_managed_package_omits_local_profile_auth_diagnostics(self):
        capabilities = builder.reviewed_capabilities(self.schema([self.tool("chat create")]), include_admin=False)
        paths = {tuple(item["argv_prefix"]) for item in capabilities}
        self.assertNotIn(("auth", "status"), paths)
        self.assertNotIn(("profile", "list"), paths)

    def test_overlapping_command_cannot_weaken_policy(self):
        schema = self.schema([self.tool("doc read"), self.tool("doc read hidden", effect="write")])
        with self.assertRaisesRegex(ValueError, "overlapping"):
            builder.reviewed_capabilities(schema, include_admin=True)

    def test_pinned_npm_integrity_is_required(self):
        with self.assertRaisesRegex(ValueError, "integrity"):
            builder.npm_assets(b"untrusted package")


if __name__ == "__main__":
    unittest.main()
