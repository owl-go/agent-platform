import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('teambition_build', Path(__file__).with_name('build.py'))
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


class TeambitionBuildTest(unittest.TestCase):
    def test_rejects_unpinned_upstream(self):
        with self.assertRaisesRegex(ValueError, 'integrity'):
            builder.binaries(b'arbitrary release')

    def test_rejects_mutable_runtime_or_skill(self):
        with self.assertRaisesRegex(ValueError, 'RepoDigest'):
            builder.build(b'', b'', 'node:24', '24.15.0')
        with self.assertRaisesRegex(ValueError, 'Skill checksum'):
            builder.build(b'', b'new skill', 'registry.example/runtime@sha256:' + 'a' * 64, '24.15.0')

    def test_no_generic_raw_tools_or_mutating_reads(self):
        policy = builder.capabilities()
        self.assertFalse(any(item['argv_prefix'] == ['tools', 'call'] for item in policy))
        for item in policy:
            if item['argv_prefix'] in [['task', 'create'], ['task', 'move'], ['task', 'comment']]:
                self.assertEqual(item['risk'], 'high')
                self.assertEqual(item['scopes'], ['task:write'])
            self.assertEqual(item['identities'], ['user'])
            self.assertEqual(item['egress_hosts'], ['open.teambition.com'])

    def test_comment_readback_has_read_scope_and_no_unsupported_task_writes(self):
        policy = builder.capabilities()
        activity = next(item for item in policy if item['argv_prefix'] == ['task', 'activity'])
        self.assertEqual(activity['risk'], 'low')
        self.assertEqual(activity['scopes'], ['task:read'])
        self.assertFalse(any(item['argv_prefix'] in [['task', 'update'], ['task', 'delete']] for item in policy))


if __name__ == '__main__':
    unittest.main()
