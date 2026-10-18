"""Deployment-only configuration. No service credential is baked into an image."""
import base64
import hashlib
import json
import os
import re
import secrets
import tempfile
from pathlib import Path
from urllib.parse import urlsplit

VERSIONS = {'claude': '2.1.233', 'codex': '0.147.0', 'hermes': '0.19.0', 'openclaw': '2026.7.1-2', 'pi': '0.84.4'}
SUBJECT = '11111111-1111-4111-8111-111111111111'


def public_origin(value):
    parsed = urlsplit(value)
    if parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username or parsed.password or parsed.path not in ('', '/') or parsed.query or parsed.fragment:
        raise ValueError('PUBLIC_URL must be an absolute HTTP(S) origin')
    if parsed.scheme != 'https' and parsed.hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ValueError('non-loopback PUBLIC_URL requires HTTPS')
    return value.rstrip('/')


def private_write(path, value):
    path = Path(path)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=path.parent, delete=False) as output:
            temporary = Path(output.name)
            output.write(value)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def initialize_state(root, username):
    if not re.fullmatch(r'[A-Za-z0-9_.-]{1,64}', username):
        raise ValueError('ADMIN_USERNAME must use 1-64 letters, digits, dots, underscores or hyphens')
    root = Path(root)
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    path = root / 'state.json'
    if path.exists():
        state = json.loads(path.read_text())
        if state.get('version') != 1 or state.get('admin_username') != username:
            raise ValueError('existing data uses a different bootstrap identity; do not reset it')
        return state
    if any(item.name != 'instance.lock' for item in root.iterdir()):
        raise ValueError('data directory is not empty but state.json is missing; restore the original state instead of regenerating keys')
    state = {'version': 1, 'admin_username': username,
             **{key: secrets.token_hex(32) for key in ('database_password', 'service_secret', 'identity_password', 'minio_password')},
             'encryption_key': base64.b64encode(secrets.token_bytes(32)).decode()}
    private_write(path, json.dumps(state))
    return state


def realm_config(template, state, origin, password):
    realm = json.loads(json.dumps(template))
    realm['sslRequired'] = 'none' if urlsplit(origin).hostname in ('localhost', '127.0.0.1', '::1') else 'external'
    client = realm['clients'][0]
    client['redirectUris'], client['webOrigins'] = [origin + '/*'], [origin]
    client['attributes']['post.logout.redirect.uris'] = origin + '/*'
    realm['clients'][1]['secret'] = state['service_secret']
    user = realm['users'][0]
    user['username'], user['email'] = state['admin_username'], state['admin_username'] + '@agent-workspace.local'
    user['credentials'][0]['value'] = password
    realm['users'].append({'username': 'service-account-agent-workspace-admin', 'enabled': True,
                           'serviceAccountClientId': 'agent-workspace-admin',
                           'clientRoles': {'realm-management': ['manage-users', 'view-users', 'query-users']}})
    return realm


def sandbox_runtime(profile, origin):
    runtime = {'local': 'runc', 'production': 'runsc'}.get(profile)
    if not runtime:
        raise ValueError('SANDBOX_PROFILE must be explicitly local or production')
    if profile == 'local' and urlsplit(origin).hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ValueError('public deployments require explicit SANDBOX_PROFILE=production and runsc')
    return runtime


def platform_config(state, origin, image, source, profile, network, environment):
    runtime = sandbox_runtime(profile, origin)
    if not (re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}', image) or profile == 'local' and re.fullmatch(r'sha256:[a-f0-9]{64}', image)):
        raise ValueError('Runtime must use an immutable image; production requires repository@sha256')
    verified = set(environment.get('VERIFIED_RUNTIME_ENGINES', '').split(',')) - {''}
    if verified - VERSIONS.keys():
        raise ValueError('VERIFIED_RUNTIME_ENGINES contains an unknown Runtime')
    return {
        'api': {'address': '127.0.0.1:19080', 'read_header_timeout': '5s', 'idle_timeout': '60s', 'shutdown_timeout': '10s'},
        'authentication': {'mode': 'oidc', 'issuer': origin + '/identity/realms/agent-platform',
                           'internal_base_url': 'http://127.0.0.1:8081', 'audience': 'agent-platform-api',
                           'client_id': 'agent-platform-web', 'redirect_uri': origin + '/auth/callback',
                           'logout_redirect_uri': origin, 'signing_algorithms': ['RS256'], 'discovery_timeout': '10s', 'jwks_timeout': '10s'},
        'accounts': {'keycloak_base_url': 'http://127.0.0.1:8081/identity', 'realm': 'agent-platform',
                     'admin_client_id': 'agent-workspace-admin', 'admin_client_secret': state['service_secret'],
                     'bootstrap_subject': SUBJECT, 'bootstrap_username': state['admin_username'],
                     'bootstrap_email': state['admin_username'] + '@agent-workspace.local', 'bootstrap_display_name': 'Administrator'},
        'workspace': {'root': '/data/workspaces', 'known_hosts': '/data/config/known_hosts'},
        'security': {'data_encryption_key': state['encryption_key']},
        'worker': {'management_address': '127.0.0.1:19082', 'shutdown_timeout': '15s', 'poll_interval': '1s',
                   'runtime_idle_timeout': '30m', 'runtime_namespace': hashlib.sha256(source.encode()).hexdigest()[:24],
                   'credential_temp_root': '/data/run-credentials', 'sandbox_uid': 65532, 'sandbox_gid': 65532,
                   'cli_builder': {'enabled': False},
                   'runtimes': {name: {'available': profile == 'local' or name in verified, 'native_resume': False, 'image_digest': image, 'cli_version': version} for name, version in VERSIONS.items()}},
        'database': {'dsn': 'postgres://agent_workspace:' + state['database_password'] + '@127.0.0.1:5432/agent_workspace?sslmode=disable',
                     'max_open_connections': 20, 'max_idle_connections': 10, 'connection_max_idle': '5m', 'connection_max_lifetime': '30m'},
        'object_store': {'provider': 'minio', 'minio': {'endpoint': '127.0.0.1:9000', 'access_key': 'agent_workspace',
                         'secret_key': state['minio_password'], 'bucket': 'agent-workspace', 'secure': False}},
        'sandbox': {'profile': profile, 'runtime': runtime, 'egress_network': network,
                    'egress_subnet': environment.get('EGRESS_SUBNET', '172.30.0.0/24'),
                    'egress_controller_socket': '/run/agent-platform-egress/egress-controller.sock',
                    'resolver_config': '/data/config/sandbox-resolv.conf', 'resolver_addresses': ['1.1.1.1']},
        'message_channels': {'enabled': False},
    }


def browser_config(origin):
    return 'window.__AGENT_WORKSPACE_CONFIG__ = ' + json.dumps({
        'VITE_OIDC_AUTHORITY': origin + '/identity/realms/agent-platform', 'VITE_OIDC_CLIENT_ID': 'agent-platform-web',
        'VITE_OIDC_REDIRECT_URI': origin + '/auth/callback', 'VITE_OIDC_POST_LOGOUT_REDIRECT_URI': origin}) + ';\n'


def nginx_config(origin):
    public = urlsplit(origin)
    # Values are encoded through validated URL fields, never raw shell input.
    if not re.fullmatch(r'[A-Za-z0-9.\-:\[\]]+', public.netloc):
        raise ValueError('PUBLIC_URL contains unsupported hostname characters')
    return '''pid /run/agent-platform/nginx.pid;
error_log /dev/stderr warn;
events { worker_connections 1024; }
http {
  include /etc/nginx/mime.types;
  access_log off;
  client_max_body_size 1g;
  server {
    listen 8080;
    root /opt/agent-platform/web;
    location = /runtime-config.js { alias /run/agent-platform/runtime-config.js; add_header Cache-Control "no-store"; }
    location = /api/healthz { proxy_pass http://127.0.0.1:19080/healthz; add_header Cache-Control "no-store"; }
    location = /api/readyz { proxy_pass http://127.0.0.1:19080/readyz; add_header Cache-Control "no-store"; }
    location /identity/ {
      proxy_pass http://127.0.0.1:8081;
      proxy_set_header Host PUBLIC_HOST;
      proxy_set_header X-Forwarded-Host PUBLIC_HOST;
      proxy_set_header X-Forwarded-Proto PUBLIC_SCHEME;
      proxy_set_header X-Forwarded-Port PUBLIC_PORT;
      proxy_set_header X-Forwarded-For $remote_addr;
    }
    location /api/ {
      proxy_pass http://127.0.0.1:19080;
      proxy_http_version 1.1;
      proxy_buffering off;
      proxy_read_timeout 3600s;
    }
    location /assets/ { add_header Cache-Control "public, max-age=31536000, immutable"; }
    location / { try_files $uri $uri/ /index.html; add_header Cache-Control "no-store"; }
  }
}
'''.replace('PUBLIC_HOST', public.netloc).replace('PUBLIC_SCHEME', public.scheme).replace('PUBLIC_PORT', str(public.port or (443 if public.scheme == 'https' else 80)))
