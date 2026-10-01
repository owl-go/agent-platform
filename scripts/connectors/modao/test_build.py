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

spec = importlib.util.spec_from_file_location("modao_build", Path(__file__).with_name("build.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)
IMAGE = "public.ecr.aws/docker/library/node@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d"


class BuildTests(unittest.TestCase):
    def test_deterministic_real_bundle_and_frozen_risk_boundary(self):
        data, bundle_sha = build.build(IMAGE, build.RUNTIME_VERSION)
        self.assertEqual((data, bundle_sha), build.build(IMAGE, build.RUNTIME_VERSION))
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            self.assertEqual(set(archive.namelist()), {"connector-meta.json", "cli.json", "icon.svg", "cli-bundle.tgz", "skills/modao/SKILL.md", "skills/modao/capabilities.json"})
            for info in archive.infolist():
                self.assertEqual(info.external_attr >> 16 & 0o170000, 0o100000)
            manifest = json.loads(archive.read("cli.json"))
            self.assertEqual(manifest["authentication_driver"], "connector_package")
            self.assertEqual(manifest["egress_hosts"], ["modao.cc"])
            capabilities = manifest["capabilities"]
            self.assertEqual(len(capabilities), 10)
            high = {tuple(value["argv_prefix"]) for value in capabilities if value["risk"] == "high"}
            self.assertEqual(high, {("generate", "auto"), ("generate", "html"), ("generate", "react"), ("generate", "prd"), ("proto", "import")})
            self.assertFalse(any(value["argv_prefix"][0] in ["call", "raw", "auth", "status", "unauth"] for value in capabilities))
            self.assertTrue(all(value["identities"] == ["user"] and value["scopes"] == [] for value in capabilities))
            self.assertEqual(capabilities, json.loads(archive.read("skills/modao/capabilities.json")))
            bundle = archive.read("cli-bundle.tgz")
            self.assertEqual(hashlib.sha256(bundle).hexdigest(), bundle_sha)
            with tarfile.open(fileobj=io.BytesIO(bundle), mode="r:gz") as tar:
                members = tar.getmembers()
                self.assertEqual(len(members), 1)
                self.assertEqual(members[0].name, manifest["bundle_path"])
                self.assertEqual(members[0].mode, 0o755)
                self.assertEqual(hashlib.sha256(tar.extractfile(members[0]).read()).hexdigest(), build.SOURCE_SHA256)

    def test_final_zip_passes_the_current_go_package_parser(self):
        data, _ = build.build(IMAGE, build.RUNTIME_VERSION)
        repo = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as directory:
            package = Path(directory) / "modao.zip"
            package.write_bytes(data)
            result = subprocess.run(["go", "-C", str(repo / "backend"), "run", "./cmd/connector-package-validate", str(package)], check=True, capture_output=True, text=True, timeout=90)
            self.assertIn("source=modao version=0.1.0", result.stdout)
            self.assertIn("skills=1 capabilities=10", result.stdout)

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
        data, _ = build.build(IMAGE, build.RUNTIME_VERSION)
        with tempfile.TemporaryDirectory() as directory, zipfile.ZipFile(io.BytesIO(data)) as archive:
            with tarfile.open(fileobj=io.BytesIO(archive.read("cli-bundle.tgz")), mode="r:gz") as tar:
                path = Path(directory) / "modao"
                path.write_bytes(tar.extractfile(tar.getmembers()[0]).read())
                help_result = subprocess.run(["node", str(path), "--help"], capture_output=True, text=True, check=True, timeout=10)
                self.assertTrue(json.loads(help_result.stdout)["ok"])
                status = subprocess.run(["node", str(path), "status"], capture_output=True, text=True, check=True, timeout=10, env={"PATH": build.os.environ["PATH"]})
                self.assertFalse(json.loads(status.stdout)["data"]["authenticated"])

    def test_brand_projection_uses_the_same_official_png(self):
        import base64
        import re
        icon = (build.ROOT / "icon.svg").read_text()
        png = base64.b64decode(re.search("base64,([^\"]+)", icon).group(1))
        repo = build.ROOT.parents[2]
        self.assertEqual(png, (repo / "frontend/src/assets/modao.png").read_bytes())


if __name__ == "__main__":
    unittest.main()
