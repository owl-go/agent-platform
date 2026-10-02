import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from urllib import error
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('kling_publish', ROOT / 'publish.py')
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublishTest(unittest.TestCase):
    def test_staging_reuses_revision_without_mutation_or_oauth(self):
        self.exercise(False)

    def test_activation_checks_callback_and_verifies_real_catalog_route(self):
        self.exercise(True)

    def test_explicit_unverified_release_still_checks_deployed_callback_and_catalog(self):
        self.exercise(True, allow_unverified=True)

    def exercise(self, activate, allow_unverified=False):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            package = directory / 'package.zip'
            with zipfile.ZipFile(package, 'w') as archive:
                for name in ['connector-meta.json', 'mcp.json']:
                    archive.write(ROOT / name, name)
            sha = hashlib.sha256(package.read_bytes()).hexdigest()
            target = {'id': 'revision', 'source': 'kling-ai', 'sha256': sha, 'mode': 'mcp', 'package_version': '0.1.0'}
            publication = {'source': 'kling-ai', 'active_revision_id': 'revision', 'version': 1, 'state': 'available'}
            calls = []

            def api(base, token, method, path, body=None):
                calls.append((method, path))
                if path == '/api/v1/admin/connectors/publications':
                    return {'items': [{'revision': target}]}
                if path == '/api/v1/admin/connectors/publications/revision/publish':
                    self.assertEqual(body, {'expected_version': 0})
                    return publication
                if path == '/api/v1/connectors/catalog':
                    return {'items': [{**publication, 'revision': {'icon': 'data:image/png;base64,official-image'}}]}
                self.fail('Unexpected mutation or API route: ' + method + ' ' + path)

            opener = unittest.mock.Mock()
            registered = {'client_id': 'client', 'redirect_uris': ['https://workspace.test/api/v1/connectors/kling-ai/oauth/callback'], 'token_endpoint_auth_method': 'none'}
            opener.open.side_effect = [error.HTTPError('https://workspace.test/callback', 400, '', {'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer'}, None), contextlib.closing(io.BytesIO(json.dumps(registered).encode()))]
            argv = ['publish.py', '--config', str(directory / 'config'), '--package', str(package), '--evidence-directory', str(directory / 'evidence')]
            if activate:
                argv.append('--activate')
            if allow_unverified:
                argv.append('--allow-unverified-oauth')
            with patch('sys.argv', argv), patch.object(publisher.platform, 'read_config', return_value={'VITE_OIDC_AUTHORITY': 'https://workspace.test/identity/realms/workspace'}), patch.object(publisher.platform, 'administrator_token', return_value='fixture-token'), patch.object(publisher.platform, 'api', side_effect=api), patch.object(publisher.request, 'build_opener', return_value=opener), contextlib.redirect_stdout(io.StringIO()):
                publisher.main()
            if activate:
                self.assertEqual(opener.open.call_count, 1 if allow_unverified else 2)
                self.assertIn(('GET', '/api/v1/connectors/catalog'), calls)
            else:
                opener.open.assert_not_called()
                self.assertEqual(calls, [('GET', '/api/v1/admin/connectors/publications')])


if __name__ == '__main__':
    unittest.main()
