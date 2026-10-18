import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name('publish-unified-images.sh')


class UnifiedPublicationTests(unittest.TestCase):
    def run_publisher(self, **overrides):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'scripts').mkdir()
            (root / 'bin').mkdir()
            shutil.copyfile(SCRIPT, root / 'scripts/publish-unified-images.sh')
            fake = '''import json,os,sys
from pathlib import Path
name=Path(sys.argv[0]).name
with open(os.environ['TEST_CALLS'],'a') as output:
    output.write(json.dumps({'name':name,'args':sys.argv[1:],'tag':os.environ.get('TAG'),'runtime_tag':os.environ.get('RUNTIME_TAG')})+'\\n')
args=sys.argv[1:]
if name=='git':
    if args[:1]==['status'] and os.environ.get('TEST_DIRTY'): print(' M source.go')
    elif args[:1]==['rev-parse']:
        print(('b' if args[-1]=='origin/main_temp' and os.environ.get('TEST_MISMATCH') else 'a')*40)
    elif args[:1]==['show']: print('2026-10-18')
elif 'bake' in args and os.environ.get('TAG','').endswith('-amd64') and os.environ.get('TEST_AMD_FAIL'):
    sys.exit(9)
'''
            for name in ('git', 'docker'):
                executable = root / 'bin' / name
                executable.write_text('#!' + sys.executable + '\n' + fake)
                executable.chmod(0o700)
            calls = root / 'calls.jsonl'
            environment = {key: value for key, value in os.environ.items()
                           if key not in ('IMAGE_REPOSITORY', 'RUNTIME_IMAGE_REPOSITORY', 'RELEASE_TAG',
                                          'TAG', 'RUNTIME_TAG', 'TEST_DIRTY', 'TEST_MISMATCH', 'TEST_AMD_FAIL')}
            environment.update(PATH=str(root / 'bin') + os.pathsep + os.environ['PATH'], TEST_CALLS=str(calls), **overrides)
            result = subprocess.run(['bash', str(root / 'scripts/publish-unified-images.sh')],
                                    env=environment, capture_output=True, text=True, timeout=15)
            recorded = [json.loads(line) for line in calls.read_text().splitlines()]
            return result, [call for call in recorded if call['name'] == 'docker']

    def test_dirty_checkout_never_contacts_docker(self):
        result, calls = self.run_publisher(TEST_DIRTY='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('dirty checkout', result.stderr)
        self.assertEqual(calls, [])

    def test_unintegrated_commit_never_contacts_docker(self):
        result, calls = self.run_publisher(TEST_MISMATCH='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('main_temp', result.stderr)
        self.assertEqual(calls, [])

    def test_failed_second_architecture_does_not_update_release_aliases(self):
        result, calls = self.run_publisher(TEST_AMD_FAIL='1')
        self.assertEqual(result.returncode, 9)
        self.assertEqual(len(calls), 3)
        self.assertTrue(all('bake' in call['args'] for call in calls))

    def test_both_architectures_precede_multiarchitecture_and_explicit_aliases(self):
        result, calls = self.run_publisher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 12)
        for offset, architecture in ((0, 'arm64'), (2, 'amd64')):
            app, runtime = calls[offset:offset + 2]
            self.assertIn('*.platform=linux/' + architecture, app['args'])
            self.assertIn('*.platform=linux/' + architecture, runtime['args'])
            self.assertIn('--push', app['args'])
            self.assertIn('--push', runtime['args'])
            self.assertTrue(app['tag'].endswith('-' + architecture))
            self.assertTrue(runtime['runtime_tag'].endswith('-' + architecture))
        app_manifest = calls[4]['args']
        runtime_manifest = calls[5]['args']
        self.assertIn('haozhi152/agent-platform:latest', app_manifest)
        self.assertEqual(app_manifest[-2:], ['haozhi152/agent-platform:2026-10-18-aaaaaaaaaaaa-arm64',
                                            'haozhi152/agent-platform:2026-10-18-aaaaaaaaaaaa-amd64'])
        self.assertIn('haozhi152/agent-platform-runtime:latest', runtime_manifest)
        self.assertEqual(runtime_manifest[-2:], ['haozhi152/agent-platform-runtime:2026-10-18-aaaaaaaaaaaa-arm64',
                                                'haozhi152/agent-platform-runtime:2026-10-18-aaaaaaaaaaaa-amd64'])
        self.assertIn('haozhi152/agent-platform:arm64', calls[6]['args'])
        self.assertIn('haozhi152/agent-platform-runtime:arm64', calls[7]['args'])
        self.assertIn('haozhi152/agent-platform:amd64', calls[8]['args'])
        self.assertIn('haozhi152/agent-platform-runtime:amd64', calls[9]['args'])


class RuntimeSmokeShellTests(unittest.TestCase):
    def test_default_and_unified_modes_work_with_system_bash(self):
        for unified in ('false', 'true'):
            with self.subTest(unified=unified), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                executable = root / 'docker'
                executable.write_text('#!' + sys.executable + '''
import json,os,sys
with open(os.environ['TEST_CALLS'],'a') as output: output.write(json.dumps(sys.argv[1:])+'\\n')
if '--version' in sys.argv:
    print({'claude':'2.1.233','codex':'0.147.0','hermes':'0.19.0','openclaw':'2026.7.1-2','pi':'0.84.4'}[sys.argv[-2]])
elif sys.argv[-1]=='-u': print('65532')
''')
                executable.chmod(0o700)
                calls = root / 'calls.jsonl'
                environment = dict(os.environ, PATH=str(root) + os.pathsep + os.environ['PATH'],
                                   TEST_CALLS=str(calls), UNIFIED_CONTROL_IMAGE=unified)
                result = subprocess.run(['/bin/bash', str(SCRIPT.parent / 'conformance/runtime-image-smoke.sh')],
                                        env=environment, capture_output=True, text=True, timeout=15)
                self.assertEqual(result.returncode, 0, result.stderr)
                recorded = [json.loads(line) for line in calls.read_text().splitlines()]
                self.assertEqual(len(recorded), 8)
                for arguments in recorded[:-1]:
                    self.assertEqual('--user' in arguments, unified == 'true')
                self.assertNotIn('--user', recorded[-1], 'Builder smoke must check its own default UID')


if __name__ == '__main__':
    unittest.main()
