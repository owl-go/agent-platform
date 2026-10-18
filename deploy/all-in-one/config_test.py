import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess
import io
from urllib.error import HTTPError

from config import browser_config, initialize_state, nginx_config, platform_config, public_origin, realm_config, sandbox_runtime
from docker_paths import rewrite
import supervise

ROOT = Path(__file__).resolve().parents[2]


class ConfigurationTests(unittest.TestCase):
    def test_only_runtime_creation_disables_the_control_plane_healthcheck(self):
        mounts = json.dumps([{'Destination': '/data', 'Source': '/var/lib/docker/volumes/test/_data'}])
        for command in ('create', 'run'):
            arguments = rewrite([command, '--read-only', 'image'], mounts)
            self.assertEqual(arguments[:2], [command, '--no-healthcheck'])
        self.assertEqual(rewrite(['inspect', 'runtime'], mounts), ['inspect', 'runtime'])

    def test_bucket_setup_retries_only_the_minio_initialization_race(self):
        initializing = HTTPError('http://127.0.0.1:9000/agent-workspace', 503, 'starting', {},
                                 io.BytesIO(b'<Error><Code>XMinioServerNotInitialized</Code></Error>'))
        with patch.object(supervise, 'request', side_effect=[initializing, b'']) as request, patch.object(
                supervise, 'check_children'), patch.object(supervise.time, 'sleep'):
            supervise.wait_for(lambda: supervise.initialize_bucket({'minio_password': 'test-only-secret'}), 'bucket')
        self.assertEqual(request.call_count, 2)

    def test_bucket_authentication_errors_fail_without_echoing_the_signed_response(self):
        denied = HTTPError('http://127.0.0.1:9000/agent-workspace', 403, 'denied', {},
                           io.BytesIO(b'<Error><Code>SignatureDoesNotMatch</Code><Request>private-signature</Request></Error>'))
        with patch.object(supervise, 'request', side_effect=denied):
            with self.assertRaises(RuntimeError) as captured:
                supervise.initialize_bucket({'minio_password': 'test-only-secret'})
        self.assertIn('SignatureDoesNotMatch', str(captured.exception))
        self.assertNotIn('private-signature', str(captured.exception))
    def test_missing_state_never_rotates_keys_over_retained_data(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'postgres').mkdir()
            (root / 'postgres/PG_VERSION').write_text('17')
            with self.assertRaisesRegex(ValueError, 'state.json is missing'):
                initialize_state(root, 'admin')
            self.assertFalse((root / 'state.json').exists())
            self.assertEqual((root / 'postgres/PG_VERSION').read_text(), '17')

    def test_readiness_retries_a_starting_subprocess_without_leaking_output(self):
        probe = unittest.mock.Mock(side_effect=[subprocess.SubprocessError('not ready'), None])
        with patch.object(supervise, 'check_children'), patch.object(supervise.time, 'sleep'):
            supervise.wait_for(probe, 'database')
        self.assertEqual(probe.call_count, 2)

    def test_command_failures_hide_captured_credentials_and_remain_retryable(self):
        failure = subprocess.CompletedProcess(['pg_isready'], 2, b'private stdout', b'private password')
        with patch.object(supervise.subprocess, 'run', return_value=failure):
            with self.assertRaises(subprocess.SubprocessError) as captured:
                supervise.execute(['pg_isready'])
        self.assertEqual(str(captured.exception), 'pg_isready initialization failed (exit 2)')

    def test_bootstrap_defaults_are_created_once_without_resetting_state(self):
        with tempfile.TemporaryDirectory() as directory:
            first = initialize_state(directory, 'admin')
            second = initialize_state(directory, 'admin')
            self.assertEqual(first, second)
            self.assertEqual(os.stat(Path(directory) / 'state.json').st_mode & 0o777, 0o600)
            with self.assertRaises(ValueError):
                initialize_state(directory, 'other-admin')
            template = json.loads((ROOT / 'deploy/platform/config/keycloak-realm.json').read_text())
            realm = realm_config(template, first, 'http://localhost:8080', 'admin123')
            self.assertEqual(realm['users'][0]['username'], 'admin')
            self.assertEqual(realm['users'][0]['credentials'][0]['value'], 'admin123')
            self.assertEqual(realm['clients'][0]['redirectUris'], ['http://localhost:8080/*'])
            configuration = platform_config(first, 'http://localhost:8080', 'sha256:' + 'a' * 64, '/volumes/test', 'local', 'runtime-test', {})
            self.assertEqual(Path(configuration['sandbox']['egress_controller_socket']).name, 'egress-controller.sock')
            self.assertTrue(all(item['available'] and not item['native_resume'] for item in configuration['worker']['runtimes'].values()))
            self.assertEqual(configuration['accounts']['bootstrap_subject'], realm['users'][0]['id'])
            self.assertNotIn('admin123', json.dumps(configuration))
            with self.assertRaises(ValueError):
                platform_config(first, 'https://workspace.example', 'sha256:' + 'a' * 64, '/volumes/test', 'production', 'network', {})
            production = platform_config(first, 'https://workspace.example', 'example/runtime@sha256:' + 'a' * 64, '/volumes/test', 'production', 'network', {})
            self.assertFalse(any(item['available'] for item in production['worker']['runtimes'].values()))
            verified = platform_config(first, 'https://workspace.example', 'example/runtime@sha256:' + 'a' * 64, '/volumes/test', 'production', 'network', {'VERIFIED_RUNTIME_ENGINES': 'codex'})
            self.assertTrue(verified['worker']['runtimes']['codex']['available'])
            self.assertFalse(verified['worker']['runtimes']['claude']['available'])

    def test_origins_are_validated_and_public_settings_have_no_secrets(self):
        for origin in ('http://workspace.example', 'https://user:password@workspace.example', 'https://workspace.example/path', 'https://workspace.example?test=1'):
            with self.assertRaises(ValueError):
                public_origin(origin)
        self.assertEqual(public_origin('http://localhost:18080/'), 'http://localhost:18080')
        public = browser_config('https://workspace.example')
        self.assertIn('https://workspace.example/identity/realms/agent-platform', public)
        self.assertNotIn('password', public)
        proxy = nginx_config('https://workspace.example:8443')
        self.assertIn('X-Forwarded-Port 8443', proxy)
        self.assertIn('proxy_buffering off', proxy)
        self.assertIn('location = /api/healthz { proxy_pass http://127.0.0.1:19080/healthz;', proxy)
        self.assertIn('location = /api/readyz { proxy_pass http://127.0.0.1:19080/readyz;', proxy)
        self.assertEqual(sandbox_runtime('local', 'http://localhost:8080'), 'runc')
        self.assertEqual(sandbox_runtime('production', 'https://workspace.example'), 'runsc')
        with self.assertRaises(ValueError):
            sandbox_runtime('local', 'https://workspace.example')

    def test_docker_mapping_confines_sources_and_preserves_readonly(self):
        mounts = json.dumps([{'Destination': '/data', 'Source': '/var/lib/docker/volumes/test/_data'}])
        mapped = rewrite(['create', '--mount', 'type=bind,src=/data/workspaces/run-1,dst=/workspace,readonly=true', '--user', '65532:65532'], mounts)
        self.assertIn('type=bind,src=/var/lib/docker/volumes/test/_data/workspaces/run-1,dst=/workspace,readonly=true', mapped)
        for source in ('/etc/passwd', '/data/../etc/passwd', '/database/run-1', '/var/run/docker.sock'):
            with self.assertRaises(ValueError):
                rewrite(['run', '--mount', 'type=bind,src=' + source + ',dst=/workspace'], mounts)
        self.assertEqual(rewrite(['inspect', 'agent-runtime-warm-test'], mounts), ['inspect', 'agent-runtime-warm-test'])


if __name__ == '__main__':
    unittest.main()
