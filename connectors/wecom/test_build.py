import hashlib
from pathlib import Path
import unittest

from connectors.wecom import build


class UpstreamResourcesTest(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
