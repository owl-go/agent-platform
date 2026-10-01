#!/usr/bin/env python3
"""Run platform ZIP build/Conformance, stage and publish a reviewed Teambition package."""
import argparse
import base64
import hashlib
import html
import json
import secrets
import time
import zipfile
from html.parser import HTMLParser
from http.cookiejar import CookieJar
from pathlib import Path
from urllib import error, parse, request


def read_config(path):
    config = {}
    for line in path.read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            config[key] = value.strip().strip('"').strip("'")
    return config


class LoginForm(HTMLParser):
    action = ''

    def handle_starttag(self, tag, attrs):
        if tag == 'form' and not self.action:
            self.action = dict(attrs).get('action', '')


def administrator_token(config):
    verifier = secrets.token_urlsafe(48)
    challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).decode().rstrip('=')
    state = secrets.token_urlsafe(16)
    authority = config['VITE_OIDC_AUTHORITY'].rstrip('/')
    redirect, client = config['VITE_OIDC_REDIRECT_URI'], config['VITE_OIDC_CLIENT_ID']
    params = {'client_id': client, 'redirect_uri': redirect, 'response_type': 'code', 'scope': 'openid',
              'state': state, 'code_challenge': challenge, 'code_challenge_method': 'S256'}
    opener = request.build_opener(request.HTTPCookieProcessor(CookieJar()))
    with opener.open(authority + '/protocol/openid-connect/auth?' + parse.urlencode(params), timeout=20) as response:
        page = response.read().decode()
    form = LoginForm()
    form.feed(page)
    if not form.action or parse.urlparse(html.unescape(form.action)).netloc != parse.urlparse(authority).netloc:
        raise RuntimeError('platform administrator login form is unavailable or has changed origin')
    body = parse.urlencode({'username': config['BOOTSTRAP_ADMIN_EMAIL'],
                            'password': config['PLATFORM_ADMIN_PASSWORD'], 'credentialId': ''}).encode()
    with opener.open(request.Request(html.unescape(form.action), data=body,
                                    headers={'Content-Type': 'application/x-www-form-urlencoded'}), timeout=20) as response:
        final_url = response.url
    query = parse.parse_qs(parse.urlparse(final_url).query)
    if query.get('state', [''])[0] != state or not query.get('code'):
        raise RuntimeError('platform administrator authorization code is unavailable')
    body = parse.urlencode({'grant_type': 'authorization_code', 'client_id': client,
                            'redirect_uri': redirect, 'code': query['code'][0], 'code_verifier': verifier}).encode()
    with opener.open(request.Request(authority + '/protocol/openid-connect/token', data=body,
                                    headers={'Content-Type': 'application/x-www-form-urlencoded'}), timeout=20) as response:
        return json.load(response)['access_token']


def api(base, token, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    headers = {'Authorization': 'Bearer ' + token}
    if body is not None:
        headers['Content-Type'] = 'application/json'
    try:
        with request.urlopen(request.Request(base + path, data=data, headers=headers, method=method), timeout=120) as response:
            return json.load(response)
    except error.HTTPError as exc:
        try:
            reason = json.loads(exc.read(4096)).get('reason', '')
        except Exception:
            reason = ''
        raise RuntimeError(f'platform API {method} {path} returned HTTP {exc.code} ({reason})') from None


def has_verified_revision(items, bundle_sha, digest):
    return any(item['revision'].get('bundle_sha256') == bundle_sha and
               digest in item['revision'].get('runtime_digests', []) and
               item['revision'].get('conformance_available') for item in items)


def build_identity(source):
    if source not in ['teambition', 'modao', 'camscanner']:
        raise RuntimeError('unreviewed publication source')
    return {'teambition':'Teambition','modao':'Modao','camscanner':'CamScanner'}[source] + ' package build ', '@agent-platform/' + source + '-connector'


def cleanup_staging_definitions(base, token, source='teambition'):
    prefix, npm_package = build_identity(source)
    definitions = api(base, token, 'GET', '/api/v1/connectors/cli').get('items', [])
    health = {item['definition_id']: item for item in
              api(base, token, 'GET', '/api/v1/admin/connectors/cli-health').get('items', [])}
    for definition in definitions:
        if definition.get('managed_installation') or not definition['name'].startswith(prefix):
            continue
        if definition.get('npm_package') != npm_package:
            raise RuntimeError('staging source package identity changed; cleanup stopped')
        usage = health.get(definition['id'])
        if usage is None or any(usage.get(key, 0) for key in ['enablement_count', 'active_authorization_count']):
            raise RuntimeError('staging Definition has user usage; cleanup stopped')
        api(base, token, 'DELETE', '/api/v1/admin/connectors/cli/' + parse.quote(definition['id']) +
            '?expected_version=' + str(definition['version']))
    remaining = api(base, token, 'GET', '/api/v1/connectors/cli').get('items', [])
    if any(item['name'].startswith(prefix) for item in remaining):
        raise RuntimeError('staging Definition remains in administrator catalog')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connector-source', choices=['teambition', 'modao', 'camscanner'], default='teambition')
    parser.add_argument('--api-base', help='trusted platform API origin; deployment-host container origin avoids public upload hairpin')
    parser.add_argument('--config', type=Path, required=True, help='platform env file on the authorized deployment host')
    parser.add_argument('--package', type=Path, required=True)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--evidence-directory', type=Path, required=True)
    args = parser.parse_args()
    config = read_config(args.config)
    print('Authenticate deployment administrator', flush=True)
    token = administrator_token(config)
    base = args.api_base or config['VITE_OIDC_AUTHORITY'].split('/identity/realms/')[0]
    parsed_base = parse.urlparse(base)
    if parsed_base.scheme not in ['http', 'https'] or not parsed_base.netloc or parsed_base.path or parsed_base.query or parsed_base.fragment or parsed_base.username:
        raise RuntimeError('API base must be the trusted platform origin')
    package = args.package.read_bytes()
    with zipfile.ZipFile(args.package) as archive:
        meta = json.loads(archive.read('connector-meta.json'))
        cli = json.loads(archive.read('cli.json'))
        bundle_sha = hashlib.sha256(archive.read('cli-bundle.tgz')).hexdigest()
    if meta['source'] != args.connector_source or cli['authentication_driver'] != 'connector_package':
        raise RuntimeError('unexpected package identity')
    digest = cli['runtime']['digest']
    prefix, npm_package = build_identity(args.connector_source)
    if not config['RUNTIME_IMAGE'].endswith('@' + digest):
        raise RuntimeError('package Runtime differs from current deployment')
    args.evidence_directory.mkdir(parents=True, exist_ok=True)

    def record(name, value):
        (args.evidence_directory / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    print('Inspect existing publication and Conformance source', flush=True)
    listing = api(base, token, 'GET', '/api/v1/admin/connectors/publications')
    items = [item for item in listing.get('items', []) if item['revision']['source'] == args.connector_source]
    current = next((item['publication'] for item in items if item.get('publication')), None)
    if not has_verified_revision(items, bundle_sha, digest):
        # The Worker owns the build and records only actual successful Conformance.
        definitions = api(base, token, 'GET', '/api/v1/connectors/cli').get('items', [])
        name = prefix + meta['version'] + ' ' + bundle_sha[:12]
        definition = next((item for item in definitions if item['name'] == name and not item.get('managed_installation')), None)
        if definition is None:
            print('Upload reviewed source ZIP', flush=True)
            definition = api(base, token, 'POST', '/api/v1/admin/connectors/cli', {'definition': {
                'name': name, 'icon': 'terminal', 'description': 'Reviewed ' + args.connector_source + ' package Conformance staging source',
                'installation_type': 'upload', 'archive': base64.b64encode(args.source.read_bytes()).decode(),
            }})
        print('Source state:', definition['state'], flush=True)
        if definition['state'] == 'draft':
            definition = api(base, token, 'POST', '/api/v1/admin/connectors/cli/' + parse.quote(definition['id']) + '/publish',
                             {'expected_version': definition['version']})
        deadline = time.monotonic() + 240
        while definition['state'] in ['building', 'testing', 'publishing', 'queued']:
            if time.monotonic() >= deadline:
                raise RuntimeError('platform CLI build is still running; rerun to resume without duplicating')
            time.sleep(3)
            definitions = api(base, token, 'GET', '/api/v1/connectors/cli').get('items', [])
            definition = next(item for item in definitions if item['id'] == definition['id'])
        if definition.get('bundle_sha256') != bundle_sha or digest not in definition.get('conformance_runtime_digests', []):
            raise RuntimeError('platform build failed or exact bundle/Runtime Conformance differs')
        record('build-response.json', definition)
        print(json.dumps({'platform_conformance': 'passed', 'bundle_sha256': bundle_sha, 'runtime_digest': digest}), flush=True)
    target = next((item['revision'] for item in items if item['revision'].get('bundle_sha256') == bundle_sha and
                   item['revision']['package_version'] == meta['version']), None)
    if target is None:
        target = api(base, token, 'POST', '/api/v1/admin/connectors/packages', {'archive': base64.b64encode(package).decode()})
    if target.get('bundle_sha256') != bundle_sha or not target.get('conformance_available'):
        raise RuntimeError('staged revision is not verified for this bundle')
    record('stage-response.json', target)
    if not current or current['active_revision_id'] != target['id'] or current['state'] != 'available':
        current = api(base, token, 'POST', '/api/v1/admin/connectors/publications/' + parse.quote(target['id']) + '/publish',
                      {'expected_version': current['version'] if current else 0})
    if current['active_revision_id'] != target['id'] or current['state'] != 'available':
        raise RuntimeError('publication did not activate expected revision')
    record('publication-response.json', current)
    cleanup_staging_definitions(base, token, args.connector_source)
    health = api(base, token, 'GET', '/api/v1/admin/connectors/publication-health')
    record('publication-health.json', [item for item in health.get('items', []) if item['source'] == args.connector_source])
    print(json.dumps({'source': args.connector_source, 'revision_id': target['id'], 'state': current['state'],
                      'package_sha256': target['sha256'], 'bundle_sha256': bundle_sha,
                      'account_api_verification': 'not_run'}, ensure_ascii=False), flush=True)


if __name__ == '__main__':
    try:
        main()
    except error.HTTPError as exc:
        print('publication_http_error', exc.code)
        raise SystemExit(1)
    except Exception as exc:
        print('publication_failed', type(exc).__name__, str(exc) if isinstance(exc, RuntimeError) else '')
        raise SystemExit(1)
