import copy
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location("haokuaiji_build", Path(__file__).with_name("build.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


def fixture():
    # Synthetic configuration; never sent upstream or delivered as a real package.
    return {"mcpServers": {"HKJ_TENANT_MCP": {
        "type": "streamable-http", "url": "https://mcphub.chanapp.chanjet.com/999999/mcp",
        "headers": {"Authorization": "Bearer local-credential-canary"}}}}


class BuildTests(unittest.TestCase):
    def test_credentials_are_removed_from_a_deterministic_mcp_only_zip(self):
        config = fixture()
        original = copy.deepcopy(config)
        package = build.build(config)
        self.assertEqual(package, build.build(config))
        self.assertEqual(config, original)
        with zipfile.ZipFile(io.BytesIO(package)) as archive:
            self.assertEqual(set(archive.namelist()), {
                "connector-meta.json", "icon.svg", "mcp.json", "skills/haokuaiji/SKILL.md"})
            for name in archive.namelist():
                self.assertNotIn(b"local-credential-canary", archive.read(name))
                self.assertNotIn(b"Bearer ", archive.read(name))
            manifest = json.loads(archive.read("mcp.json"))
            self.assertNotIn("headers", manifest)
            self.assertEqual(manifest["egress_hosts"], [build.HOST])
            self.assertEqual(json.loads(archive.read("connector-meta.json"))["auth_mode"], "cli")

    def test_final_zip_is_checked_by_current_go_parse(self):
        # Validates a real archive of synthetic config, not upstream connectivity.
        self.assertIn("source=haokuaiji version=0.1.0", build.validate_package(build.build(fixture())))

    def test_rejects_unsafe_and_unrecognized_endpoints(self):
        for address in ["http://" + build.HOST + "/2/mcp", "https://localhost/2/mcp",
                        "https://" + build.HOST + ".evil.example/2/mcp",
                        "https://user:secret@" + build.HOST + "/2/mcp",
                        "https://" + build.HOST + ":443/2/mcp",
                        "https://" + build.HOST + "/2/mcp?apiKey=url-secret-canary",
                        "https://" + build.HOST + "/2/mcp#url-secret-canary",
                        "https://" + build.HOST + "/2/%2e%2e/mcp",
                        "https://" + build.HOST + "/2/mcp\n", "https://[invalid", None]:
            with self.subTest(address=address):
                config = fixture()
                config["mcpServers"]["HKJ_TENANT_MCP"]["url"] = address
                with self.assertRaises(ValueError) as error:
                    build.build(config)
                self.assertNotIn("url-secret-canary", str(error.exception))

    def test_rejects_unsupported_auth_and_routing_headers(self):
        for headers in [None, {}, {"X-API-Key": "secret-canary"},
                        {"Authorization": "Basic secret-canary"},
                        {"Authorization": "Bearer secret-canary\r\nX-Header: injected"},
                        {"Authorization": "Bearer secret-canary", "X-Org-Id": "42"},
                        {"Authorization": "Bearer secret-canary", "authorization": "Bearer other"},
                        {"Authorization": "Bearer secret-canary", "Content-Type": "text/plain"}]:
            with self.subTest(headers=headers):
                config = fixture()
                config["mcpServers"]["HKJ_TENANT_MCP"]["headers"] = headers
                with self.assertRaises(ValueError) as error:
                    build.build(config)
                self.assertNotIn("secret-canary", str(error.exception))

    def test_does_not_ignore_unknown_fields_or_local_command_configuration(self):
        for key, value in [("command", "uvx"), ("args", []), ("env", {}), ("disabled", True),
                           ("transport", "sse"), ("type", "stdio"), ("type", {}), ("transport", [])]:
            config = fixture()
            config["mcpServers"]["HKJ_TENANT_MCP"][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                build.build(config)

    def test_requires_unambiguous_haokuaiji_product_selection(self):
        server = fixture()["mcpServers"]["HKJ_TENANT_MCP"]
        for name in ["HSY-MCP", "HYC-MCP", "TPlusAPI", "ydz"]:
            config = {"mcpServers": {name: server}}
            with self.subTest(name=name), self.assertRaises(ValueError):
                build.build(config, name)
        config = {"mcpServers": {"HKJ-MCP": server, "好会计": server}}
        with self.assertRaises(ValueError):
            build.build(config)
        self.assertTrue(build.build(config, "好会计"))

    def test_rejects_duplicate_json_keys_and_non_regular_config_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / "private.json"
            path.write_text('{"mcpServers":{},"mcpServers":{}}')
            with self.assertRaises(ValueError):
                build.read_config(path)
            path.write_text('{"credential": "broken-secret-canary}')
            with self.assertRaises(ValueError) as error:
                build.read_config(path)
            self.assertNotIn("broken-secret-canary", str(error.exception))
            path.write_bytes(b" " * (64 * 1024 + 1))
            with self.assertRaises(ValueError):
                build.read_config(path)
            link = root / "link.json"
            link.symlink_to(path)
            with self.assertRaises(ValueError):
                build.read_config(link)

    def test_brand_asset_drift_is_rejected(self):
        with patch.object(build, "ICON_SHA256", "0" * 64), self.assertRaises(ValueError):
            build.build(fixture())

    def test_cli_failure_never_overwrites_input_or_leaks_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "private.json"
            config.write_text(json.dumps(fixture()))
            original = config.read_bytes()
            result = subprocess.run(["python3", str(build.ROOT / "build.py"), "--config", str(config),
                                     "--output", str(config)], capture_output=True, text=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(config.read_bytes(), original)
            self.assertNotIn("local-credential-canary", result.stdout + result.stderr)
            config.write_text('{"token":"bad-secret-canary}')
            output = root / "package.zip"
            result = subprocess.run(["python3", str(build.ROOT / "build.py"), "--config", str(config),
                                     "--output", str(output)], capture_output=True, text=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())
            self.assertNotIn("bad-secret-canary", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
