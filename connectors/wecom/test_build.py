import hashlib
import json
from pathlib import Path
import re
import tarfile
import unittest

from connectors.wecom import build


class UpstreamResourcesTest(unittest.TestCase):
    def test_distributed_bundle_carries_both_reviewed_linux_binaries(self):
        with tarfile.open(build.HERE / "package/cli-bundle.tgz", mode="r:gz") as archive:
            for arch, digest in build.NATIVE_SHA256.items():
                body = archive.extractfile(f"node_modules/@wecom/cli-linux-{arch}/bin/wecom-cli").read()
                self.assertEqual(hashlib.sha256(body).hexdigest(), digest)

    def test_dual_bundle_rejects_unreviewed_native_artifact(self):
        from connectors.wecom.dual_arch import dual_bundle
        with self.assertRaisesRegex(ValueError, "integrity mismatch"):
            dual_bundle(b"irrelevant", b"unreviewed artifact")

    def test_runsc_process_limit_allows_connector_container_startup(self):
        # Production runsc could not start its sandbox with a limit of eight.
        self.assertGreaterEqual(build.RESOURCE_LIMITS["child_processes"], 128)

    def test_pinned_upstream_skills_are_complete(self):
        archive = Path(build.HERE / "upstream-v1.3.4.tar.gz").read_bytes()
        self.assertEqual(hashlib.sha256(archive).hexdigest(), build.UPSTREAM_SHA256)

        resources = build.upstream_resources()
        self.assertEqual(len(resources), 125)
        skill_paths = {name for name in resources if name.endswith("/SKILL.md")}
        self.assertEqual(
            skill_paths,
            {f"skills/wecom/references/upstream/skills/{name}/SKILL.md" for name in build.UPSTREAM_SKILLS},
        )
        self.assertIn("skills/wecom/references/upstream/docs/cli-reference.md", resources)
        self.assertIn("skills/wecom/references/upstream/LICENSE", resources)

    def test_reviewed_catalog_matches_pinned_upstream_commands(self):
        resources = build.upstream_resources()
        upstream = "\n".join(
            body.decode("utf-8") for name, body in resources.items()
            if name.startswith("skills/wecom/references/upstream/skills/") and name.endswith(".md")
        )
        documented = set(re.findall(r"wecom-cli ((?:[a-z][a-z0-9-]*)(?: [a-z][a-z0-9-]*){1,4})", upstream))
        reviewed = json.loads((build.HERE / "capabilities.json").read_text())
        commands = {item["command"] for item in reviewed}
        self.assertEqual(len(reviewed), len(commands))
        self.assertEqual(len(commands), 90)
        self.assertTrue(commands <= documented)
        self.assertEqual(commands, {" ".join(capability["argv_prefix"]) for capability in build.reviewed_capabilities()})
        for capability in build.reviewed_capabilities():
            self.assertEqual(capability["id"], "_".join(capability["argv_prefix"]))
            self.assertEqual(capability["risk"], "low" if capability["argv_prefix"][-1] in {"whoami", "search", "list", "get", "query", "download", "extract"} else "high")
        for item in reviewed:
            self.assertNotIn(item["command"].split()[0], {"auth", "schema", "cache", "slide"})


if __name__ == "__main__":
    unittest.main()
