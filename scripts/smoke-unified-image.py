#!/usr/bin/env python3
"""Isolated, model-free acceptance for a real unified image."""
import argparse
import json
import socket
import subprocess
import time
import uuid
from urllib.error import HTTPError, URLError
from urllib.request import urlopen


def docker(*arguments):
    return subprocess.check_output(['docker', *arguments], text=True).strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('image')
    parser.add_argument('--platform', choices=['linux/arm64', 'linux/amd64'])
    parser.add_argument('--keep', action='store_true', help='keep the named test data and service for browser acceptance')
    args = parser.parse_args()
    identity = 'agent-platform-smoke-' + uuid.uuid4().hex[:10]
    volume = identity + '-data'
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    origin = 'http://localhost:' + str(port)
    docker('volume', 'create', '--label', 'com.agent-workspace.test=true', volume)
    try:
        flags = ['--platform', args.platform] if args.platform else []
        docker('run', '-d', '--name', identity, '--label', 'com.agent-workspace.test=true', *flags,
               '--security-opt', 'no-new-privileges', '--memory', '4g', '--pids-limit', '2048',
               '-p', '127.0.0.1:' + str(port) + ':8080', '-e', 'PUBLIC_URL=' + origin,
               '-v', volume + ':/data', '-v', '/var/run/docker.sock:/var/run/docker.sock', args.image)
        deadline = time.monotonic() + 900
        while time.monotonic() < deadline:
            state = json.loads(docker('inspect', identity))[0]['State']
            if not state['Running']:
                raise RuntimeError('unified container exited; inspect docker logs ' + identity)
            if state.get('Health', {}).get('Status') == 'healthy':
                break
            time.sleep(1)
        else:
            raise RuntimeError('unified image health timed out; inspect docker logs ' + identity)
        with urlopen(origin, timeout=5) as response:
            assert response.status == 200
            assert b'runtime-config.js' in response.read()
        for path in ('/api/healthz', '/api/readyz'):
            with urlopen(origin + path, timeout=5) as response:
                assert response.status == 200
                assert json.load(response)['status'] in ('ok', 'ready')
        with urlopen(origin + '/runtime-config.js', timeout=5) as response:
            assert origin.encode() in response.read()
            assert 'no-store' in response.headers.get('Cache-Control', '')
        with urlopen(origin + '/identity/realms/agent-platform/.well-known/openid-configuration', timeout=5) as response:
            discovery = json.load(response)
            assert discovery['issuer'] == origin + '/identity/realms/agent-platform'
        try:
            urlopen(origin + '/api/v1/runtime-engines', timeout=5)
            raise AssertionError('anonymous request bypassed authentication')
        except HTTPError as error:
            assert error.code == 401, error.code
        # The exact source-image and volume-to-host mapping are exercised by a
        # non-root, read-only, no-network Runtime container using that image.
        probe = '''import json,os,subprocess
from pathlib import Path
root=Path('/data/workspaces/smoke-isolation')
root.mkdir(exist_ok=True)
os.chown(root,65532,65532)
os.chmod(root,0o750)
configuration=json.loads(Path('/data/config/platform.yaml').read_text())
metadata=json.loads(subprocess.check_output(['/usr/bin/docker','inspect',os.uname().nodename]))[0]
environment=dict(os.environ,AGENT_PLATFORM_DOCKER_MOUNTS=json.dumps([m for m in metadata['Mounts'] if m['Destination']=='/data']))
result=subprocess.check_output(['/opt/agent-platform/docker-bin/docker','run','--rm','--runtime','runc','--user','65532:65532','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--network','none','--pids-limit','64','--memory','256m','--mount','type=bind,src='+str(root)+',dst=/workspace','--entrypoint','/usr/bin/id',configuration['worker']['runtimes']['codex']['image_digest'],'-u'],env=environment,text=True).strip()
assert result=='65532',result
root.rmdir()
print('runtime uid='+result)
'''
        print(docker('exec', identity, 'python3', '-c', probe))
        before = docker('exec', identity, 'sha256sum', '/data/state.json')
        docker('restart', '--time', '60', identity)
        deadline = time.monotonic() + 900
        while time.monotonic() < deadline:
            state = json.loads(docker('inspect', identity))[0]['State']
            if not state['Running']:
                raise RuntimeError('container failed during retained-data restart')
            if state.get('Health', {}).get('Status') == 'healthy':
                break
            time.sleep(1)
        else:
            raise RuntimeError('retained-data restart readiness timed out')
        assert docker('exec', identity, 'sha256sum', '/data/state.json') == before
        print(json.dumps({'container': identity, 'volume': volume, 'url': origin, 'status': 'passed', 'image': args.image}))
    finally:
        if not args.keep:
            # Exact, uniquely created test targets only. Existing volumes and
            # services are never included in this cleanup.
            subprocess.run(['docker', 'rm', '--force', identity], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(['docker', 'volume', 'rm', volume], check=False, stdout=subprocess.DEVNULL)


if __name__ == '__main__':
    main()
