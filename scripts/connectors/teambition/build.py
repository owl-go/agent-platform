#!/usr/bin/env python3
"""Build an immutable Teambition CLI package; never publishes or claims Conformance."""
import argparse
import base64
import hashlib
import io
import json
import re
import subprocess
import tempfile
import tarfile
import zipfile
from pathlib import Path

ROOT = Path(__file__).parent
CLI_VERSION = '0.3.3'
VERSION = '0.3.5'
NPM_INTEGRITY = 'S5+aHcBI5alBIPPUSzAJafQDws50hfyv+ns1MiUEZW611ibpOjxGK0+q/eTBPvE15vYUAzsQP/vLStz25Rt5VQ=='
SKILL_SHA256 = '3a0cd868f8fa4eb6cc56bf1be659438db2e19e1f74299e608ed0c7a77a226afc'
ASSETS = {
    'x64': 'a39c02be0417d8fba37ddbfd22ac846eb6ca73ca08cf0dccf77b2c7005078eed',
    'arm64': '0d2f8a5b1dcb195256efab7c8a20e80f7438eb553418334c6b02d54620b497c0',
}
HOSTS = ['open.teambition.com']


def sha256(value):
    return hashlib.sha256(value).hexdigest()


def json_bytes(value):
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + '\n').encode()


def reviewed_scopes(prefix):
    if prefix[0] == 'user': return ['user:read']
    if prefix[0] == 'project': return ['project:read', 'task:read']
    if prefix[0] == 'task': return ['task:write'] if prefix[1] in ['create', 'move'] else ['task:read']
    return []


def capabilities():
    reviewed = json.loads((ROOT / 'capabilities.json').read_text())
    prefixes = [tuple(item['argv_prefix']) for item in reviewed]
    if len(prefixes) != len(set(prefixes)) or any(
        a != b and b[:len(a)] == a for a in prefixes for b in prefixes
    ):
        raise ValueError('duplicate or overlapping command policy')
    return [{**item, 'id': 'tb_' + sha256(' '.join(item['argv_prefix']).encode())[:16],
             'identities': ['user'], 'scopes': reviewed_scopes(item['argv_prefix']), 'egress_hosts': HOSTS,
             'timeout_seconds': 120} for item in reviewed]


def binaries(npm):
    if base64.b64encode(hashlib.sha512(npm).digest()).decode() != NPM_INTEGRITY:
        raise ValueError('npm integrity differs from pinned 0.3.3')
    wanted = {f'package/assets/teambition-cli-linux-{arch}.tar.gz': arch for arch in ASSETS}
    found = {}
    with tarfile.open(fileobj=io.BytesIO(npm), mode='r:gz') as archive:
        for member in archive:
            if member.name not in wanted:
                continue
            arch = wanted[member.name]
            if not member.isfile() or arch in found or member.size > 30 << 20:
                raise ValueError('invalid upstream Linux asset')
            asset = archive.extractfile(member).read()
            if sha256(asset) != ASSETS[arch]:
                raise ValueError('upstream Linux asset hash differs')
            with tarfile.open(fileobj=io.BytesIO(asset), mode='r:gz') as native:
                members = native.getmembers()
                if len(members) != 1 or members[0].name != 'teambition-cli' or not members[0].isfile():
                    raise ValueError('unexpected native archive')
                binary = native.extractfile(members[0]).read()
            machine = 62 if arch == 'x64' else 183
            if binary[:4] != b'\x7fELF' or int.from_bytes(binary[18:20], 'little') != machine:
                raise ValueError('wrong native ELF architecture')
            found[arch] = binary
    if set(found) != set(ASSETS):
        raise ValueError('missing Linux assets')
    return found


def source_zip(native, reviewed):
    metadata = {
        'name': '@agent-platform/teambition-connector', 'version': CLI_VERSION,
        'bin': {'teambition': 'launcher.cjs'},
        'agentWorkspace': {
            'executable': 'teambition', 'authenticationDriver': 'connector_package',
            'supportedArchitectures': ['linux-amd64', 'linux-arm64'],
            'capabilities': [{
                'id': c['id'], 'argvPrefix': c['argv_prefix'], 'risk': c['risk'],
                'identities': c['identities'], 'scopes': c['scopes'],
                'egressHosts': c['egress_hosts'], 'timeoutSeconds': c['timeout_seconds'],
            } for c in reviewed],
        },
    }
    files = {
        'launcher.cjs': ((ROOT / 'launcher.cjs').read_bytes(), 0o755),
        'oauth-transport.cjs': ((ROOT / 'oauth-transport.cjs').read_bytes(), 0o644),
        'capabilities.json': (json_bytes(reviewed), 0o644),
        'package.json': (json_bytes(metadata), 0o644),
        **{f'teambition-linux-{arch}': (body, 0o755) for arch, body in native.items()},
    }
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name, (body, mode) in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.compress_type, entry.external_attr = zipfile.ZIP_DEFLATED, (0o100000 | mode) << 16
            archive.writestr(entry, body)
    return output.getvalue()


def bundle(source):
    # Match the platform's ZIPPackageBuilder byte for byte so its real isolated
    # Conformance records can be used when staging the enclosing package.
    with tempfile.TemporaryDirectory(prefix='teambition-build-') as temporary:
        root = Path(temporary)
        (root / 'source.zip').write_bytes(source)
        subprocess.run(['go', '-C', str(ROOT.resolve().parents[2] / 'backend'), 'run',
                        './cmd/cli-connector-bundle', str(root / 'source.zip'), str(root / 'bundle.tgz')], check=True)
        return (root / 'bundle.tgz').read_bytes()


def build(npm, skill, image, runtime_version):
    match = re.fullmatch(r'[^\s@]+(?:/[^\s@]+)*@sha256:([a-f0-9]{64})', image)
    if not match or not re.fullmatch(r'\d+\.\d+\.\d+', runtime_version):
        raise ValueError('require a Registry RepoDigest and exact Node version')
    if sha256(skill) != SKILL_SHA256:
        raise ValueError('upstream Skill checksum differs')
    reviewed = capabilities()
    source = source_zip(binaries(npm), reviewed)
    immutable = bundle(source)
    files = {
        'skills/teambition/SKILL.md': (ROOT / 'SKILL.md').read_bytes(),
        'skills/teambition/capabilities.json': json_bytes(reviewed),
        'cli-bundle.tgz': immutable,

    }
    icon = (ROOT / 'teambition.png').read_bytes()
    if sha256(icon) != '941a5a0aa5c3609ead813267836a717d369c1e48321fd69eae1d985df4674908':
        raise ValueError('Teambition icon checksum differs')
    files['icon.svg'] = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><image width="128" height="128" href="data:image/png;base64,' + base64.b64encode(icon).decode() + '"/></svg>').encode()
    with zipfile.ZipFile(io.BytesIO(skill)) as archive:
        expected = {'teambition/SKILL.md', 'teambition/references/tql.md', 'teambition/references/project-tql.md'}
        if set(archive.namelist()) != expected:
            raise ValueError('upstream Skill archive content differs')
        for name in expected:
            files['skills/teambition/reference/' + name.removeprefix('teambition/')] = archive.read(name)
    files['connector-meta.json'] = json_bytes({
        'source': 'teambition', 'version': VERSION, 'type': 'cli', 'name': '钉钉项目',
        'description': 'Teambition 项目与任务查询、任务创建和移动；通过浏览器 OAuth + PKCE 授权连接。',
        'examples_zh': ['查询我参与的钉钉项目', '在指定项目创建任务'],
        'examples_en': ['List my Teambition projects', 'Create a task in a selected project'],
        'minPlatformVersion': '1.0.0', 'auth_mode': 'oauth',
    })
    files['cli.json'] = json_bytes({
        'runtime': {'kind': 'node', 'version': runtime_version, 'digest': 'sha256:' + match.group(1)},
        'executable': 'teambition', 'bundle_path': 'node_modules/.bin/teambition',
        'authentication_driver': 'connector_package',
        'commands': {key: {'argv': ['platform', action]} for key, action in
                     [('init', 'init'), ('auth', 'auth'), ('status', 'status'), ('unAuth', 'unauth')]},
        'status_match': {'json_path': '$.configured', 'equals': True},
        'capabilities': reviewed, 'egress_hosts': HOSTS, 'timeout_seconds': 120,
        'resource_limits': {'cpu_millis': 1000, 'memory_mib': 512, 'timeout_seconds': 180,
                            'concurrency': 1, 'child_processes': 64},
    })
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name, body in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.compress_type, entry.external_attr = zipfile.ZIP_DEFLATED, 0o100644 << 16
            archive.writestr(entry, body)
    return output.getvalue(), immutable, source


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--npm-tgz', type=Path, required=True)
    parser.add_argument('--skill-zip', type=Path, required=True)
    parser.add_argument('--runtime-image', required=True)
    parser.add_argument('--runtime-version', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    package, immutable, source = build(args.npm_tgz.read_bytes(), args.skill_zip.read_bytes(), args.runtime_image, args.runtime_version)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(package)
    args.output.with_suffix(".source.zip").write_bytes(source)
    print(json.dumps({'output': str(args.output), 'sha256': sha256(package),
                      'bundle_sha256': sha256(immutable), 'capabilities': len(capabilities()),
                      'conformance': 'not_run', 'authorization': 'browser_oauth_pkce'}))


if __name__ == '__main__':
    main()
