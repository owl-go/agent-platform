import importlib.util
import io
import json
from pathlib import Path
import unittest
from unittest.mock import patch
import zipfile
spec=importlib.util.spec_from_file_location('picset_build',Path(__file__).with_name('build.py'))
builder=importlib.util.module_from_spec(spec);spec.loader.exec_module(builder)

class BuildTest(unittest.TestCase):
    def test_runtime_and_source_are_pinned(self):
        for image,version in [('image:latest',builder.RUNTIME_VERSION),('registry/image@sha256:'+'a'*64,'24.0.0')]:
            with self.assertRaises(ValueError):builder.build(image,version)
        source,icon,operations=builder.reviewed_source()
        self.assertEqual(len(operations),15)
        self.assertEqual(builder.sha256(source),builder.SOURCE_SHA256)
        with patch.object(builder,'SOURCE_SHA256','0'*64):
            with self.assertRaises(ValueError):builder.reviewed_source()
        with patch.object(builder,'POLICY_SHA256','0'*64):
            with self.assertRaises(ValueError):builder.reviewed_source()

    def test_zip_source_and_manifest_have_same_allowlist_and_no_network_escape(self):
        with patch.object(builder,'bundle',lambda source:b'fixture'):
            package,sha,source=builder.build('registry/image@sha256:'+'a'*64,builder.RUNTIME_VERSION)
        with zipfile.ZipFile(io.BytesIO(package)) as archive:
            cli=json.loads(archive.read('cli.json'))
            self.assertNotIn('mcp.json',archive.namelist())
            self.assertEqual(cli['egress_hosts'],['picsetai.cn'])
            self.assertEqual(len(cli['capabilities']),18)
            self.assertEqual(cli['authentication_driver'],'connector_package')
            self.assertEqual(json.loads(archive.read('connector-meta.json'))['source'],'picset-ai')
            for cap in cli['capabilities']:
                self.assertEqual(cap['identities'],['user'])
                self.assertEqual(cap['scopes'],[])
                self.assertEqual(cap['risk'],'low' if cap['argv_prefix'][0] in ['request','schema','operations','version'] else 'high')
        with zipfile.ZipFile(io.BytesIO(source)) as archive:
            meta=json.loads(archive.read('package.json'))
            self.assertEqual(meta['agentWorkspace']['executable'],'picset-ai')
            self.assertEqual(len(meta['agentWorkspace']['capabilities']),18)
            self.assertIn('operations.json',archive.namelist())

if __name__=='__main__':unittest.main()
