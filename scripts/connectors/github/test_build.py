import importlib.util
import io
import json
import tarfile
import unittest
from pathlib import Path
spec=importlib.util.spec_from_file_location('github_build',Path(__file__).with_name('build.py'));build=importlib.util.module_from_spec(spec);spec.loader.exec_module(build)
class BuildTest(unittest.TestCase):
 def test_policy_revision_binds_native_process_limit_to_new_bundle(self):
  package=build.ROOT.parents[2]/'connectors'/'github'/'package'
  meta=json.loads((package/'connector-meta.json').read_text())
  policy=json.loads((package/'cli.json').read_text())
  self.assertEqual(meta['version'],build.VERSION)
  self.assertEqual(policy['resource_limits']['child_processes'],64)
  # A 32-process historical revision cannot reuse this 64-process proof.
  import hashlib
  self.assertNotEqual(hashlib.sha256((package/'cli-bundle.tgz').read_bytes()).hexdigest(),'b98cf1c75a45b73c9c3b3254bee28da1cd54524586ccd981e1c4646bea13b073')
  with tarfile.open(package/'cli-bundle.tgz',mode='r:gz') as archive:
   metadata=json.load(archive.extractfile('node_modules/@agent-platform/github-connector/package.json'))
   self.assertEqual(metadata['version'],meta['version'])
   limits=metadata['agentWorkspace']['resourceLimits']
   self.assertEqual(limits,{'cpuMillis':1000,'memoryMiB':512,'childProcesses':64})
 def test_assets_require_exact_release_hash(self):
  for arch in build.ASSETS:
   with self.assertRaisesRegex(ValueError,'checksum'):build.native(b'unknown',arch)
 def test_manual_snapshot_is_complete_and_local(self):
  body=(build.ROOT/'manual-v2.102.0.tar.gz').read_bytes();self.assertEqual(build.sha(body),build.MANUAL_SHA256)
  with tarfile.open(fileobj=io.BytesIO(body),mode='r:gz') as tar:
   index=json.load(tar.extractfile('index.json'));self.assertEqual(len(index),237)
   self.assertTrue(all(item['url'].startswith('https://cli.github.com/manual/') for item in index))
   for item in index:self.assertIsNotNone(tar.getmember((item['url'].split('/')[-1] or 'index')+'.txt'))
 def test_capability_sources_and_risks_match_review(self):
  reviewed=json.loads((build.ROOT/'capabilities.json').read_text());self.assertEqual(len(reviewed),113)
  self.assertTrue(all(c['identities']==['user'] for c in reviewed))
  self.assertEqual(next(c for c in reviewed if c['argv_prefix']==['issue','comment'])['risk'],'high')
  self.assertFalse(any(c['argv_prefix'][0] in ['api','auth','extension','codespace','alias'] for c in reviewed))
  for c in reviewed:
   self.assertNotIn('--hostname',c['options']);self.assertNotIn('--editor',c['options']);self.assertNotIn('--web',c['options'])
if __name__=='__main__':unittest.main()
