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
    def test_https_registration_infrastructure_survives_restart_without_rotating_other_secrets(self):
        with tempfile.TemporaryDirectory() as directory:
            state = initialize_state(directory, 'admin')
            original = dict(state)
            registration = supervise.registration_config(Path(directory) / 'config', 'https://workspace.example')
            self.assertEqual(registration['public_url'], 'https://workspace.example')
            self.assertGreaterEqual(len(registration['client_secret']), 32)
            self.assertTrue(registration['signing_key'])
            restarted = supervise.registration_config(Path(directory) / 'config', 'https://workspace.example')
            self.assertEqual(registration, restarted)
            moved = supervise.registration_config(Path(directory) / 'config', 'https://other.example')
            self.assertEqual(moved['client_secret'], registration['client_secret'])
            self.assertEqual(moved['signing_key'], registration['signing_key'])
            self.assertEqual(state, original)
            self.assertEqual(initialize_state(directory, 'admin'), original)
            self.assertEqual((Path(directory) / 'config/registration.json').stat().st_mode & 0o777, 0o600)
            configured = platform_config(state, 'https://workspace.example', 'example/runtime@sha256:' + 'a' * 64,
                                        '/volumes/test', 'production', 'network', {}, registration=registration)
            self.assertEqual(configured['accounts']['registration_broker'], registration)
            self.assertNotIn(registration['client_secret'], browser_config('https://workspace.example'))

    def test_http_local_registration_stays_unconfigured_and_invalid_retained_keys_are_not_replaced(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertEqual(supervise.registration_config(root, 'http://localhost:8080'), {})
            self.assertFalse((root / 'registration.json').exists())
            (root / 'registration.json').write_text('{"client_secret":"broken","signing_key":"broken"}')
            with self.assertRaises(ValueError):
                supervise.registration_config(root, 'https://workspace.example')
            self.assertIn('broken', (root / 'registration.json').read_text())

    def test_https_startup_uses_the_shared_narrow_identity_setup(self):
        profile = {'attributes': [{'name': 'email', 'required': {'roles': ['user']}, 'validations': {'email': {}}}],
                   'unmanagedAttributePolicy': 'DISABLED'}
        roles = [{'name': 'query-users'}]
        changes = []
        def call(path, method='GET', body=None):
            if path == '/users/profile':
                if method == 'PUT':
                    profile.update(json.loads(body))
                    changes.append('profile')
                return profile
            if path == '/clients?clientId=agent-workspace-admin':
                return [{'id': 'service', 'clientId': 'agent-workspace-admin', 'serviceAccountsEnabled': True}]
            if path == '/clients?clientId=realm-management':
                return [{'id': 'management'}]
            if path == '/clients/service/service-account-user':
                return {'id': 'service-user'}
            if path == '/users/service-user/role-mappings/clients/management':
                if method == 'POST':
                    roles.extend(json.loads(body))
                    changes.append('roles')
                return roles
            if path == '/clients/management/roles':
                return [{'name': name} for name in ('manage-users', 'view-users', 'manage-identity-providers', 'manage-realm')]
            self.fail('Unexpected identity request')
        with patch.object(supervise, 'APPLICATION', ROOT / 'scripts'), \
                patch.object(supervise, 'identity_request', return_value=call):
            supervise.synchronize_registration({'identity_password': 'private'}, 'https://workspace.example')
            supervise.synchronize_registration({'identity_password': 'private'}, 'https://workspace.example')
        self.assertEqual(changes, ['profile', 'roles'])
        self.assertEqual({role['name'] for role in roles}, {'query-users', 'manage-users', 'view-users', 'manage-identity-providers'})
        self.assertEqual(profile['unmanagedAttributePolicy'], 'DISABLED')
        self.assertEqual(profile['attributes'][0], {'name': 'email', 'validations': {'email': {}}})
        with patch.object(supervise, 'identity_request', side_effect=AssertionError('HTTP must not configure identity')):
            supervise.synchronize_registration({}, 'http://localhost:8080')

    def test_registration_key_failure_never_echoes_subprocess_output_or_persists_partial_state(self):
        with tempfile.TemporaryDirectory() as directory, patch('config.subprocess.run', return_value=
                subprocess.CompletedProcess(['openssl'], 1, b'private-key-output', b'private-error')):
            with self.assertRaisesRegex(ValueError, '^registration signing key initialization failed$'):
                supervise.registration_config(directory, 'https://workspace.example')
            self.assertFalse((Path(directory) / 'registration.json').exists())

    def test_enabled_engines_do_not_claim_optional_capabilities_or_override_explicit_opt_out(self):
        with tempfile.TemporaryDirectory() as directory:
            state = initialize_state(directory, 'admin')
            args = (state, 'https://workspace.example', 'example/runtime@sha256:' + 'a' * 64, '/volumes/test', 'production', 'network')
            selected = platform_config(*args, {'ENABLED_RUNTIME_ENGINES': 'codex,pi', 'VERIFIED_RUNTIME_ENGINES': 'claude'})
            self.assertEqual({name for name, item in selected['worker']['runtimes'].items() if item['available']}, {'codex', 'pi'})
            self.assertFalse(any(item['native_resume'] for item in selected['worker']['runtimes'].values()))
            with self.assertRaisesRegex(ValueError, 'unknown Runtime'):
                platform_config(*args, {'ENABLED_RUNTIME_ENGINES': 'codex,unknown'})

    def test_fixed_components_build_without_backend_side_effects(self):
        dockerfile = (ROOT / 'deploy/all-in-one/Dockerfile').read_text()
        fixed_stage = dockerfile.split('COPY backend .', 1)[0]
        self.assertIn('mkdir -p /out', fixed_stage)
        self.assertIn('go install -trimpath', fixed_stage)
        self.assertGreater(dockerfile.index('COPY --from=build /out/api /out/worker'),
                           dockerfile.index('/opt/keycloak/bin/kc.sh build --db=postgres'))

    def test_compose_prepares_a_separate_runtime_image(self):
        compose = (ROOT / 'deploy/all-in-one/compose.yaml').read_text()
        dockerfile = (ROOT / 'deploy/all-in-one/Dockerfile').read_text()
        self.assertIn('haozhi152/agent-platform-runtime:latest', compose)
        self.assertIn('condition: service_completed_successfully', compose)
        self.assertIn('name: ${DATA_VOLUME:-agent-platform-data}', compose)
        self.assertNotIn('FROM runtime', dockerfile)

    def test_local_control_uses_the_separate_runtime_image_id(self):
        control = [{'Image': 'sha256:' + 'c' * 64,
                    'Mounts': [{'Destination': '/data', 'Source': '/var/lib/docker/volumes/test/_data', 'RW': True}]}]
        runtime = [{'Id': 'sha256:' + 'a' * 64, 'RepoDigests': []}]
        runtimes = {'runc': {'path': 'runc'}}

        def fake_docker(*arguments):
            if arguments == ('inspect', 'control'):
                return json.dumps(control)
            if arguments == ('info', '--format', '{{json .Runtimes}}'):
                return json.dumps(runtimes)
            if arguments == ('image', 'inspect', 'example/runtime:latest'):
                return json.dumps(runtime)
            if arguments[:2] == ('network', 'inspect'):
                raise subprocess.SubprocessError('missing')
            if arguments[:2] == ('network', 'create'):
                return 'runtime-network'
            raise AssertionError(arguments)

        with patch.object(supervise.Path, 'is_socket', return_value=True), \
                patch.object(supervise.os, 'uname', return_value=unittest.mock.Mock(nodename='control')), \
                patch.dict(os.environ, {'RUNTIME_IMAGE': 'example/runtime:latest'}), \
                patch.object(supervise, 'docker', side_effect=fake_docker):
            image, mounts, source, network = supervise.docker_identity('local')
        self.assertEqual(image, 'sha256:' + 'a' * 64)
        self.assertEqual(source, '/var/lib/docker/volumes/test/_data')
        self.assertEqual(mounts, control[0]['Mounts'])
        self.assertTrue(network.startswith('agent-platform-runtime-'))

    def test_production_runtime_requires_an_exact_available_repo_digest(self):
        control = [{'Image': 'sha256:' + 'c' * 64,
                    'Mounts': [{'Destination': '/data', 'Source': '/var/lib/docker/volumes/test/_data', 'RW': True}]}]

        def fake_docker(*arguments):
            if arguments == ('inspect', 'control'):
                return json.dumps(control)
            if arguments == ('info', '--format', '{{json .Runtimes}}'):
                return json.dumps({'runsc': {'path': 'runsc'}})
            raise AssertionError(arguments)

        with patch.object(supervise.Path, 'is_socket', return_value=True), \
                patch.object(supervise.os, 'uname', return_value=unittest.mock.Mock(nodename='control')), \
                patch.dict(os.environ, {'RUNTIME_IMAGE': 'example/runtime:latest'}), \
                patch.object(supervise, 'docker', side_effect=fake_docker):
            with self.assertRaisesRegex(ValueError, 'immutable repository'):
                supervise.docker_identity('production')

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
            self.assertTrue(all(item['available'] and not item['native_resume'] for item in production['worker']['runtimes'].values()))
            disabled = platform_config(first, 'https://workspace.example', 'example/runtime@sha256:' + 'a' * 64, '/volumes/test', 'production', 'network', {'ENABLED_RUNTIME_ENGINES': 'none'})
            self.assertFalse(any(item['available'] for item in disabled['worker']['runtimes'].values()))
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
