import unittest
import build
class BuildTests(unittest.TestCase):
 def test_rejects_unpinned_native_packages(self):
  for arch in build.INTEGRITIES:
   with self.assertRaisesRegex(ValueError,'integrity'):build.native(b'unreviewed',arch)
 def test_individual_policy_and_risk(self):
  caps=build.capabilities();self.assertEqual(len(caps),32)
  self.assertEqual(len({c['id'] for c in caps}),32)
  for c in caps:
   if c['argv_prefix'][0] in ['image','pdf','office','txt'] or c['argv_prefix'] in [['doc','download'],['doc','move']]:self.assertEqual(c['risk'],'high')
   self.assertEqual(c['identities'],['user']);self.assertEqual(c['scopes'],[])
  self.assertFalse(any(c['argv_prefix'][0] in ['auth','completion'] for c in caps))
 def test_runtime_and_skill_validation(self):
  with self.assertRaisesRegex(ValueError,'RepoDigest'):build.build({},b'', 'node:latest','24.15.0')
  with self.assertRaisesRegex(ValueError,'Skill'):build.build({},b'changed', 'registry.invalid/runtime@sha256:'+'a'*64,'24.15.0')
