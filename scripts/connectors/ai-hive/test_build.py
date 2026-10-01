import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("ai_hive_build", Path(__file__).with_name("build.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


class IntegrityTest(unittest.TestCase):
    def test_rejects_unreviewed_upstream_before_packaging(self):
        with self.assertRaisesRegex(ValueError, "reviewed npm"):
            build.build(b"unreviewed archive")


if __name__ == "__main__":
    unittest.main()
