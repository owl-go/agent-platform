import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location("openboost_build", Path(__file__).with_name("build.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)
IMAGE = "public.ecr.aws/docker/library/node@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d"


class BuildTests(unittest.TestCase):
    def test_deterministic_real_bundle_and_frozen_risk_boundary(self):
        data, bundle_sha, source = build.build(IMAGE, build.RUNTIME_VERSION)
        self.assertEqual((data, bundle_sha, source), build.build(IMAGE, build.RUNTIME_VERSION))
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            self.assertEqual(set(archive.namelist()), {"connector-meta.json", "cli.json", "icon.svg", "cli-bundle.tgz", "skills/openboost/SKILL.md", "skills/openboost/capabilities.json", "skills/openboost/tools.json"})
            for info in archive.infolist():
                self.assertEqual(info.external_attr >> 16 & 0o170000, 0o100000)
            manifest = json.loads(archive.read("cli.json"))
            self.assertEqual(manifest["authentication_driver"], "connector_package")
            self.assertEqual(manifest["egress_hosts"], ["mcp.microdata-inc.com"])
            capabilities = manifest["capabilities"]
            self.assertEqual(len(capabilities), 103)
            high = {tuple(value["argv_prefix"]) for value in capabilities if value["risk"] == "high"}
            self.assertEqual(high, set())
            self.assertFalse(any(value["argv_prefix"] == ["call", "account_order_create"] for value in capabilities))
            self.assertFalse(any(value["argv_prefix"][0] in ["raw", "auth", "status", "unauth"] for value in capabilities))
            self.assertTrue(all(value["identities"] == ["user"] and value["scopes"] == [] for value in capabilities))
            self.assertEqual(capabilities, json.loads(archive.read("skills/openboost/capabilities.json")))
            bundle = archive.read("cli-bundle.tgz")
            self.assertEqual(hashlib.sha256(bundle).hexdigest(), bundle_sha)
            with tarfile.open(fileobj=io.BytesIO(bundle), mode="r:gz") as tar:
                members = tar.getmembers()
                executable = next(member for member in members if member.name == manifest["bundle_path"])
                self.assertTrue(executable.issym())
                self.assertEqual(executable.linkname, "../@agent-platform/openboost-connector/openboost.mjs")
                script = next(member for member in members if member.name.endswith("/openboost.mjs"))
                self.assertEqual(script.mode, 0o755)
                self.assertEqual(hashlib.sha256(tar.extractfile(script).read()).hexdigest(), build.SOURCE_SHA256)
            with zipfile.ZipFile(io.BytesIO(source)) as uploaded:
                package = json.loads(uploaded.read("package.json"))
                policy = package["agentWorkspace"]["capabilities"]
                self.assertEqual(package["name"], "@agent-platform/openboost-connector")
                self.assertEqual({value["id"]: value["risk"] for value in policy}, {value["id"]: value["risk"] for value in capabilities})
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "source.zip").write_bytes(source)
                subprocess.run(["go", "-C", str(build.ROOT.parents[2] / "backend"), "run", "./cmd/cli-connector-bundle", str(root / "source.zip"), str(root / "bundle.tgz")], check=True, timeout=90)
                self.assertEqual((root / "bundle.tgz").read_bytes(), bundle)

    def test_final_zip_passes_the_current_go_package_parser(self):
        data, _, _ = build.build(IMAGE, build.RUNTIME_VERSION)
        repo = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as directory:
            package = Path(directory) / "openboost.zip"
            package.write_bytes(data)
            result = subprocess.run(["go", "-C", str(repo / "backend"), "run", "./cmd/connector-package-validate", str(package)], check=True, capture_output=True, text=True, timeout=90)
            self.assertIn("source=openboost version=" + build.VERSION, result.stdout)
            self.assertIn("skills=1 capabilities=103", result.stdout)

    def test_wrong_runtime_or_source_integrity_is_rejected(self):
        for image, version in [("node:latest", build.RUNTIME_VERSION), (IMAGE, "24.14.0"), (IMAGE + "\n", build.RUNTIME_VERSION)]:
            with self.assertRaises(ValueError):
                build.build(image, version)
        with patch.object(build, "SOURCE_SHA256", "0" * 64):
            with self.assertRaisesRegex(ValueError, "CLI source"):
                build.build(IMAGE, build.RUNTIME_VERSION)
        with patch.object(build, "ICON_SHA256", "0" * 64):
            with self.assertRaisesRegex(ValueError, "brand asset"):
                build.build(IMAGE, build.RUNTIME_VERSION)

    def test_packaged_executable_launches_and_reports_missing_auth(self):
        data, _, _ = build.build(IMAGE, build.RUNTIME_VERSION)
        with tempfile.TemporaryDirectory() as directory, zipfile.ZipFile(io.BytesIO(data)) as archive:
            with tarfile.open(fileobj=io.BytesIO(archive.read("cli-bundle.tgz")), mode="r:gz") as tar:
                path = Path(directory) / "openboost"
                path.write_bytes(tar.extractfile(next(member for member in tar.getmembers() if member.name.endswith("/openboost.mjs"))).read())
                help_result = subprocess.run(["node", str(path), "--help"], capture_output=True, text=True, check=True, timeout=10)
                self.assertTrue(json.loads(help_result.stdout)["ok"])
                status = subprocess.run(["node", str(path), "status"], capture_output=True, text=True, check=True, timeout=10, env={"PATH": build.os.environ["PATH"]})
                self.assertFalse(json.loads(status.stdout)["data"]["authenticated"])

    def test_brand_projection_uses_the_same_official_svg(self):
        repo = build.ROOT.parents[2]
        self.assertEqual((build.ROOT / "icon.svg").read_bytes(), (repo / "frontend/src/assets/openboost.svg").read_bytes())


if __name__ == "__main__":
    unittest.main()
