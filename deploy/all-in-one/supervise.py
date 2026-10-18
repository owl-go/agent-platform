#!/usr/local/bin/python3
"""PID 1: ordered startup, health gates and bounded, reverse-order shutdown."""
import fcntl
import hashlib
import hmac
import json
import os
from pathlib import Path
import pwd
import re
import signal
import subprocess
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import Request, urlopen

from config import browser_config, initialize_state, nginx_config, platform_config, private_write, public_origin, realm_config, sandbox_runtime

DATA = Path('/data')
RUN = Path('/run/agent-platform')
APPLICATION = Path('/opt/agent-platform')
children = []
stopping = False
instance_lock = None


def request(url, *, data=None, headers=None, method=None):
    with urlopen(Request(url, data=data, headers=headers or {}, method=method), timeout=3) as response:
        return response.read()


def health():
    for url in ('http://127.0.0.1:19080/readyz', 'http://127.0.0.1:19082/readyz',
                'http://127.0.0.1:9000/minio/health/ready',
                'http://127.0.0.1:8081/identity/realms/agent-platform/.well-known/openid-configuration'):
        request(url)


def check_children():
    if stopping:
        raise InterruptedError('shutdown requested')
    for name, process in children:
        if process.poll() is not None:
            raise RuntimeError(name + ' exited; restarting the entire container is required')


def wait_for(probe, description, timeout=180):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        check_children()
        try:
            probe()
            return
        except (OSError, URLError, subprocess.SubprocessError):
            time.sleep(0.5)
    raise RuntimeError(description + ' did not become ready')


def identity_setup():
    user = pwd.getpwnam('postgres')
    os.setgroups([])
    os.setgid(user.pw_gid)
    os.setuid(user.pw_uid)


def execute(arguments, *, environment=None, data=None, postgres=False):
    result = subprocess.run(arguments, input=data, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            preexec_fn=identity_setup if postgres else None, timeout=120, check=False)
    if result.returncode:
        # Child output may include a connection string or SQL secret: never
        # interpolate arguments, env, stdin or output into the startup log.
        raise subprocess.SubprocessError(Path(arguments[0]).name + ' initialization failed (exit ' + str(result.returncode) + ')')
    return result.stdout.decode().strip()


def start(name, arguments, *, environment=None, postgres=False):
    process = subprocess.Popen(arguments, env=environment, start_new_session=True,
                               preexec_fn=identity_setup if postgres else None)
    children.append((name, process))
    print('Started ' + name, flush=True)


def docker(*arguments):
    return execute(['/usr/bin/docker', *arguments])


def docker_identity(profile):
    if not Path('/var/run/docker.sock').is_socket():
        raise ValueError('mount /var/run/docker.sock to execute isolated Runtime tasks')
    instance = os.environ.get('AGENT_PLATFORM_CONTAINER', os.uname().nodename)
    metadata = json.loads(docker('inspect', instance))[0]
    mounts = [item for item in metadata['Mounts'] if item['Destination'] != '/var/run/docker.sock']
    data_mount = next((item for item in mounts if item['Destination'] == '/data'), None)
    if not data_mount or not data_mount['RW'] or ',' in data_mount['Source']:
        raise ValueError('mount one persistent writable data volume at /data')
    runtimes = json.loads(docker('info', '--format', '{{json .Runtimes}}'))
    runtime = {'local': 'runc', 'production': 'runsc'}.get(profile)
    if not runtime or runtime not in runtimes:
        raise ValueError('the explicitly selected Sandbox Runtime is not installed on the Docker host')
    image = metadata['Image']
    if profile == 'production':
        image = os.environ.get('RUNTIME_IMAGE', '')
        pinned = json.loads(docker('image', 'inspect', image))[0] if '@sha256:' in image else {}
        if pinned.get('Id') != metadata['Image']:
            raise ValueError('production RUNTIME_IMAGE must be this exact unified image repository@sha256 digest')
        if not Path('/run/agent-platform-egress/egress-controller.sock').is_socket() or not (DATA / 'config/sandbox-resolv.conf').is_file():
            raise ValueError('production requires the existing host Egress Controller socket and resolver policy')
        network = os.environ.get('EGRESS_NETWORK', 'agent-public-egress')
        docker('network', 'inspect', network)
    else:
        network = 'agent-platform-runtime-' + hashlib.sha256(data_mount['Source'].encode()).hexdigest()[:12]
        try:
            details = json.loads(docker('network', 'inspect', network))[0]
            if details['Driver'] != 'bridge' or details['Internal'] or details.get('Labels', {}).get('com.agent-workspace.unified') != 'true':
                raise ValueError('local Runtime network ownership or configuration has drifted')
        except subprocess.SubprocessError:
            docker('network', 'create', '--label', 'com.agent-workspace.unified=true', network)
    return image, mounts, data_mount['Source'], network


def initialize_database(state):
    user = pwd.getpwnam('postgres')
    for path in (DATA / 'postgres', RUN / 'postgres'):
        path.mkdir(exist_ok=True, mode=0o700)
        os.chown(path, user.pw_uid, user.pw_gid)
    # PostgreSQL must be able to traverse /data, but private system credentials
    # and config remain root-only files/directories.
    DATA.chmod(0o711)
    if not (DATA / 'postgres/PG_VERSION').exists():
        execute(['initdb', '-D', str(DATA / 'postgres'), '--username=postgres', '--encoding=UTF8', '--locale=C.UTF-8', '--auth-local=trust', '--auth-host=scram-sha-256'], postgres=True)
    start('postgres', ['postgres', '-D', str(DATA / 'postgres'), '-h', '127.0.0.1', '-p', '5432', '-k', str(RUN / 'postgres')], postgres=True)
    wait_for(lambda: execute(['pg_isready', '-h', str(RUN / 'postgres'), '-U', 'postgres'], postgres=True), 'PostgreSQL')
    sql = "SELECT 1 FROM pg_roles WHERE rolname='agent_workspace';"
    exists = execute(['psql', '-h', str(RUN / 'postgres'), '-U', 'postgres', '-At'], data=sql.encode(), postgres=True)
    if exists != '1':
        # Only a generated hex value enters this fixed SQL statement, over
        # stdin, not argv. The role is not a superuser.
        sql = "CREATE ROLE agent_workspace LOGIN PASSWORD '" + state['database_password'] + "';"
        execute(['psql', '-v', 'ON_ERROR_STOP=1', '-h', str(RUN / 'postgres'), '-U', 'postgres'], data=sql.encode(), postgres=True)
    for name in ('agent_workspace', 'keycloak'):
        found = execute(['psql', '-h', str(RUN / 'postgres'), '-U', 'postgres', '-At'], data=("SELECT 1 FROM pg_database WHERE datname='" + name + "';").encode(), postgres=True)
        if found != '1':
            execute(['createdb', '-h', str(RUN / 'postgres'), '-U', 'postgres', '-O', 'agent_workspace', name], postgres=True)


def initialize_bucket(state):
    from datetime import datetime, timezone
    date = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    day, host = date[:8], '127.0.0.1:9000'
    empty = hashlib.sha256(b'').hexdigest()
    canonical = 'PUT\n/agent-workspace\n\nhost:' + host + '\nx-amz-content-sha256:' + empty + '\nx-amz-date:' + date + '\n\nhost;x-amz-content-sha256;x-amz-date\n' + empty
    scope = day + '/us-east-1/s3/aws4_request'
    key = ('AWS4' + state['minio_password']).encode()
    for value in (day, 'us-east-1', 's3', 'aws4_request'):
        key = hmac.new(key, value.encode(), hashlib.sha256).digest()
    signature = hmac.new(key, ('AWS4-HMAC-SHA256\n' + date + '\n' + scope + '\n' + hashlib.sha256(canonical.encode()).hexdigest()).encode(), hashlib.sha256).hexdigest()
    try:
        request('http://' + host + '/agent-workspace', method='PUT', headers={
            'x-amz-date': date, 'x-amz-content-sha256': empty,
            'Authorization': 'AWS4-HMAC-SHA256 Credential=agent_workspace/' + scope + ', SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=' + signature})
    except HTTPError as error:
        if error.code != 409:
            # Only the bounded provider error code is diagnostic. Never log
            # the response body: signature failures can echo signed requests.
            code = re.search(rb'<Code>([A-Za-z0-9_]{1,64})</Code>', error.read(4096))
            detail = code.group(1).decode() if code else 'unknown'
            if error.code == 503 and detail == 'XMinioServerNotInitialized':
                raise URLError('private object storage is still initializing') from None
            raise RuntimeError('private object bucket initialization failed (HTTP ' + str(error.code) + ', ' + detail + ')') from None


def synchronize_redirects(state, origin):
    # Imports deliberately never reset an existing User password. Only the
    # public OIDC client's redirect/origin configuration is updated at restart.
    base = 'http://127.0.0.1:8081/identity'
    token = json.loads(request(base + '/realms/master/protocol/openid-connect/token', data=urlencode({
        'grant_type': 'password', 'client_id': 'admin-cli', 'username': 'bootstrap-administrator', 'password': state['identity_password']}).encode(),
        headers={'Content-Type': 'application/x-www-form-urlencoded'}))['access_token']
    headers = {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'}
    endpoint = base + '/admin/realms/agent-platform/clients'
    clients = json.loads(request(endpoint + '?clientId=agent-platform-web', headers=headers))
    if len(clients) != 1:
        raise RuntimeError('public OIDC client is missing or ambiguous')
    client = clients[0]
    client['redirectUris'], client['webOrigins'] = [origin + '/*'], [origin]
    client.setdefault('attributes', {})['post.logout.redirect.uris'] = origin + '/*'
    request(endpoint + '/' + client['id'], method='PUT', data=json.dumps(client).encode(), headers=headers)


def main():
    global instance_lock
    os.umask(0o077)
    RUN.mkdir(parents=True, exist_ok=True)
    RUN.chmod(0o711)
    DATA.mkdir(exist_ok=True)
    # Retain this descriptor for the entire control process lifetime.
    instance_lock = open(DATA / 'instance.lock', 'a')
    fcntl.flock(instance_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    origin = public_origin(os.environ.get('PUBLIC_URL', 'http://localhost:8080'))
    password = os.environ.get('ADMIN_PASSWORD', 'admin123')
    if urlsplit(origin).hostname not in ('localhost', '127.0.0.1', '::1') and password == 'admin123':
        raise ValueError('set ADMIN_PASSWORD before exposing a public deployment; admin123 is local-only')
    state = initialize_state(DATA, os.environ.get('ADMIN_USERNAME', 'admin'))
    for name in ('config', 'config/identity-import', 'minio', 'workspaces', 'run-credentials'):
        (DATA / name).mkdir(parents=True, exist_ok=True, mode=0o700)
    profile = os.environ.get('SANDBOX_PROFILE', 'local')
    sandbox_runtime(profile, origin)
    image, mounts, source, network = docker_identity(profile)
    configuration = platform_config(state, origin, image, source, profile, network, os.environ)
    private_write(DATA / 'config/platform.yaml', json.dumps(configuration))
    known_hosts = DATA / 'config/known_hosts'
    if not known_hosts.exists():
        private_write(known_hosts, '')
    realm = realm_config(json.loads((APPLICATION / 'keycloak-realm.json').read_text()), state, origin, password)
    private_write(DATA / 'config/identity-import/agent-platform-realm.json', json.dumps(realm))
    import_directory = Path('/opt/keycloak/data/import')
    import_directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    private_write(import_directory / 'agent-platform-realm.json', json.dumps(realm))
    private_write(RUN / 'runtime-config.js', browser_config(origin))
    private_write(RUN / 'nginx.conf', nginx_config(origin))
    # Nginx workers only need this public file, not any data/config secret.
    (RUN / 'runtime-config.js').chmod(0o644)
    initialize_database(state)
    environment = dict(os.environ, MINIO_ROOT_USER='agent_workspace', MINIO_ROOT_PASSWORD=state['minio_password'])
    start('minio', ['minio', 'server', '/data/minio', '--address', '127.0.0.1:9000', '--console-address', '127.0.0.1:9001'], environment=environment)
    wait_for(lambda: request('http://127.0.0.1:9000/minio/health/ready'), 'MinIO')
    wait_for(lambda: initialize_bucket(state), 'private object bucket', timeout=60)
    environment = dict(os.environ, KC_DB='postgres', KC_DB_URL='jdbc:postgresql://127.0.0.1:5432/keycloak',
                       KC_DB_USERNAME='agent_workspace', KC_DB_PASSWORD=state['database_password'],
                       KC_BOOTSTRAP_ADMIN_USERNAME='bootstrap-administrator', KC_BOOTSTRAP_ADMIN_PASSWORD=state['identity_password'],
                       KC_HOSTNAME=origin + '/identity', KC_HTTP_RELATIVE_PATH='/identity', KC_HTTP_HOST='127.0.0.1', KC_HTTP_PORT='8081',
                       KC_CACHE='local', KC_HTTP_ENABLED='true',
                       KC_PROXY_HEADERS='xforwarded', KC_PROXY_TRUSTED_ADDRESSES='127.0.0.1',
                       JAVA_OPTS_APPEND='-Xms128m -Xmx768m -Djava.net.preferIPv4Stack=true')
    start('keycloak', ['/opt/keycloak/bin/kc.sh', 'start', '--optimized', '--import-realm'], environment=environment)
    wait_for(lambda: request('http://127.0.0.1:8081/identity/realms/agent-platform/.well-known/openid-configuration'), 'Keycloak', timeout=600)
    synchronize_redirects(state, origin)
    start('nginx', ['/usr/sbin/nginx', '-c', str(RUN / 'nginx.conf'), '-g', 'daemon off;'])
    start('api', ['/usr/local/bin/api', '-config', '/data/config/platform.yaml'])
    wait_for(lambda: request('http://127.0.0.1:19080/readyz'), 'API')
    environment = dict(os.environ, PATH='/opt/agent-platform/docker-bin:' + os.environ['PATH'], AGENT_PLATFORM_DOCKER_MOUNTS=json.dumps(mounts))
    start('worker', ['/usr/local/bin/worker', '-config', '/data/config/platform.yaml'], environment=environment)
    wait_for(health, 'Worker and application')
    print('Agent Workspace ready at ' + origin + '; bootstrap account: ' + state['admin_username'] + '. Change the initial password before public use.', flush=True)
    while not stopping:
        check_children()
        time.sleep(0.5)


def stop(signum=None, frame=None):
    global stopping
    stopping = True


def shutdown():
    for name, process in reversed(children):
        if process.poll() is None:
            print('Stopping ' + name, flush=True)
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=20 if name in ('worker', 'postgres') else 10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)


if __name__ == '__main__':
    if sys.argv[1:] == ['health']:
        try:
            health()
        except Exception:
            sys.exit(1)
    else:
        signal.signal(signal.SIGTERM, stop)
        signal.signal(signal.SIGINT, stop)
        result = 0
        try:
            main()
        except InterruptedError:
            pass
        except (ValueError, RuntimeError, subprocess.SubprocessError) as error:
            print('Startup failed: ' + str(error), file=sys.stderr, flush=True)
            result = 1
        except Exception as error:
            # Never print exception payloads from HTTP responses or file bodies.
            print('Startup failed (' + type(error).__name__ + '); inspect component startup output.', file=sys.stderr, flush=True)
            result = 1
        finally:
            shutdown()
            if instance_lock is not None:
                instance_lock.close()
        sys.exit(result)
