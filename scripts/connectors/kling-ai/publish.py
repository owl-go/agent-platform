#!/usr/bin/env python3
"""Stage a Kling MCP revision, activating only after real callback registration passes."""
import argparse
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
from urllib import parse, request, error
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('platform_publication', ROOT.parent / 'teambition/publish.py')
platform = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--package', type=Path, required=True)
    parser.add_argument('--evidence-directory', type=Path, required=True)
    parser.add_argument('--activate', action='store_true')
    args = parser.parse_args()
    config = platform.read_config(args.config)
    base = config['VITE_OIDC_AUTHORITY'].split('/identity/realms/')[0]
    archive = args.package.read_bytes()
    sha = hashlib.sha256(archive).hexdigest()
    with zipfile.ZipFile(args.package) as package:
        meta = json.loads(package.read('connector-meta.json'))
        mcp = json.loads(package.read('mcp.json'))
    if meta['source'] != 'kling-ai' or meta['type'] != 'mcp' or meta['auth_mode'] != 'oauth' or mcp != {
        'transport': 'streamable_http', 'url': 'https://klingai.com/mcp', 'egress_hosts': ['klingai.com'], 'timeout_seconds': 120
    }:
        raise RuntimeError('package differs from reviewed Kling policy')
    if args.activate:
        redirect = base + '/api/v1/connectors/kling-ai/oauth/callback'
        # An old deployment maps authenticated MCP to a provided-token form.
        # Its callback is protected by Bearer authentication. Probe only our own
        # empty callback, never an owner's pending state or provider credentials.
        probe = request.build_opener(NoRedirect())
        try:
            probe.open(redirect, timeout=20).close()
            raise RuntimeError('unexpected OAuth callback response; activation stopped')
        except error.HTTPError as exc:
            if exc.code != 400 or exc.headers.get('Cache-Control') != 'no-store' or exc.headers.get('Referrer-Policy') != 'no-referrer':
                raise RuntimeError('Kling browser OAuth adapter is not deployed; activation stopped') from None
        registration = {'client_name': 'agent_workspace_mcp', 'application_type': 'web', 'software_id': 'com.agentworkspace.kling-mcp',
                        'redirect_uris': [redirect], 'grant_types': ['authorization_code', 'refresh_token'],
                        'response_types': ['code'], 'token_endpoint_auth_method': 'none'}
        opener = request.build_opener(NoRedirect())
        try:
            with opener.open(request.Request('https://klingai.com/auth/register', data=json.dumps(registration).encode(),
                                             headers={'Content-Type': 'application/json', 'Accept': 'application/json'}), timeout=20) as response:
                registered = json.load(response)
        except error.HTTPError as exc:
            raise RuntimeError(f'Kling rejected the actual platform callback (HTTP {exc.code}); activation stopped') from None
        if registered.get('redirect_uris') != [redirect] or not registered.get('client_id') or registered.get('token_endpoint_auth_method') != 'none':
            raise RuntimeError('Kling callback registration mismatch; activation stopped')
    token = platform.administrator_token(config)
    listing = platform.api(base, token, 'GET', '/api/v1/admin/connectors/publications').get('items', [])
    items = [item for item in listing if item['revision']['source'] == 'kling-ai']
    target = next((item['revision'] for item in items if item['revision']['sha256'] == sha and item['revision']['package_version'] == meta['version']), None)
    if target is None:
        target = platform.api(base, token, 'POST', '/api/v1/admin/connectors/packages', {'archive': base64.b64encode(archive).decode()})
    if target['sha256'] != sha or target['source'] != 'kling-ai' or target['mode'] != 'mcp':
        raise RuntimeError('staged revision differs from exact package')
    args.evidence_directory.mkdir(parents=True, exist_ok=True)
    (args.evidence_directory / 'stage-response.json').write_text(json.dumps(target, ensure_ascii=False, indent=2) + '\n')
    current = next((item['publication'] for item in items if item.get('publication')), None)
    if args.activate:
        if not current or current['active_revision_id'] != target['id'] or current['state'] != 'available':
            current = platform.api(base, token, 'POST', '/api/v1/admin/connectors/publications/' + parse.quote(target['id']) + '/publish',
                                   {'expected_version': current['version'] if current else 0})
        catalog = platform.api(base, token, 'GET', '/api/v1/connectors/catalog').get('items', [])
        matches = [item for item in catalog if item['source'] == 'kling-ai']
        if len(matches) != 1 or matches[0]['active_revision_id'] != target['id'] or not matches[0]['revision']['icon'].startswith('data:image/png;base64,'):
            raise RuntimeError('User catalog or brand image verification failed')
        (args.evidence_directory / 'publication-response.json').write_text(json.dumps(current, indent=2) + '\n')
    print(json.dumps({'source': 'kling-ai', 'revision_id': target['id'], 'sha256': sha,
                      'state': 'available' if args.activate else 'staged', 'account_verification': 'not_run'}))


class NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


if __name__ == '__main__':
    try:
        main()
    except Exception as exc:
        print('publication_failed', str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__)
        raise SystemExit(1)
